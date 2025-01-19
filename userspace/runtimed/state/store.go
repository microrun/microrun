package state

import (
	"context"
	"fmt"
	"strings"
	"sync"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
	"github.com/microrun/microrun/userspace/runtimed/logging"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// Store provides thread-safe access to resources
type Store interface {
	// Get retrieves a resource by name with type safety
	Get(ctx context.Context, kind, name string) (*pb.Resource, error)

	// List returns all resources of a given kind
	List(ctx context.Context, kind string) ([]*pb.Resource, error)

	// Create adds a new resource
	Create(ctx context.Context, resource *pb.Resource) error

	// Update modifies an existing resource
	Update(ctx context.Context, resource *pb.Resource) error

	// Delete removes a resource
	Delete(ctx context.Context, kind, name string) error

	// Watch provides a channel of resource changes
	Watch(ctx context.Context, kind string) (<-chan Event, error)
}

// Event represents a change in the store
type Event struct {
	Type     EventType
	Resource *pb.Resource
}

type EventType int

const (
	EventCreated EventType = iota
	EventUpdated
	EventDeleted
)

// memoryStore implements Store using in-memory storage
type memoryStore struct {
	mu       sync.RWMutex
	data     map[string]map[string]*pb.Resource // kind -> name -> resource
	watchers map[string][]chan Event
	logger   *logging.Logger
}

func NewMemoryStore() Store {
	return &memoryStore{
		data:     make(map[string]map[string]*pb.Resource),
		watchers: make(map[string][]chan Event),
		logger:   logging.NewLogger("store", logging.ComponentController),
	}
}

func (s *memoryStore) Get(ctx context.Context, kind, name string) (*pb.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources, ok := s.data[kind]
	if !ok {
		s.logger.Debug("Kind not found for get",
			zap.String("kind", kind))
		return nil, fmt.Errorf("kind %s not found", kind)
	}

	resource, ok := resources[name]
	if !ok {
		s.logger.Debug("Resource not found for get",
			zap.String("kind", kind),
			zap.String("name", name))
		return nil, fmt.Errorf("resource %s/%s not found", kind, name)
	}

	// Return a deep copy to maintain immutability
	return proto.Clone(resource).(*pb.Resource), nil
}

func (s *memoryStore) List(ctx context.Context, kind string) ([]*pb.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources, ok := s.data[kind]
	if !ok {
		s.logger.Debug("Kind not found for list",
			zap.String("kind", kind))
		return nil, nil
	}

	result := make([]*pb.Resource, 0, len(resources))
	for _, r := range resources {
		// Deep copy each resource
		result = append(result, proto.Clone(r).(*pb.Resource))
	}

	return result, nil
}

func (s *memoryStore) Create(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		s.logger.Error("Resource metadata is required")
		return fmt.Errorf("resource metadata is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	kind := resource.Metadata.Kind
	name := resource.Metadata.Name

	if _, ok := s.data[kind]; !ok {
		s.data[kind] = make(map[string]*pb.Resource)
	}

	if _, exists := s.data[kind][name]; exists {
		s.logger.Error("Resource already exists",
			zap.String("kind", kind),
			zap.String("name", name))
		return fmt.Errorf("resource %s/%s already exists", kind, name)
	}

	// Store a deep copy
	s.data[kind][name] = proto.Clone(resource).(*pb.Resource)

	s.logger.Info("Resource created",
		zap.String("kind", kind),
		zap.String("name", name))
	s.logger.Debug("Created resource state",
		zap.String("kind", kind),
		zap.String("name", name),
		zap.Any("resource", resource))

	// Notify watchers
	s.notify(EventCreated, resource)

	return nil
}

func (s *memoryStore) Update(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		s.logger.Error("Resource metadata is required")
		return fmt.Errorf("resource metadata is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	kind := resource.Metadata.Kind
	name := resource.Metadata.Name

	// Get existing resource
	existing, ok := s.data[kind][name]
	if !ok {
		s.logger.Error("Resource not found for update",
			zap.String("kind", kind),
			zap.String("name", name))
		return fmt.Errorf("resource %s/%s not found", kind, name)
	}

	// Verify ownership if set
	if existing.Metadata.Owner != "" && existing.Metadata.Owner != resource.Metadata.Owner {
		s.logger.Error("Unauthorized update attempt",
			zap.String("kind", kind),
			zap.String("name", name),
			zap.String("owner", existing.Metadata.Owner),
			zap.String("attempted_owner", resource.Metadata.Owner))
		return fmt.Errorf("resource %s/%s can only be modified by owner %s", kind, name, existing.Metadata.Owner)
	}

	// Check if anything has actually changed, before touching generation
	if proto.Equal(existing, resource) {
		s.logger.Debug("No changes detected in update",
			zap.String("kind", kind),
			zap.String("name", name))
		// No changes, make it a no-op
		return nil
	}

	// We have changes, increment generation
	resource.Metadata.Generation = existing.Metadata.Generation + 1

	// Log the diff of changes
	diff := diffResources(existing, resource)
	s.logger.Debug("Resource changes",
		zap.String("kind", kind),
		zap.String("name", name),
		zap.String("diff", diff))

	// Store deep copy
	s.data[kind][name] = proto.Clone(resource).(*pb.Resource)

	s.logger.Info("Resource updated",
		zap.String("kind", kind),
		zap.String("name", name),
		zap.Int64("generation", resource.Metadata.Generation))

	// Notify watchers
	s.notify(EventUpdated, resource)

	return nil
}

func (s *memoryStore) Delete(ctx context.Context, kind, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resources, ok := s.data[kind]
	if !ok {
		s.logger.Debug("Kind not found for delete",
			zap.String("kind", kind))
		return fmt.Errorf("kind %s not found", kind)
	}

	resource, ok := resources[name]
	if !ok {
		s.logger.Debug("Resource not found for delete",
			zap.String("kind", kind),
			zap.String("name", name))
		return fmt.Errorf("resource %s/%s not found", kind, name)
	}

	s.logger.Debug("Resource state before delete",
		zap.String("kind", kind),
		zap.String("name", name),
		zap.Any("resource", resource))

	// Check finalizers
	if len(resource.Metadata.Finalizers) > 0 {
		s.logger.Error("Resource has pending finalizers",
			zap.String("kind", kind),
			zap.String("name", name),
			zap.Strings("finalizers", resource.Metadata.Finalizers))
		return fmt.Errorf("resource %s/%s has pending finalizers", kind, name)
	}

	delete(resources, name)

	s.logger.Info("Resource deleted",
		zap.String("kind", kind),
		zap.String("name", name))

	// Notify watchers
	s.notify(EventDeleted, resource)

	return nil
}

func (s *memoryStore) Watch(ctx context.Context, kind string) (<-chan Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan Event, 100)
	if _, ok := s.watchers[kind]; !ok {
		s.watchers[kind] = make([]chan Event, 0)
	}
	s.watchers[kind] = append(s.watchers[kind], ch)

	// Remove watcher when context is done
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()

		watchers := s.watchers[kind]
		for i, w := range watchers {
			if w == ch {
				s.watchers[kind] = append(watchers[:i], watchers[i+1:]...)
				close(ch)
				break
			}
		}
	}()

	return ch, nil
}

func (s *memoryStore) notify(eventType EventType, resource *pb.Resource) {
	kind := resource.Metadata.Kind
	watchers := s.watchers[kind]
	event := Event{
		Type:     eventType,
		Resource: proto.Clone(resource).(*pb.Resource),
	}
	for _, ch := range watchers {
		ch <- event
	}
}

func diffResources(old, new *pb.Resource) string {
	oldText := prototext.Format(old)
	newText := prototext.Format(new)

	// Split into lines
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	var diff strings.Builder
	for i := 0; i < len(oldLines) || i < len(newLines); i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" {
				diff.WriteString(fmt.Sprintf("- %s\n", oldLine))
			}
			if newLine != "" {
				diff.WriteString(fmt.Sprintf("+ %s\n", newLine))
			}
		}
	}

	return diff.String()
}
