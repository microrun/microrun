package state

import (
	"context"
	"testing"
	"time"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

// baseStoreTestSuite provides common functionality for store tests
type baseStoreTestSuite struct {
	suite.Suite
	ctx   context.Context
	store Store
}

func (s *baseStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = NewMemoryStore()
}

func (s *baseStoreTestSuite) createTestNetworkInterface(name string) *pb.Resource {
	iface := &pb.NetworkInterface{
		InterfaceName: name,
		IpAddresses:   []string{"192.168.1.1"},
	}
	resource := &pb.Resource{
		Metadata: &pb.ResourceMetadata{
			Kind: "NetworkInterface",
			Name: name,
		},
	}
	require.NoError(s.T(), pb.SetSpec(resource, iface))
	return resource
}

// StoreTestSuite tests the raw Store interface
type StoreTestSuite struct {
	baseStoreTestSuite
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) TestBasicOperations() {
	iface := s.createTestNetworkInterface("eth0")

	// Test Create
	err := s.store.Create(s.ctx, iface)
	s.Require().NoError(err, "Create should succeed")

	// Test duplicate creation
	err = s.store.Create(s.ctx, iface)
	s.Assert().Error(err, "Create with duplicate name should fail")

	// Test Get
	got, err := s.store.Get(s.ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err, "Get should succeed")
	s.Assert().True(proto.Equal(got, iface), "Get should return equal resource")

	// Test Update
	updatedIface := proto.Clone(iface).(*pb.Resource)
	updatedIface.GetNetworkInterface().MacAddress = "aa:bb:cc:dd:ee:ff"
	err = s.store.Update(s.ctx, updatedIface)
	s.Require().NoError(err, "Update should succeed")

	got, err = s.store.Get(s.ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err, "Get after update should succeed")
	s.Assert().Equal("aa:bb:cc:dd:ee:ff", got.GetNetworkInterface().MacAddress, "Update should persist changes")
	s.Assert().Equal(int64(1), got.Metadata.Generation, "Update should increment generation")

	// Test List
	resources, err := s.store.List(s.ctx, "NetworkInterface")
	s.Require().NoError(err, "List should succeed")
	s.Assert().Len(resources, 1, "List should return one resource")

	// Test Delete
	err = s.store.Delete(s.ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err, "Delete should succeed")

	// Verify deletion
	_, err = s.store.Get(s.ctx, "NetworkInterface", "eth0")
	s.Assert().Error(err, "Get after deletion should fail")
}

func (s *StoreTestSuite) TestOwnership() {
	iface := s.createTestNetworkInterface("eth0")
	iface.Metadata.Owner = "user1"

	err := s.store.Create(s.ctx, iface)
	s.Require().NoError(err, "Create with owner should succeed")

	// Try to update with different owner
	update := proto.Clone(iface).(*pb.Resource)
	update.Metadata.Owner = "user2"
	err = s.store.Update(s.ctx, update)
	s.Assert().Error(err, "Update with different owner should fail")

	// Update with same owner
	update.Metadata.Owner = "user1"
	err = s.store.Update(s.ctx, update)
	s.Assert().NoError(err, "Update with same owner should succeed")
}

func (s *StoreTestSuite) TestFinalizers() {
	iface := s.createTestNetworkInterface("eth0")
	iface.Metadata.Finalizers = []string{"cleanup-routes"}

	err := s.store.Create(s.ctx, iface)
	s.Require().NoError(err, "Create with finalizers should succeed")

	// Try to delete with finalizers
	err = s.store.Delete(s.ctx, "NetworkInterface", "eth0")
	s.Assert().Error(err, "Delete with finalizers should fail")

	// Remove finalizers and try again
	update := proto.Clone(iface).(*pb.Resource)
	update.Metadata.Finalizers = nil
	err = s.store.Update(s.ctx, update)
	s.Require().NoError(err, "Update to remove finalizers should succeed")

	err = s.store.Delete(s.ctx, "NetworkInterface", "eth0")
	s.Assert().NoError(err, "Delete after removing finalizers should succeed")
}

func (s *StoreTestSuite) TestWatch() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// Start watching
	events, err := s.store.Watch(ctx, "NetworkInterface")
	s.Require().NoError(err, "Watch should succeed")

	// Create resource
	iface := s.createTestNetworkInterface("eth0")
	err = s.store.Create(ctx, iface)
	s.Require().NoError(err, "Create should succeed")

	// Test create event
	select {
	case event := <-events:
		s.Assert().Equal(EventCreated, event.Type, "Should receive create event")
		s.Assert().True(proto.Equal(event.Resource, iface), "Event should contain created resource")
	case <-time.After(time.Second):
		s.T().Fatal("Timeout waiting for create event")
	}

	// Update resource
	update := proto.Clone(iface).(*pb.Resource)
	update.GetNetworkInterface().MacAddress = "aa:bb:cc:dd:ee:ff"
	err = s.store.Update(ctx, update)
	s.Require().NoError(err, "Update should succeed")

	// Test update event
	select {
	case event := <-events:
		s.Assert().Equal(EventUpdated, event.Type, "Should receive update event")
		s.Assert().True(proto.Equal(event.Resource, update), "Event should contain updated resource")
	case <-time.After(time.Second):
		s.T().Fatal("Timeout waiting for update event")
	}

	// Delete resource
	err = s.store.Delete(ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err, "Delete should succeed")

	// Test delete event
	select {
	case event := <-events:
		s.Assert().Equal(EventDeleted, event.Type, "Should receive delete event")
	case <-time.After(time.Second):
		s.T().Fatal("Timeout waiting for delete event")
	}

	// Test context cancellation
	cancel()
	time.Sleep(100 * time.Millisecond)
	_, ok := <-events
	s.Assert().False(ok, "Channel should be closed after context cancellation")
}

