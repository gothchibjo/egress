package probe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

var fixedTime = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

type fakeLinkSource struct {
	interfaces func() ([]platform.Interface, error)
	route      func(platform.Family) (platform.DefaultRoute, bool, error)
}

func (f fakeLinkSource) Interfaces() ([]platform.Interface, error) {
	if f.interfaces == nil {
		return nil, nil
	}
	return f.interfaces()
}

func (f fakeLinkSource) DefaultRoute(family platform.Family) (platform.DefaultRoute, bool, error) {
	if f.route == nil {
		return platform.DefaultRoute{}, false, nil
	}
	return f.route(family)
}

func interfacesOK() func() ([]platform.Interface, error) {
	return func() ([]platform.Interface, error) {
		return []platform.Interface{
			{Name: "lo0", Index: 1, Up: true, Loopback: true, Addresses: []string{"127.0.0.1/8"}},
			{Name: "en0", Index: 2, Up: true, MTU: 1500, Addresses: []string{"192.168.1.5/24", "fe80::1/64"}},
		}, nil
	}
}

func routesOK() func(platform.Family) (platform.DefaultRoute, bool, error) {
	return func(family platform.Family) (platform.DefaultRoute, bool, error) {
		if family == platform.FamilyIPv4 {
			return platform.DefaultRoute{Family: family, Gateway: "192.168.1.1", Interface: "en0"}, true, nil
		}
		return platform.DefaultRoute{}, false, nil
	}
}

func findMeasurement(result domain.Result, name string) (string, bool) {
	for _, measurement := range result.Measurements {
		if measurement.Name == name {
			return measurement.Value, true
		}
	}
	return "", false
}

func TestLocalCheckPass(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{interfaces: interfacesOK(), route: routesOK()}, func() time.Time { return fixedTime })
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
	if result.CheckID != "local.interfaces" || result.Target != "local host" {
		t.Fatalf("unexpected identity: %+v", result)
	}
	if !result.Timestamp.Equal(fixedTime) {
		t.Fatalf("timestamp = %v, want %v", result.Timestamp, fixedTime)
	}

	want := map[string]string{
		"interface.lo0.state":     "up",
		"interface.en0.state":     "up",
		"interface.en0.addresses": "192.168.1.5/24, fe80::1/64",
		"default_route.ipv4":      "192.168.1.1 via en0",
		"default_route.ipv6":      "none",
	}
	for name, value := range want {
		got, ok := findMeasurement(result, name)
		if !ok {
			t.Fatalf("measurement %q is missing", name)
		}
		if got != value {
			t.Fatalf("measurement %q = %q, want %q", name, got, value)
		}
	}
}

func TestLocalCheckMissingDefaultRoute(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{
		interfaces: interfacesOK(),
		route: func(platform.Family) (platform.DefaultRoute, bool, error) {
			return platform.DefaultRoute{}, false, nil
		},
	}, nil)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "no default route") {
		t.Fatalf("diagnostic = %q, want a missing-route reason", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestLocalCheckNoActiveInterface(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{
		interfaces: func() ([]platform.Interface, error) {
			return []platform.Interface{
				{Name: "lo0", Index: 1, Up: true, Loopback: true, Addresses: []string{"127.0.0.1/8"}},
				{Name: "en0", Index: 2, Up: false, Addresses: []string{"192.168.1.5/24"}},
			}, nil
		},
		route: routesOK(),
	}, nil)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "no active non-loopback interface") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestLocalCheckInterfaceError(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{
		interfaces: func() ([]platform.Interface, error) {
			return nil, errors.New("permission denied")
		},
		route: routesOK(),
	}, nil)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Error, "permission denied") {
		t.Fatalf("error = %q", result.Error)
	}
	if !strings.Contains(result.Diagnostic, "interfaces unavailable") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestLocalCheckRouteError(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{
		interfaces: interfacesOK(),
		route: func(family platform.Family) (platform.DefaultRoute, bool, error) {
			if family == platform.FamilyIPv4 {
				return platform.DefaultRoute{}, false, errors.New("route table unreadable")
			}
			return platform.DefaultRoute{}, false, nil
		},
	}, nil)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Error, "route table unreadable") {
		t.Fatalf("error = %q", result.Error)
	}
	value, ok := findMeasurement(result, "default_route.ipv4")
	if !ok || value != "unavailable" {
		t.Fatalf("default_route.ipv4 = %q, %v", value, ok)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestLocalCheckOnLinkRoute(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{
		interfaces: interfacesOK(),
		route: func(family platform.Family) (platform.DefaultRoute, bool, error) {
			if family == platform.FamilyIPv4 {
				return platform.DefaultRoute{Family: family, Interface: "en0"}, true, nil
			}
			return platform.DefaultRoute{}, false, nil
		},
	}, nil)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	value, _ := findMeasurement(result, "default_route.ipv4")
	if value != "on-link via en0" {
		t.Fatalf("default_route.ipv4 = %q", value)
	}
}

func TestLocalCheckIPv6DefaultRoute(t *testing.T) {
	check := NewLocalCheckWithSource(fakeLinkSource{
		interfaces: interfacesOK(),
		route: func(family platform.Family) (platform.DefaultRoute, bool, error) {
			if family == platform.FamilyIPv6 {
				return platform.DefaultRoute{Family: family, Gateway: "fe80::1", Interface: "en0"}, true, nil
			}
			return platform.DefaultRoute{}, false, nil
		},
	}, nil)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass", result.Status)
	}
	value, _ := findMeasurement(result, "default_route.ipv6")
	if value != "fe80::1 via en0" {
		t.Fatalf("default_route.ipv6 = %q", value)
	}
}

func TestLocalCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	check := NewLocalCheckWithSource(fakeLinkSource{}, nil)
	result := check.Run(ctx)
	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if result.Diagnostic == "" {
		t.Fatal("skipped result has no reason")
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestLocalCheckPlatform(t *testing.T) {
	check := NewLocalCheck()
	result := check.Run(context.Background())
	if err := result.Validate(); err != nil {
		t.Fatalf("platform result is invalid: %v", err)
	}
	if result.Status != domain.StatusPass && result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want pass or fail", result.Status)
	}
	if len(result.Measurements) == 0 {
		t.Fatal("platform run produced no observations")
	}
}
