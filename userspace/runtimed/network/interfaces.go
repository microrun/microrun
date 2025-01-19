package network

import (
	"context"
	"fmt"
	"time"

	"github.com/vishvananda/netlink"
	"go.uber.org/zap"

	"github.com/microrun/microrun/userspace/runtimed/api"
	"github.com/microrun/microrun/userspace/runtimed/logging"
	"github.com/microrun/microrun/userspace/runtimed/runtime"
	"github.com/microrun/microrun/userspace/runtimed/state"
)

// InterfaceGeneratorType defines the network interface generator type
type InterfaceGeneratorType struct{}

// Name returns the generator name
func (t *InterfaceGeneratorType) Name() string {
	return "network-interfaces"
}

// ManagedKinds returns the resource kinds this generator manages
func (t *InterfaceGeneratorType) ManagedKinds() []string {
	return []string{api.KindFor[*api.NetworkInterface]()}
}

// New creates a new network interface generator instance
func (t *InterfaceGeneratorType) New(ctx runtime.GeneratorContext) (runtime.Generator, error) {
	return &InterfaceGenerator{
		store:  state.NewTypedStore[*api.NetworkInterface](ctx.Store),
		logger: ctx.Logger,
	}, nil
}

// InterfaceGenerator watches system network interfaces and generates NetworkInterface resources
type InterfaceGenerator struct {
	store  *state.TypedStore[*api.NetworkInterface]
	logger *logging.Logger
}

// Run starts the network interface generator
func (g *InterfaceGenerator) Run(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := g.reconcileInterfaces(ctx); err != nil {
				g.logger.Error("Failed to reconcile interfaces", zap.Error(err))
			}
		}
	}
}

// reconcileInterfaces compares system interfaces with stored resources
func (g *InterfaceGenerator) reconcileInterfaces(ctx context.Context) error {
	// Get current system interfaces
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("failed to list network interfaces: %w", err)
	}

	// Get current stored interfaces
	stored, err := g.store.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list stored interfaces: %w", err)
	}

	// Build map of stored interfaces for easy lookup
	storedMap := make(map[string]*api.NetworkInterface)
	for _, iface := range stored {
		storedMap[iface.Spec().InterfaceName] = iface.Spec()
	}

	// Process each system interface
	for _, link := range links {
		attrs := link.Attrs()
		if attrs == nil {
			continue
		}

		name := attrs.Name
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			g.logger.Error("Failed to list addresses",
				zap.String("interface", name),
				zap.Error(err))
			continue
		}

		iface := &api.NetworkInterface{
			InterfaceName: name,
			MacAddress:    attrs.HardwareAddr.String(),
		}
		for _, addr := range addrs {
			iface.IpAddresses = append(iface.IpAddresses, addr.IPNet.String())
		}

		// First try to get the existing interface
		_, err = g.store.Get(ctx, name)
		if err == nil {
			// Interface exists, update it
			if err := g.store.Update(ctx, name, iface); err != nil {
				g.logger.Error("Failed to update interface",
					zap.String("name", name),
					zap.Error(err))
			}
		} else {
			// Interface doesn't exist, create it
			if err := g.store.Create(ctx, name, iface); err != nil {
				g.logger.Error("Failed to create interface",
					zap.String("name", name),
					zap.Error(err))
			}
		}
		delete(storedMap, name)
	}

	// Delete interfaces that no longer exist
	for name := range storedMap {
		if err := g.store.Delete(ctx, name); err != nil {
			g.logger.Error("Failed to delete interface",
				zap.String("name", name),
				zap.Error(err))
		}
	}

	return nil
}

// sliceEqual returns true if two string slices have the same elements in the same order
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
