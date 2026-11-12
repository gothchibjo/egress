package platform

import (
	"sort"
	"strings"
)

// VPNInterface is a tunnel-like interface observed read-only.
type VPNInterface struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	State        string   `json:"state"`
	Addresses    []string `json:"addresses"`
	DefaultRoute bool     `json:"default_route"`
	Source       string   `json:"source"`
}

// NetworkContext is the read-only proxy and tunnel context of the host. It is
// informational: no probe changes behaviour because of it.
type NetworkContext struct {
	Proxies []ProxySetting `json:"proxies"`
	VPN     []VPNInterface `json:"vpn"`
}

// Empty reports whether no proxy or tunnel context was detected.
func (c NetworkContext) Empty() bool {
	return len(c.Proxies) == 0 && len(c.VPN) == 0
}

// DetectNetworkContext reads proxy variables and tunnel-like interfaces. A
// returned error only means the interface list was unavailable; the proxy part
// is still populated.
func DetectNetworkContext() (NetworkContext, error) {
	caps := System()
	return detectNetworkContext(DetectProxyEnv(), caps.Interfaces, caps.DefaultRoute)
}

func detectNetworkContext(
	proxies ProxyConfig,
	interfaces func() ([]Interface, error),
	route func(Family) (DefaultRoute, bool, error),
) (NetworkContext, error) {
	context := NetworkContext{Proxies: proxies.Settings, VPN: []VPNInterface{}}
	if context.Proxies == nil {
		context.Proxies = []ProxySetting{}
	}

	local, err := interfaces()
	if err != nil {
		return context, err
	}

	routed := make(map[string]bool, 2)
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		if defaultRoute, found, routeErr := route(family); routeErr == nil && found {
			routed[defaultRoute.Interface] = true
		}
	}

	for _, iface := range local {
		kind := vpnKind(iface.Name)
		if kind == "" {
			continue
		}
		state := "down"
		if iface.Up {
			state = "up"
		}
		source := "interface name"
		if routed[iface.Name] {
			source = "interface name and default route"
		}
		context.VPN = append(context.VPN, VPNInterface{
			Name:         iface.Name,
			Kind:         kind,
			State:        state,
			Addresses:    append([]string(nil), iface.Addresses...),
			DefaultRoute: routed[iface.Name],
			Source:       source,
		})
	}
	sort.Slice(context.VPN, func(i, j int) bool {
		return context.VPN[i].Name < context.VPN[j].Name
	})
	return context, nil
}

// vpnKind maps a tunnel-like interface name to its documented kind. An empty
// result means the interface is not a tunnel.
func vpnKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "utun"), strings.HasPrefix(lower, "tun"):
		return "tun"
	case strings.HasPrefix(lower, "tap"):
		return "tap"
	case strings.HasPrefix(lower, "wg"):
		return "wireguard"
	case strings.HasPrefix(lower, "ppp"):
		return "ppp"
	case strings.HasPrefix(lower, "ipsec"):
		return "ipsec"
	default:
		return ""
	}
}