func (s *StoreTestSuite) TestNonExistentResources() {
	// Test Get
	_, err := s.store.Get(s.ctx, "NetworkInterface", "nonexistent")
	s.Assert().Error(err, "Get nonexistent resource should fail")

	// Test Update
	iface := s.createTestNetworkInterface("nonexistent")
	err = s.store.Update(s.ctx, iface)
	s.Assert().Error(err, "Update nonexistent resource should fail")

	// Test Delete
	err = s.store.Delete(s.ctx, "NetworkInterface", "nonexistent")
	s.Assert().Error(err, "Delete nonexistent resource should fail")
}

func (s *StoreTestSuite) TestNoOpUpdate() {
	// Create initial resource
	iface := &pb.NetworkInterface{
		InterfaceName: "eth0",
		IpAddresses:   []string{"192.168.1.1/24"},
	}
	resource := &pb.Resource{
		Metadata: &pb.ResourceMetadata{
			Kind:  "NetworkInterface",
			Name:  "eth0",
			Owner: "test",
		},
	}
	require.NoError(s.T(), pb.SetSpec(resource, iface))
	err := s.store.Create(s.ctx, resource)
	s.Require().NoError(err)

	// Get the resource to check initial generation
	created, err := s.store.Get(s.ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err)
	initialGen := created.Metadata.Generation

	// Set up watch to verify no events are sent
	events, err := s.store.Watch(s.ctx, "NetworkInterface")
	s.Require().NoError(err)

	// Update with identical resource
	identicalResource := proto.Clone(resource).(*pb.Resource)
	err = s.store.Update(s.ctx, identicalResource)
	s.Require().NoError(err)

	// Verify no update event was sent
	select {
	case event := <-events:
		s.Failf("Unexpected event", "Got event %v for identical update", event)
	case <-time.After(100 * time.Millisecond):
		// No event received, as expected
	}

	// Verify generation was not incremented for no-op update
	unchanged, err := s.store.Get(s.ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err)
	s.Equal(initialGen, unchanged.Metadata.Generation, "Generation should not change for no-op update")

	// Verify update with actual changes does generate event
	changedResource := proto.Clone(resource).(*pb.Resource)
	changedResource.Spec.(*pb.Resource_NetworkInterface).NetworkInterface.MacAddress = "aa:bb:cc:dd:ee:ff"
	err = s.store.Update(s.ctx, changedResource)
	s.Require().NoError(err)

	// Verify update event was sent
	select {
	case event := <-events:
		s.Equal(EventUpdated, event.Type)
		s.True(proto.Equal(changedResource, event.Resource), "Changed resource should match event resource")
	case <-time.After(100 * time.Millisecond):
		s.Fail("Expected update event not received")
	}

	// Verify generation was incremented for real update
	changed, err := s.store.Get(s.ctx, "NetworkInterface", "eth0")
	s.Require().NoError(err)
	s.Equal(initialGen + 1, changed.Metadata.Generation, "Generation should increment for real update")
}

func TestMemoryStoreUpdate(t *testing.T) {
	t.Run("identical update should be no-op", func(t *testing.T) {
		store := NewMemoryStore()
		ctx := context.Background()

		// Create initial resource
		iface := &pb.NetworkInterface{
			InterfaceName: "test",
			IpAddresses: []string{"192.168.1.1"},
		}
		resource := &pb.Resource{
			Metadata: &pb.ResourceMetadata{
				Kind: "NetworkInterface",
				Name: "test",
			},
		}
		require.NoError(t, pb.SetSpec(resource, iface))
		err := store.Create(ctx, resource)
		require.NoError(t, err)

		// Get the resource to check initial generation
		created, err := store.Get(ctx, "NetworkInterface", "test")
		require.NoError(t, err)
		initialGen := created.Metadata.Generation

		// Update with identical data
		identical := proto.Clone(created).(*pb.Resource)
		err = store.Update(ctx, identical)
		require.NoError(t, err)

		// Get the resource again and verify generation hasn't changed
		updated, err := store.Get(ctx, "NetworkInterface", "test")
		require.NoError(t, err)
		assert.Equal(t, initialGen, updated.Metadata.Generation, 
			"generation should not change for identical update")

		// Make an actual change
		identical.GetNetworkInterface().IpAddresses = []string{"192.168.1.2"}
		err = store.Update(ctx, identical)
		require.NoError(t, err)

		// Verify generation was incremented for real change
		changed, err := store.Get(ctx, "NetworkInterface", "test")
		require.NoError(t, err)
		assert.Equal(t, initialGen + 1, changed.Metadata.Generation,
			"generation should increment for actual change")
	})
}
