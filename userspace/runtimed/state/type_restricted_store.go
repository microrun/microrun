package state

import (
	"context"
	"fmt"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
)

// TypeRestrictedError represents errors related to disallowed resource types
type TypeRestrictedError struct {
	ResourceKind string
	Action      string
}

func (e *TypeRestrictedError) Error() string {
	return fmt.Sprintf("access to resource kind %q is not allowed for operation: %s", 
		e.ResourceKind, e.Action)
}

// NewTypeRestrictedError creates a new TypeRestrictedError
func NewTypeRestrictedError(kind, action string) error {
	return &TypeRestrictedError{
		ResourceKind: kind,
		Action:      action,
	}
}

// TypeRestrictedStore wraps a Store to enforce access to only allowed resource types
type TypeRestrictedStore struct {
	store        Store
	allowedKinds map[string]struct{}
}

// NewTypeRestrictedStore creates a new TypeRestrictedStore that only allows access to the specified kinds
func NewTypeRestrictedStore(store Store, allowedKinds []string) Store {
	allowed := make(map[string]struct{}, len(allowedKinds))
	for _, kind := range allowedKinds {
		allowed[kind] = struct{}{}
	}
	return &TypeRestrictedStore{
		store:        store,
		allowedKinds: allowed,
	}
}

func (s *TypeRestrictedStore) checkKindAllowed(kind, action string) error {
	if _, ok := s.allowedKinds[kind]; !ok {
		return NewTypeRestrictedError(kind, action)
	}
	return nil
}

func (s *TypeRestrictedStore) Get(ctx context.Context, kind, name string) (*pb.Resource, error) {
	if err := s.checkKindAllowed(kind, "get"); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, kind, name)
}

func (s *TypeRestrictedStore) List(ctx context.Context, kind string) ([]*pb.Resource, error) {
	if err := s.checkKindAllowed(kind, "list"); err != nil {
		return nil, err
	}
	return s.store.List(ctx, kind)
}

func (s *TypeRestrictedStore) Create(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		return fmt.Errorf("resource metadata is required")
	}
	if err := s.checkKindAllowed(resource.Metadata.Kind, "create"); err != nil {
		return err
	}
	return s.store.Create(ctx, resource)
}

func (s *TypeRestrictedStore) Update(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		return fmt.Errorf("resource metadata is required")
	}
	if err := s.checkKindAllowed(resource.Metadata.Kind, "update"); err != nil {
		return err
	}
	return s.store.Update(ctx, resource)
}

func (s *TypeRestrictedStore) Delete(ctx context.Context, kind, name string) error {
	if err := s.checkKindAllowed(kind, "delete"); err != nil {
		return err
	}
	return s.store.Delete(ctx, kind, name)
}

func (s *TypeRestrictedStore) Watch(ctx context.Context, kind string) (<-chan Event, error) {
	if err := s.checkKindAllowed(kind, "watch"); err != nil {
		return nil, err
	}
	return s.store.Watch(ctx, kind)
}
