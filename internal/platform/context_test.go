package platform

import (
	"errors"
	"strings"
	"testing"
)

func contextInterfaces() []Interface {
	return []Interface{
		{Name: "en0", Index: 1, Up: true, Addresses: []string{"192.0.2.10/24"}},
		{Name: "utun3", Index: 2, Up: true, Addresses: []string{"10.8.0.2/32"}},
		{Name: "tap0", Index: 3, Up: false, Addresses: []string{"10.9.0.2/24"}},
		{Name: "wg0", Index: 4, Up: true, Addresses: []string{"10.10.0.2/32"}},
	}
}

func contextRoute(family Family) (DefaultRoute, bool, error) {
	if family == FamilyIPv4 {
		return DefaultRoute{Interface: "utun3", Gateway: "10.8.0.1"}, true, nil
	}
	return DefaultRoute{}, false, nil
}

func TestDetectNetworkContext(t *testing.T) {
	proxies := ProxyConfig{Settings: []ProxySetting{
		{Name: "HTTPS_PROXY", Source: "https_proxy", Value: "http://proxy.example:8080"},
	}}
	context, err := detectNetworkContext(proxies, func() ([]Interface, error) {
		return contextInterfaces(), nil
	}, contextRoute)
	if err != nil {
		t.Fatalf("detectNetworkContext() error = %v", err)
	}

	if len(context.Proxies) != 1 || context.Proxies[0].Source != "https_proxy" {
		t.Fatalf("proxies = %+v", context.Proxies)
	}
	if len(context.VPN) != 3 {
		t.Fatalf("vpn interfaces = %d, want 3: %+v", len(context.VPN), context.VPN)
	}

	byName := make(map[string]VPNInterface, len(context.VPN))
	for _, iface := range context.VPN {
		byName[iface.Name] = iface
	}

	tun, ok := byName["utun3"]
	if !ok {
		t.Fatal("utun3 is missing")
	}
	if tun.Kind != "tun" || tun.State != "up" || !tun.DefaultRoute {
		t.Fatalf("utun3 = %+v", tun)
	}
	if tun.Source != "interface name and default route" {
		t.Fatalf("utun3 source = %q", tun.Source)
	}

	wg, ok := byName["wg0"]
	if !ok {
		t.Fatal("wg0 is missing")
	}
	if wg.Kind != "wireguard" || wg.DefaultRoute {
		t.Fatalf("wg0 = %+v", wg)
	}
	if wg.Source != "interface name" {
		t.Fatalf("wg0 source = %q", wg.Source)
	}

	tap, ok := byName["tap0"]
	if !ok {
		t.Fatal("tap0 is missing")
	}
	if tap.Kind != "tap" || tap.State != "down" {
		t.Fatalf("tap0 = %+v", tap)
	}
	if _, ok := byName["en0"]; ok {
		t.Fatal("en0 was reported as a tunnel")
	}
}

func TestDetectNetworkContextStatesAbsence(t *testing.T) {
	context, err := detectNetworkContext(ProxyConfig{}, func() ([]Interface, error) {
		return []Interface{{Name: "en0", Up: true}}, nil
	}, func(Family) (DefaultRoute, bool, error) { return DefaultRoute{}, false, nil })
	if err != nil {
		t.Fatalf("detectNetworkContext() error = %v", err)
	}
	if !context.Empty() {
		t.Fatalf("context is not empty: %+v", context)
	}
	if context.Proxies == nil || context.VPN == nil {
		t.Fatal("absent context must be an empty array, not null")
	}
}

func TestDetectNetworkContextKeepsProxiesWhenInterfacesFail(t *testing.T) {
	proxies := ProxyConfig{Settings: []ProxySetting{{Name: "HTTP_PROXY", Source: "http_proxy", Value: "http://proxy:3128"}}}
	context, err := detectNetworkContext(proxies, func() ([]Interface, error) {
		return nil, errors.New("interface list unavailable")
	}, contextRoute)
	if err == nil {
		t.Fatal("interface error was swallowed")
	}
	if len(context.Proxies) != 1 {
		t.Fatalf("proxies = %+v", context.Proxies)
	}
}

func TestVPNKind(t *testing.T) {
	tests := map[string]string{
		"utun3":  "tun",
		"tun0":   "tun",
		"tap1":   "tap",
		"wg0":    "wireguard",
		"ppp0":   "ppp",
		"ipsec0": "ipsec",
		"en0":    "",
		"lo0":    "",
	}
	for name, want := range tests {
		if got := vpnKind(name); got != want {
			t.Fatalf("vpnKind(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestProxyValuesStayRedactedInContext(t *testing.T) {
	config := detectProxyEnv(func(key string) (string, bool) {
		if key == "https_proxy" {
			return "https://user:secret@proxy.example:8443", true
		}
		return "", false
	})
	context, err := detectNetworkContext(config, func() ([]Interface, error) {
		return []Interface{{Name: "utun0", Up: true}}, nil
	}, contextRoute)
	if err != nil {
		t.Fatalf("detectNetworkContext() error = %v", err)
	}
	if len(context.Proxies) != 1 {
		t.Fatalf("proxies = %+v", context.Proxies)
	}
	if strings.Contains(context.Proxies[0].Value, "secret") {
		t.Fatalf("proxy credential leaked: %q", context.Proxies[0].Value)
	}
}
