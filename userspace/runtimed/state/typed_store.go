package state

import (
	"context"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
	"google.golang.org/protobuf/proto"
)

// TypedResource provides type-safe access to a resource and its spec
type TypedResource[T proto.Message] struct {
	resource *pb.Resource
}

// Resource returns the underlying resource
func (t *TypedResource[T]) Resource() *pb.Resource {
	return t.resource
}

// Spec returns the typed spec of the resource
func (t *TypedResource[T]) Spec() T {
	spec, err := pb.ExtractSpec[T](t.resource)
	if err != nil {
		panic(err) // This should never happen as we validate on creation
	}
	return spec
}

// TypedStore provides type-safe access to resources
type TypedStore[T proto.Message] struct {
	store Store
}

// NewTypedStore creates a new TypedStore
func NewTypedStore[T proto.Message](store Store) *TypedStore[T] {
	return &TypedStore[T]{store: store}
}

// Get retrieves a typed resource by name
func (t *TypedStore[T]) Get(ctx context.Context, name string) (*TypedResource[T], error) {
	kind := pb.KindFor[T]()
	resource, err := t.store.Get(ctx, kind, name)
	if err != nil {
		return nil, err
	}
	return &TypedResource[T]{resource: resource}, nil
}

// List retrieves all resources of this type
func (t *TypedStore[T]) List(ctx context.Context) ([]*TypedResource[T], error) {
	kind := pb.KindFor[T]()
	resources, err := t.store.List(ctx, kind)
	if err != nil {
		return nil, err
	}

	result := make([]*TypedResource[T], len(resources))
	for i, resource := range resources {
		result[i] = &TypedResource[T]{resource: resource}
	}
	return result, nil
}

// Create creates a new resource
func (t *TypedStore[T]) Create(ctx context.Context, name string, spec T) error {
	kind := pb.KindFor[T]()
	resource := &pb.Resource{
		Metadata: &pb.ResourceMetadata{
			Kind: kind,
			Name: name,
		},
	}
	if err := pb.SetSpec(resource, spec); err != nil {
		return err
	}
	return t.store.Create(ctx, resource)
}

// Update updates an existing resource
func (t *TypedStore[T]) Update(ctx context.Context, name string, spec T) error {
	kind := pb.KindFor[T]()
	resource := &pb.Resource{
		Metadata: &pb.ResourceMetadata{
			Kind: kind,
			Name: name,
		},
	}
	if err := pb.SetSpec(resource, spec); err != nil {
		return err
	}
	return t.store.Update(ctx, resource)
}

// Delete removes a resource
func (t *TypedStore[T]) Delete(ctx context.Context, name string) error {
	kind := pb.KindFor[T]()
	return t.store.Delete(ctx, kind, name)
}

// Watch provides a channel of resource changes
func (t *TypedStore[T]) Watch(ctx context.Context) (<-chan *TypedResource[T], error) {
	kind := pb.KindFor[T]()
	events, err := t.store.Watch(ctx, kind)
	if err != nil {
		return nil, err
	}

	ch := make(chan *TypedResource[T], 100)
	go func() {
		defer close(ch)
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return
				}
				select {
				case ch <- &TypedResource[T]{resource: event.Resource}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}
