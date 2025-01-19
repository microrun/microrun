package state

import (
	"context"
	"testing"
	"time"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type TypedStoreTestSuite struct {
	suite.Suite
	ctx   context.Context
	store *TypedStore[*pb.NetworkInterface]
}

func TestTypedStoreSuite(t *testing.T) {
	suite.Run(t, new(TypedStoreTestSuite))
}

func (s *TypedStoreTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = NewTypedStore[*pb.NetworkInterface](NewMemoryStore())
}

func (s *TypedStoreTestSuite) createNetworkInterface(name string) *pb.NetworkInterface {
	return &pb.NetworkInterface{
		InterfaceName: name,
		MacAddress:    "00:11:22:33:44:55",
	}
}

func (s *TypedStoreTestSuite) TestTypedOperations() {
	iface := s.createNetworkInterface("eth0")

	// Test Create
	err := s.store.Create(s.ctx, "eth0", iface)
	s.Require().NoError(err, "Create should succeed")

	// Test Get
	got, err := s.store.Get(s.ctx, "eth0")
	s.Require().NoError(err, "Get should succeed")
	s.Assert().True(proto.Equal(got.Spec(), iface), "Get should return equal spec")
	s.Assert().Equal(pb.KindFor[*pb.NetworkInterface](), got.Resource().Metadata.Kind, "Resource should have correct kind")

	// Test Update
	updatedIface := proto.Clone(iface).(*pb.NetworkInterface)
	updatedIface.MacAddress = "aa:bb:cc:dd:ee:ff"
	err = s.store.Update(s.ctx, "eth0", updatedIface)
	s.Require().NoError(err, "Update should succeed")

	got, err = s.store.Get(s.ctx, "eth0")
	s.Require().NoError(err, "Get after update should succeed")
	s.Assert().Equal("aa:bb:cc:dd:ee:ff", got.Spec().MacAddress, "Update should persist changes")
	s.Assert().Equal(int64(1), got.Resource().Metadata.Generation, "Update should increment generation")

	// Test List
	resources, err := s.store.List(s.ctx)
	s.Require().NoError(err, "List should succeed")
	s.Assert().Len(resources, 1, "List should return one resource")
	s.Assert().True(proto.Equal(resources[0].Spec(), updatedIface), "Listed resource should match updated spec")

	// Test Delete
	err = s.store.Delete(s.ctx, "eth0")
	s.Require().NoError(err, "Delete should succeed")

	// Verify deletion
	_, err = s.store.Get(s.ctx, "eth0")
	s.Assert().Error(err, "Get after deletion should fail")
}

func (s *TypedStoreTestSuite) TestTypedWatch() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	// Start watching
	events, err := s.store.Watch(ctx)
	s.Require().NoError(err, "Watch should succeed")

	// Create resource
	iface := s.createNetworkInterface("eth0")
	err = s.store.Create(ctx, "eth0", iface)
	s.Require().NoError(err, "Create should succeed")

	// Test create event
	select {
	case event := <-events:
		s.Assert().True(proto.Equal(event.Spec(), iface), "Event should contain created resource")
	case <-time.After(time.Second):
		s.T().Fatal("Timeout waiting for create event")
	}

	// Update resource
	update := proto.Clone(iface).(*pb.NetworkInterface)
	update.MacAddress = "aa:bb:cc:dd:ee:ff"
	err = s.store.Update(ctx, "eth0", update)
	s.Require().NoError(err, "Update should succeed")

	// Test update event
	select {
	case event := <-events:
		s.Assert().True(proto.Equal(event.Spec(), update), "Event should contain updated resource")
	case <-time.After(time.Second):
		s.T().Fatal("Timeout waiting for update event")
	}

	// Delete resource
	err = s.store.Delete(ctx, "eth0")
	s.Require().NoError(err, "Delete should succeed")

	// Test delete event
	select {
	case event := <-events:
		s.Assert().True(proto.Equal(event.Spec(), update), "Event should contain deleted resource")
	case <-time.After(time.Second):
		s.T().Fatal("Timeout waiting for delete event")
	}

	// Test context cancellation
	cancel()
	time.Sleep(100 * time.Millisecond)
	_, ok := <-events
	s.Assert().False(ok, "Channel should be closed after context cancellation")
}
