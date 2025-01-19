package state

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type OwnershipStoreTestSuite struct {
	baseStoreTestSuite
	ownerStore Store
}

func TestOwnershipStoreSuite(t *testing.T) {
	suite.Run(t, new(OwnershipStoreTestSuite))
}

func (s *OwnershipStoreTestSuite) SetupTest() {
	s.baseStoreTestSuite.SetupTest()
	s.ownerStore = NewOwnershipStore(s.store, "test-owner")
}

func (s *OwnershipStoreTestSuite) TestOwnershipEnforcement() {
	// Test creation sets owner
	iface := s.createTestNetworkInterface("eth0")
	err := s.ownerStore.Create(s.ctx, iface)
	s.Require().NoError(err)

	// Verify owner was set
	created, err := s.ownerStore.Get(s.ctx, iface.Metadata.Kind, iface.Metadata.Name)
	s.Require().NoError(err)
	s.Equal("test-owner", created.Metadata.Owner)

	// Test update preserves owner
	iface.Metadata.Owner = "different-owner"
	err = s.ownerStore.Update(s.ctx, iface)
	s.Require().NoError(err)

	updated, err := s.ownerStore.Get(s.ctx, iface.Metadata.Kind, iface.Metadata.Name)
	s.Require().NoError(err)
	s.Equal("test-owner", updated.Metadata.Owner)
}

func (s *OwnershipStoreTestSuite) TestOwnershipProtection() {
	// Create resource with different owner
	iface := s.createTestNetworkInterface("eth0")
	iface.Metadata.Owner = "other-owner"
	err := s.store.Create(s.ctx, iface) // Create directly in store to bypass ownership
	s.Require().NoError(err)

	// Test update fails for different owner
	err = s.ownerStore.Update(s.ctx, iface)
	s.Require().Error(err)
	ownerErr, ok := err.(*OwnershipError)
	s.Require().True(ok, "expected OwnershipError")
	s.Equal("other-owner", ownerErr.Owner)
	s.Equal("update", ownerErr.Action)
	s.Equal(iface.Metadata.Kind, ownerErr.ResourceKind)
	s.Equal(iface.Metadata.Name, ownerErr.ResourceName)

	// Test delete fails for different owner
	err = s.ownerStore.Delete(s.ctx, iface.Metadata.Kind, iface.Metadata.Name)
	s.Require().Error(err)
	ownerErr, ok = err.(*OwnershipError)
	s.Require().True(ok, "expected OwnershipError")
	s.Equal("other-owner", ownerErr.Owner)
	s.Equal("delete", ownerErr.Action)
	s.Equal(iface.Metadata.Kind, ownerErr.ResourceKind)
	s.Equal(iface.Metadata.Name, ownerErr.ResourceName)
}

func (s *OwnershipStoreTestSuite) TestReadOperations() {
	// Test that read operations work regardless of owner
	iface := s.createTestNetworkInterface("eth0")
	iface.Metadata.Owner = "other-owner"
	err := s.store.Create(s.ctx, iface)
	s.Require().NoError(err)

	// Test Get works
	_, err = s.ownerStore.Get(s.ctx, iface.Metadata.Kind, iface.Metadata.Name)
	s.Require().NoError(err)

	// Test List works
	resources, err := s.ownerStore.List(s.ctx, iface.Metadata.Kind)
	s.Require().NoError(err)
	s.Len(resources, 1)

	// Test Watch works
	ch, err := s.ownerStore.Watch(s.ctx, iface.Metadata.Kind)
	s.Require().NoError(err)
	s.NotNil(ch)
}
