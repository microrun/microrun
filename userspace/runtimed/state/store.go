package state

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
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
}

func NewMemoryStore() Store {
	return &memoryStore{
		data:     make(map[string]map[string]*pb.Resource),
		watchers: make(map[string][]chan Event),
	}
}

func (s *memoryStore) Get(ctx context.Context, kind, name string) (*pb.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources, ok := s.data[kind]
	if !ok {
		return nil, fmt.Errorf("kind %s not found", kind)
	}

	resource, ok := resources[name]
	if !ok {
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
		return fmt.Errorf("resource %s/%s already exists", kind, name)
	}

	// Store a deep copy
	s.data[kind][name] = proto.Clone(resource).(*pb.Resource)

	// Notify watchers
	s.notify(EventCreated, resource)

	return nil
}

func (s *memoryStore) Update(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		return fmt.Errorf("resource metadata is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	kind := resource.Metadata.Kind
	name := resource.Metadata.Name

	existing, ok := s.data[kind][name]
	if !ok {
		return fmt.Errorf("resource %s/%s not found", kind, name)
	}

	// Verify ownership if set
	if existing.Metadata.Owner != "" && existing.Metadata.Owner != resource.Metadata.Owner {
		return fmt.Errorf("resource %s/%s can only be modified by owner %s", kind, name, existing.Metadata.Owner)
	}

	// Increment generation
	resource.Metadata.Generation = existing.Metadata.Generation + 1

	// Store deep copy
	s.data[kind][name] = proto.Clone(resource).(*pb.Resource)

	// Notify watchers
	s.notify(EventUpdated, resource)

	return nil
}

func (s *memoryStore) Delete(ctx context.Context, kind, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	resources, ok := s.data[kind]
	if !ok {
		return fmt.Errorf("kind %s not found", kind)
	}

	resource, ok := resources[name]
	if !ok {
		return fmt.Errorf("resource %s/%s not found", kind, name)
	}

	// Check finalizers
	if len(resource.Metadata.Finalizers) > 0 {
		return fmt.Errorf("resource %s/%s has pending finalizers", kind, name)
	}

	delete(resources, name)

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
