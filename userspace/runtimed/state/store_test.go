package state

import (
	"context"
	"testing"
	"time"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
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
	return &pb.Resource{
		Metadata: &pb.ResourceMetadata{
			Kind: pb.KindFor[*pb.NetworkInterface](),
			Name: name,
		},
		Spec: &pb.Resource_NetworkInterface{
			NetworkInterface: &pb.NetworkInterface{
				InterfaceName: name,
				MacAddress:    "00:11:22:33:44:55",
			},
		},
	}
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
	got, err := s.store.Get(s.ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
	s.Require().NoError(err, "Get should succeed")
	s.Assert().True(proto.Equal(got, iface), "Get should return equal resource")

	// Test Update
	updatedIface := proto.Clone(iface).(*pb.Resource)
	updatedIface.GetNetworkInterface().MacAddress = "aa:bb:cc:dd:ee:ff"
	err = s.store.Update(s.ctx, updatedIface)
	s.Require().NoError(err, "Update should succeed")

	got, err = s.store.Get(s.ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
	s.Require().NoError(err, "Get after update should succeed")
	s.Assert().Equal("aa:bb:cc:dd:ee:ff", got.GetNetworkInterface().MacAddress, "Update should persist changes")
	s.Assert().Equal(int64(1), got.Metadata.Generation, "Update should increment generation")

	// Test List
	resources, err := s.store.List(s.ctx, pb.KindFor[*pb.NetworkInterface]())
	s.Require().NoError(err, "List should succeed")
	s.Assert().Len(resources, 1, "List should return one resource")

	// Test Delete
	err = s.store.Delete(s.ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
	s.Require().NoError(err, "Delete should succeed")

	// Verify deletion
	_, err = s.store.Get(s.ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
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
	err = s.store.Delete(s.ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
	s.Assert().Error(err, "Delete with finalizers should fail")

	// Remove finalizers and try again
	update := proto.Clone(iface).(*pb.Resource)
	update.Metadata.Finalizers = nil
	err = s.store.Update(s.ctx, update)
	s.Require().NoError(err, "Update to remove finalizers should succeed")

	err = s.store.Delete(s.ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
	s.Assert().NoError(err, "Delete after removing finalizers should succeed")
}

func (s *StoreTestSuite) TestWatch() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// Start watching
	events, err := s.store.Watch(ctx, pb.KindFor[*pb.NetworkInterface]())
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
	err = s.store.Delete(ctx, pb.KindFor[*pb.NetworkInterface](), "eth0")
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
	_, err := s.store.Get(s.ctx, pb.KindFor[*pb.NetworkInterface](), "nonexistent")
	s.Assert().Error(err, "Get nonexistent resource should fail")

	// Test Update
	iface := s.createTestNetworkInterface("nonexistent")
	err = s.store.Update(s.ctx, iface)
	s.Assert().Error(err, "Update nonexistent resource should fail")

	// Test Delete
	err = s.store.Delete(s.ctx, pb.KindFor[*pb.NetworkInterface](), "nonexistent")
	s.Assert().Error(err, "Delete nonexistent resource should fail")
}
