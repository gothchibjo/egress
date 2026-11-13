package domain

import (
	"strings"
	"testing"
)

func TestPresetCatalogContents(t *testing.T) {
	presets := PresetCatalog()
	if len(presets) != 11 {
		t.Fatalf("catalog has %d entries, want 11", len(presets))
	}

	wantIDs := []string{
		"youtube", "telegram", "discord", "github", "google", "cloudflare",
		"wikipedia", "x", "tor", "cloudflare-dns", "google-dns",
	}
	for i, want := range wantIDs {
		if presets[i].ID != want {
			t.Fatalf("entry %d has id %q, want %q", i, presets[i].ID, want)
		}
	}

	seen := make(map[string]bool, len(presets))
	for _, preset := range presets {
		if preset.DisplayName == "" || preset.Host == "" || preset.Category == "" {
			t.Fatalf("preset %q has an empty field: %+v", preset.ID, preset)
		}
		if strings.Contains(preset.Host, "://") || strings.Contains(preset.Host, "/") {
			t.Fatalf("preset %q exposes a URL instead of an address: %q", preset.ID, preset.Host)
		}
		if seen[preset.ID] {
			t.Fatalf("duplicate preset id %q", preset.ID)
		}
		seen[preset.ID] = true

		if len(preset.Ports) == 0 {
			t.Fatalf("preset %q has no default ports", preset.ID)
		}
		for _, port := range preset.Ports {
			if port < 1 || port > 65535 {
				t.Fatalf("preset %q has port %d out of range", preset.ID, port)
			}
		}
		if len(preset.Protocols) == 0 {
			t.Fatalf("preset %q has no default protocols", preset.ID)
		}
		for _, protocol := range preset.Protocols {
			switch protocol {
			case PresetICMP, PresetTCP, PresetTLS, PresetHTTP:
			default:
				t.Fatalf("preset %q has unknown protocol %q", preset.ID, protocol)
			}
		}
	}
}

func TestPresetCatalogIsACopy(t *testing.T) {
	first := PresetCatalog()
	first[0].DisplayName = "mutated"
	first[0].Ports[0] = 1
	first[0].Protocols[0] = "mutated"

	second := PresetCatalog()
	if second[0].DisplayName != "YouTube" {
		t.Fatalf("catalog was mutated through the returned slice: %q", second[0].DisplayName)
	}
	if second[0].Ports[0] != 443 {
		t.Fatalf("ports were mutated through the returned slice: %v", second[0].Ports)
	}
	if second[0].Protocols[0] != PresetICMP {
		t.Fatalf("protocols were mutated through the returned slice: %v", second[0].Protocols)
	}
}

func TestPresetCatalogIsDeterministic(t *testing.T) {
	first := PresetCatalog()
	second := PresetCatalog()
	if len(first) != len(second) {
		t.Fatalf("catalog lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].Host != second[i].Host {
			t.Fatalf("entry %d differs between calls", i)
		}
	}
}

func TestCatalogVersion(t *testing.T) {
	if CatalogVersion == "" {
		t.Fatal("catalog version is empty")
	}
}
