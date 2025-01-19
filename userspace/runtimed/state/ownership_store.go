package state

import (
	"context"
	"fmt"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
)

// OwnershipError represents errors related to resource ownership
type OwnershipError struct {
	ResourceKind string
	ResourceName string
	Owner       string
	Action      string
}

func (e *OwnershipError) Error() string {
	return fmt.Sprintf("cannot %s resource %s/%s owned by %s", 
		e.Action, e.ResourceKind, e.ResourceName, e.Owner)
}

// NewOwnershipError creates a new OwnershipError
func NewOwnershipError(kind, name, owner, action string) error {
	return &OwnershipError{
		ResourceKind: kind,
		ResourceName: name,
		Owner:       owner,
		Action:      action,
	}
}

// OwnershipStore wraps a Store to enforce resource ownership
type OwnershipStore struct {
	store  Store
	owner  string
}

// NewOwnershipStore creates a new OwnershipStore
func NewOwnershipStore(store Store, owner string) Store {
	return &OwnershipStore{
		store: store,
		owner: owner,
	}
}

func (s *OwnershipStore) Get(ctx context.Context, kind, name string) (*pb.Resource, error) {
	return s.store.Get(ctx, kind, name)
}

func (s *OwnershipStore) List(ctx context.Context, kind string) ([]*pb.Resource, error) {
	return s.store.List(ctx, kind)
}

func (s *OwnershipStore) Create(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		return fmt.Errorf("resource metadata is required")
	}
	
	// Set the owner on creation
	if resource.Metadata.Owner != "" && resource.Metadata.Owner != s.owner {
		return fmt.Errorf("cannot create resource with different owner: got %s, expected %s", 
			resource.Metadata.Owner, s.owner)
	}
	resource.Metadata.Owner = s.owner
	
	return s.store.Create(ctx, resource)
}

func (s *OwnershipStore) Update(ctx context.Context, resource *pb.Resource) error {
	if resource.Metadata == nil {
		return fmt.Errorf("resource metadata is required")
	}

	// Check ownership before update
	existing, err := s.store.Get(ctx, resource.Metadata.Kind, resource.Metadata.Name)
	if err != nil {
		return err
	}

	if existing.Metadata.Owner != s.owner {
		return NewOwnershipError(resource.Metadata.Kind, resource.Metadata.Name, 
			existing.Metadata.Owner, "update")
	}

	// Preserve ownership
	resource.Metadata.Owner = s.owner
	
	return s.store.Update(ctx, resource)
}

func (s *OwnershipStore) Delete(ctx context.Context, kind, name string) error {
	// Check ownership before delete
	existing, err := s.store.Get(ctx, kind, name)
	if err != nil {
		return err
	}

	if existing.Metadata.Owner != s.owner {
		return NewOwnershipError(kind, name, existing.Metadata.Owner, "delete")
	}

	return s.store.Delete(ctx, kind, name)
}

func (s *OwnershipStore) Watch(ctx context.Context, kind string) (<-chan Event, error) {
	return s.store.Watch(ctx, kind)
}
