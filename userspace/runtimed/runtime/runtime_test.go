package runtime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/microrun/microrun/userspace/runtimed/api"
	"github.com/microrun/microrun/userspace/runtimed/logging"
	"github.com/microrun/microrun/userspace/runtimed/state"
)

// mockGeneratorType implements GeneratorType for testing
type mockGeneratorType struct{}

func (t *mockGeneratorType) Name() string {
	return "test-generator"
}

func (t *mockGeneratorType) ManagedKinds() []string {
	return []string{api.KindFor[*api.NetworkInterface]()}
}

func (t *mockGeneratorType) New(ctx GeneratorContext) (Generator, error) {
	return &mockGenerator{
		ifaceStore: state.NewTypedStore[*api.NetworkInterface](ctx.Store),
		dhcpStore:  state.NewTypedStore[*api.DHCPClient](ctx.Store),
		logger:     ctx.Logger,
	}, nil
}

// mockGenerator implements Generator for testing
type mockGenerator struct {
	ifaceStore  *state.TypedStore[*api.NetworkInterface]
	dhcpStore   *state.TypedStore[*api.DHCPClient]
	logger      *logging.Logger
}

func (g *mockGenerator) Run(ctx context.Context) error {
	return nil
}

type RuntimeTestSuite struct {
	suite.Suite
	ctx    context.Context
	store  state.Store
	logger *logging.Logger
}

func TestRuntimeSuite(t *testing.T) {
	suite.Run(t, new(RuntimeTestSuite))
}

func (s *RuntimeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.store = state.NewMemoryStore()
	s.logger = logging.NewLogger("test", logging.ComponentController)
}

func (s *RuntimeTestSuite) TestStoreRestrictions() {
	// Create runtime
	rt := New(s.store)

	// Create mock generator type
	genType := &mockGeneratorType{}

	// Register generator
	err := rt.RegisterGenerator(genType)
	s.Require().NoError(err)

	// Get the wrapped generator from the runtime to test with
	wrappedGen := rt.generators[genType.Name()].(*mockGenerator)

	// Try to create a resource of the allowed type through generator's store
	iface := &api.NetworkInterface{
		InterfaceName: "eth0",
		MacAddress:    "00:11:22:33:44:55",
	}
	err = wrappedGen.ifaceStore.Create(s.ctx, "eth0", iface)
	s.Require().NoError(err)

	// Verify owner was set through runtime's store
	created, err := rt.store.Get(s.ctx, api.KindFor[*api.NetworkInterface](), "eth0")
	s.Require().NoError(err)
	s.Equal("test-generator", created.Metadata.Owner)

	// Try to create a DHCP client resource - this should fail since it's not in ManagedKinds
	dhcpClient := &api.DHCPClient{
		InterfaceRef: "eth0",
		Enabled:      true,
	}
	err = wrappedGen.dhcpStore.Create(s.ctx, "client1", dhcpClient)
	s.Require().Error(err)
	typeErr, ok := err.(*state.TypeRestrictedError)
	s.Require().True(ok)
	s.Equal(api.KindFor[*api.DHCPClient](), typeErr.ResourceKind)
	s.Equal("create", typeErr.Action)

	// Try to modify a resource owned by another generator
	otherIface := &api.NetworkInterface{
		InterfaceName: "eth1",
		MacAddress:    "00:11:22:33:44:66",
	}
	err = rt.store.Create(s.ctx, &api.Resource{
		Metadata: &api.ResourceMetadata{
			Kind: api.KindFor[*api.NetworkInterface](),
			Name: "eth1",
			Owner: "other-generator",
		},
		Spec: &api.Resource_NetworkInterface{
			NetworkInterface: otherIface,
		},
	})
	s.Require().NoError(err)

	// Try to modify through the interface store
	err = wrappedGen.ifaceStore.Update(s.ctx, "eth1", otherIface)
	s.Require().Error(err)
	ownerErr, ok := err.(*state.OwnershipError)
	s.Require().True(ok)
	s.Equal("other-generator", ownerErr.Owner)
	s.Equal("update", ownerErr.Action)

	// Verify runtime's store can still access everything without restrictions
	err = rt.store.Create(s.ctx, &api.Resource{
		Metadata: &api.ResourceMetadata{
			Kind: api.KindFor[*api.DHCPClient](),
			Name: "unrestricted",
		},
		Spec: &api.Resource_DhcpClient{
			DhcpClient: &api.DHCPClient{
				InterfaceRef: "eth0",
				Enabled:      true,
			},
		},
	})
	s.Require().NoError(err)
}
