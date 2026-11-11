// Package platform hosts OS and environment detection shared by probes.
package platform

import (
	"errors"
	"fmt"
	"net"
	"sort"
)

// ErrUnsupported reports a platform without a v1 implementation.
var ErrUnsupported = errors.New("platform is not supported by egress v1")

// Family selects an address family for route lookup.
type Family int

const (
	FamilyIPv4 Family = iota
	FamilyIPv6
)

// String returns the address family name.
func (f Family) String() string {
	switch f {
	case FamilyIPv4:
		return "ipv4"
	case FamilyIPv6:
		return "ipv6"
	default:
		return "unknown"
	}
}

// Interface is one local network interface observed read-only.
type Interface struct {
	Name      string   `json:"name"`
	Index     int      `json:"index"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
	MTU       int      `json:"mtu"`
	Addresses []string `json:"addresses"`
}

// DefaultRoute is the selected default route for one address family.
type DefaultRoute struct {
	Family    Family `json:"-"`
	Gateway   string `json:"gateway"`
	Interface string `json:"interface"`
}

// ReadInterfaces lists every local interface with its link state and
// addresses. It only reads host state.
func ReadInterfaces() ([]Interface, error) {
	raw, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	interfaces := make([]Interface, 0, len(raw))
	for _, iface := range raw {
		addresses, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("read addresses for %s: %w", iface.Name, err)
		}
		values := make([]string, 0, len(addresses))
		for _, address := range addresses {
			values = append(values, address.String())
		}
		sort.Strings(values)
		interfaces = append(interfaces, Interface{
			Name:      iface.Name,
			Index:     iface.Index,
			Up:        iface.Flags&net.FlagUp != 0,
			Loopback:  iface.Flags&net.FlagLoopback != 0,
			MTU:       iface.MTU,
			Addresses: values,
		})
	}
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Index < interfaces[j].Index
	})
	return interfaces, nil
}
