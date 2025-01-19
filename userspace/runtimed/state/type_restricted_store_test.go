package state

import (
	"testing"

	pb "github.com/microrun/microrun/userspace/runtimed/api"
	"github.com/stretchr/testify/suite"
)

type TypeRestrictedStoreTestSuite struct {
	baseStoreTestSuite
	restrictedStore Store
}

func TestTypeRestrictedStoreSuite(t *testing.T) {
	suite.Run(t, new(TypeRestrictedStoreTestSuite))
}

func (s *TypeRestrictedStoreTestSuite) SetupTest() {
	s.baseStoreTestSuite.SetupTest()
	allowedKinds := []string{pb.KindFor[*pb.NetworkInterface]()}
	s.restrictedStore = NewTypeRestrictedStore(s.store, allowedKinds)
}

func (s *TypeRestrictedStoreTestSuite) TestAllowedTypes() {
	// Test operations with allowed type succeed
	iface := s.createTestNetworkInterface("eth0")
	
	// Test Create
	err := s.restrictedStore.Create(s.ctx, iface)
	s.Require().NoError(err)

	// Test Get
	_, err = s.restrictedStore.Get(s.ctx, iface.Metadata.Kind, iface.Metadata.Name)
	s.Require().NoError(err)

	// Test List
	resources, err := s.restrictedStore.List(s.ctx, iface.Metadata.Kind)
	s.Require().NoError(err)
	s.Len(resources, 1)

	// Test Update
	err = s.restrictedStore.Update(s.ctx, iface)
	s.Require().NoError(err)

	// Test Watch
	ch, err := s.restrictedStore.Watch(s.ctx, iface.Metadata.Kind)
	s.Require().NoError(err)
	s.NotNil(ch)

	// Test Delete
	err = s.restrictedStore.Delete(s.ctx, iface.Metadata.Kind, iface.Metadata.Name)
	s.Require().NoError(err)
}

func (s *TypeRestrictedStoreTestSuite) TestDisallowedTypes() {
	// Create a resource of a different type
	resource := &pb.Resource{
		Metadata: &pb.ResourceMetadata{
			Kind: "DisallowedType",
			Name: "test",
		},
	}

	// Test Create fails
	err := s.restrictedStore.Create(s.ctx, resource)
	s.Require().Error(err)
	typeErr, ok := err.(*TypeRestrictedError)
	s.Require().True(ok, "expected TypeRestrictedError")
	s.Equal("DisallowedType", typeErr.ResourceKind)
	s.Equal("create", typeErr.Action)

	// Create directly in store to test other operations
	err = s.store.Create(s.ctx, resource)
	s.Require().NoError(err)

	// Test Get fails
	_, err = s.restrictedStore.Get(s.ctx, resource.Metadata.Kind, resource.Metadata.Name)
	s.Require().Error(err)
	typeErr, ok = err.(*TypeRestrictedError)
	s.Require().True(ok, "expected TypeRestrictedError")
	s.Equal("DisallowedType", typeErr.ResourceKind)
	s.Equal("get", typeErr.Action)

	// Test List fails
	_, err = s.restrictedStore.List(s.ctx, resource.Metadata.Kind)
	s.Require().Error(err)
	typeErr, ok = err.(*TypeRestrictedError)
	s.Require().True(ok, "expected TypeRestrictedError")
	s.Equal("DisallowedType", typeErr.ResourceKind)
	s.Equal("list", typeErr.Action)

	// Test Update fails
	err = s.restrictedStore.Update(s.ctx, resource)
	s.Require().Error(err)
	typeErr, ok = err.(*TypeRestrictedError)
	s.Require().True(ok, "expected TypeRestrictedError")
	s.Equal("DisallowedType", typeErr.ResourceKind)
	s.Equal("update", typeErr.Action)

	// Test Watch fails
	_, err = s.restrictedStore.Watch(s.ctx, resource.Metadata.Kind)
	s.Require().Error(err)
	typeErr, ok = err.(*TypeRestrictedError)
	s.Require().True(ok, "expected TypeRestrictedError")
	s.Equal("DisallowedType", typeErr.ResourceKind)
	s.Equal("watch", typeErr.Action)

	// Test Delete fails
	err = s.restrictedStore.Delete(s.ctx, resource.Metadata.Kind, resource.Metadata.Name)
	s.Require().Error(err)
	typeErr, ok = err.(*TypeRestrictedError)
	s.Require().True(ok, "expected TypeRestrictedError")
	s.Equal("DisallowedType", typeErr.ResourceKind)
	s.Equal("delete", typeErr.Action)
}
