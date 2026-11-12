package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gothchibjo/egress/internal/platform"
	"github.com/gothchibjo/egress/internal/report"
)

func contextFixture() platform.NetworkContext {
	return platform.NetworkContext{
		Proxies: []platform.ProxySetting{
			{Name: "HTTPS_PROXY", Source: "https_proxy", Value: "http://***@proxy.example:8080"},
		},
		VPN: []platform.VPNInterface{
			{
				Name:         "utun3",
				Kind:         "tun",
				State:        "up",
				Addresses:    []string{"10.8.0.2/32"},
				DefaultRoute: true,
				Source:       "interface name and default route",
			},
			{
				Name:      "wg0",
				Kind:      "wireguard",
				State:     "down",
				Addresses: []string{"10.10.0.2/32"},
				Source:    "interface name",
			},
		},
	}
}

func TestContextPageShowsSources(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithContext(contextFixture(), nil))
	model, _ = pressKey(t, model, "c")

	view := model.View()
	assertBlockShape(t, view, 100, 30)
	for _, want := range []string{
		"CONTEXT",
		"PROXY",
		"HTTPS_PROXY",
		"http://***@proxy.example:8080",
		"environment (https_proxy)",
		"VPN",
		"utun3",
		"tun  up  [default route]",
		"10.8.0.2/32",
		"Sources: proxy variables from the environment",
		"wg0",
		"wireguard  down",
		"Read-only context",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("context page is missing %q:\n%s", want, view)
		}
	}
}

func TestContextPageStatesAbsence(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(),
		WithContext(platform.NetworkContext{
			Proxies: []platform.ProxySetting{},
			VPN:     []platform.VPNInterface{},
		}, nil))
	model, _ = pressKey(t, model, "c")

	view := model.View()
	if got := strings.Count(view, "none detected"); got != 2 {
		t.Fatalf("absence stated %d times, want 2:\n%s", got, view)
	}
}

func TestContextPageWithoutCollectedContext(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, _ = pressKey(t, model, "c")

	if view := model.View(); !strings.Contains(view, "No context was collected") {
		t.Fatalf("missing context is not explained:\n%s", view)
	}
}

func TestContextPageReportsInterfaceError(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(),
		WithContext(contextFixture(), errors.New("interface list unavailable")))
	model, _ = pressKey(t, model, "c")

	view := model.View()
	if !strings.Contains(view, "interface list unavailable") {
		t.Fatal("interface error is hidden")
	}
	if !strings.Contains(view, "HTTPS_PROXY") {
		t.Fatal("proxy context was dropped because the interface read failed")
	}
}

func TestContextPageReturnsToItsOrigin(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithContext(contextFixture(), nil))

	model, _ = pressKey(t, model, "c")
	if !strings.Contains(model.View(), "CONTEXT") {
		t.Fatal("c did not open the context page")
	}
	model, _ = pressKey(t, model, "esc")
	if strings.Contains(model.View(), "CONTEXT") {
		t.Fatal("esc did not leave the context page")
	}

	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "c")
	model, _ = pressKey(t, model, "esc")
	if !strings.Contains(model.View(), "DETAIL") {
		t.Fatal("esc from context did not return to the detail page")
	}
}

func TestContextPageScrollsOnShortTerminals(t *testing.T) {
	context := platform.NetworkContext{Proxies: []platform.ProxySetting{}, VPN: []platform.VPNInterface{}}
	for i := 0; i < 12; i++ {
		context.VPN = append(context.VPN, platform.VPNInterface{
			Name:      fmt.Sprintf("utun%d", i),
			Kind:      "tun",
			State:     "up",
			Addresses: []string{"10.8.0.2/32"},
			Source:    "interface name",
		})
	}

	model := fixtureModel(t, 60, 20, WithoutColor(), WithContext(context, nil))
	model, _ = pressKey(t, model, "c")
	if model.contextMaxOffset() == 0 {
		t.Fatal("test requires more context than the terminal height")
	}
	model, _ = pressKey(t, model, "G")
	if model.contextOffset != model.contextMaxOffset() {
		t.Fatalf("G scrolled to %d, want %d", model.contextOffset, model.contextMaxOffset())
	}
	assertBlockShape(t, model.View(), 60, 20)
}

func TestExportIncludesNetworkContext(t *testing.T) {
	var calls []exportCall
	model := fixtureModel(t, 100, 30, WithoutColor(),
		WithContext(contextFixture(), nil),
		WithExporter(func(path string, envelope report.Envelope, replace bool) error {
			calls = append(calls, exportCall{path: path, envelope: envelope, replace: replace})
			return nil
		}),
	)
	model, _ = pressKey(t, model, "e")

	if len(calls) != 1 {
		t.Fatalf("exporter called %d times", len(calls))
	}
	context := calls[0].envelope.Context
	if context == nil {
		t.Fatal("export has no network context")
	}
	if len(context.Proxies) != 1 || len(context.VPN) != 2 {
		t.Fatalf("exported context = %+v", context)
	}
}

func TestExportWithoutContextOmitsIt(t *testing.T) {
	var calls []exportCall
	model := fixtureModel(t, 100, 30, WithoutColor(),
		WithExporter(func(path string, envelope report.Envelope, replace bool) error {
			calls = append(calls, exportCall{path: path, envelope: envelope, replace: replace})
			return nil
		}),
	)
	model, _ = pressKey(t, model, "e")

	if len(calls) != 1 {
		t.Fatalf("exporter called %d times", len(calls))
	}
	if calls[0].envelope.Context != nil {
		t.Fatalf("export invented a context: %+v", calls[0].envelope.Context)
	}
}

func TestContextPageStatesTheSourceRuleOnceAtTheTop(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithContext(contextFixture(), nil))
	model, _ = pressKey(t, model, "c")

	view := model.View()
	if got := strings.Count(view, "Sources:"); got != 1 {
		t.Fatalf("source rule appears %d times, want once:\n%s", got, view)
	}
	if normalized := strings.Join(strings.Fields(view), " "); !strings.Contains(normalized, "with the default route marked when it applies") {
		t.Fatal("the source rule does not explain the default-route marker")
	}
	if got := strings.Count(view, "     source  "); got != 1 {
		t.Fatalf("per-entry source lines = %d, want only the proxy source:\n%s", got, view)
	}

	lines := strings.Split(view, "\n")
	vpn := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "VPN" {
			vpn = i
			break
		}
	}
	if vpn < 0 || vpn+1 >= len(lines) {
		t.Fatalf("VPN section not found:\n%s", view)
	}
	if got := strings.TrimSpace(lines[vpn+1]); strings.HasPrefix(got, "source") {
		t.Fatalf("VPN section still repeats the source rule: %q", got)
	}
}
