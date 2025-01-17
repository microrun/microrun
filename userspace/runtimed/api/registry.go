//go:generate go run ../../../tools/gen-registry/main.go

// Code generated by gen-registry. DO NOT EDIT.2

package api

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

// Resource kind constants
const (
	KindNetworkInterface = "NetworkInterface"
	KindDHCPClient = "DHCPClient"
)

// KindFor returns the resource kind for a specific type
func KindFor[T proto.Message]() string {
	var zero T
	switch any(zero).(type) {
	case *NetworkInterface:
		return KindNetworkInterface
	case *DHCPClient:
		return KindDHCPClient
	default:
		panic("unregistered type")
	}
}

// ExtractSpec extracts the typed spec from a resource using the protobuf type system
func ExtractSpec[T proto.Message](resource *Resource) (T, error) {
    var zero T
    switch any(zero).(type) {
	case *NetworkInterface:
		if spec := resource.GetNetworkInterface(); spec != nil {
			return any(spec).(T), nil
		}
	case *DHCPClient:
		if spec := resource.GetDhcpClient(); spec != nil {
			return any(spec).(T), nil
		}
    }
    return zero, fmt.Errorf("resource does not contain spec of type %T", zero)
}

// SetSpec sets the spec field in a resource based on the type
func SetSpec[T proto.Message](resource *Resource, spec T) error {
	switch s := any(spec).(type) {
	case *NetworkInterface:
		resource.Spec = &Resource_NetworkInterface{NetworkInterface: s}
		return nil
	case *DHCPClient:
		resource.Spec = &Resource_DhcpClient{DhcpClient: s}
		return nil
	default:
		return fmt.Errorf("unsupported resource type: %T", spec)
	}
}
