package probe

import (
	"strings"
	"testing"

	"github.com/gothchibjo/egress/internal/domain"
)

func TestBaselineChecks(t *testing.T) {
	checks := BaselineChecks(BaselineOptions{})
	if len(checks) != 17 {
		t.Fatalf("checks = %d, want 17", len(checks))
	}

	wantPrefixes := []string{"local.interfaces", "gateway.reach", "dns.canary"}
	for i, want := range wantPrefixes {
		if string(checks[i].ID()) != want {
			t.Fatalf("check %d = %q, want %q", i, checks[i].ID(), want)
		}
	}
	if checks[len(checks)-2].ID() != "mtu.pmtu" {
		t.Fatalf("penultimate check = %q, want mtu.pmtu", checks[len(checks)-2].ID())
	}
	if checks[len(checks)-1].ID() != "http.captive" {
		t.Fatalf("last check = %q, want http.captive", checks[len(checks)-1].ID())
	}

	seen := make(map[domain.CheckID]bool, len(checks))
	for _, check := range checks {
		if seen[check.ID()] {
			t.Fatalf("duplicate check id %q", check.ID())
		}
		seen[check.ID()] = true
	}
}

func TestBaselineChecksAreStableAcrossModes(t *testing.T) {
	plain := BaselineChecks(BaselineOptions{})
	elevated := BaselineChecks(BaselineOptions{Elevated: true})
	if len(plain) != len(elevated) {
		t.Fatalf("elevated checks = %d, want %d", len(elevated), len(plain))
	}
	for i := range plain {
		if plain[i].ID() != elevated[i].ID() {
			t.Fatalf("check %d changed id between modes: %q vs %q", i, plain[i].ID(), elevated[i].ID())
		}
	}
}

func TestBaselineChecksUseLayerNamespaces(t *testing.T) {
	prefixes := map[string]bool{
		"local":   true,
		"gateway": true,
		"dns":     true,
		"icmp":    true,
		"tcp":     true,
		"mtu":     true,
		"http":    true,
	}
	for _, check := range BaselineChecks(BaselineOptions{}) {
		prefix := string(check.ID())
		if i := strings.Index(prefix, "."); i >= 0 {
			prefix = prefix[:i]
		}
		if !prefixes[prefix] {
			t.Fatalf("check %q has no path-layer namespace", check.ID())
		}
	}
}
