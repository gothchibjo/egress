package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/gothchibjo/egress/internal/domain"
)

func destinationFixture() []domain.Destination {
	return []domain.Destination{
		{
			Kind:      domain.DestinationResolver,
			Host:      "192.0.2.53",
			Port:      53,
			Transport: domain.TransportUDPAndTCP,
			Purpose:   "configured resolver for the canary query",
		},
		{
			Kind:      domain.DestinationDoH,
			Host:      "cloudflare-dns.com",
			Port:      443,
			Transport: domain.TransportHTTPS,
			Purpose:   "public DoH comparison for the canary",
		},
		{
			Kind:      domain.DestinationAnchor,
			Host:      "1.1.1.1",
			Port:      443,
			Transport: domain.TransportTCP,
			Purpose:   "TCP reachability to the anchor",
		},
		{
			Kind:      domain.DestinationAnchor,
			Host:      "example.com",
			Transport: domain.TransportICMP,
			Purpose:   "ICMP reachability and latency sample",
		},
		{
			Kind:      domain.DestinationPathMTU,
			Host:      "example.com",
			Transport: domain.TransportICMPDF,
			Purpose:   "path MTU probe with the don't-fragment flag",
		},
		{
			Kind:      domain.DestinationCaptive,
			Host:      "captive.apple.com",
			Port:      80,
			Transport: domain.TransportHTTP,
			Purpose:   "captive-portal canary request",
		},
	}
}

func TestDestinationsPageShowsDocumentedEndpoints(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithDestinations(destinationFixture(), nil))
	model, _ = pressKey(t, model, "d")

	view := model.View()
	assertBlockShape(t, view, 100, 30)
	for _, want := range []string{
		"DESTINATIONS",
		"KIND",
		"TRANSPORT",
		"ENDPOINT",
		"PURPOSE",
		"resolver",
		"doh",
		"anchor",
		"path-mtu",
		"captive",
		"192.0.2.53:53",
		"cloudflare-dns.com:443",
		"1.1.1.1:443",
		"captive.apple.com:80",
		"No telemetry",
		"6 endpoints",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("destination page is missing %q", want)
		}
	}
}

func TestDestinationsPageReturnsToItsOrigin(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithDestinations(destinationFixture(), nil))

	model, _ = pressKey(t, model, "d")
	if !strings.Contains(model.View(), "DESTINATIONS") {
		t.Fatal("d did not open the destination page")
	}
	model, _ = pressKey(t, model, "esc")
	if strings.Contains(model.View(), "DESTINATIONS") {
		t.Fatal("esc did not leave the destination page")
	}

	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "d")
	model, _ = pressKey(t, model, "esc")
	if !strings.Contains(model.View(), "DETAIL") {
		t.Fatal("esc from destinations did not return to the detail page")
	}
}

func TestDestinationsPageHelpReturns(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithDestinations(destinationFixture(), nil))
	model, _ = pressKey(t, model, "d")
	model, _ = pressKey(t, model, "?")

	if view := model.View(); !strings.Contains(view, "HELP") || !strings.Contains(view, "destinations") {
		t.Fatalf("help does not document the destination page:\n%s", view)
	}
	model, _ = pressKey(t, model, "esc")
	if !strings.Contains(model.View(), "DESTINATIONS") {
		t.Fatal("help did not return to the destination page")
	}
}

func TestDestinationsPageWithoutResolverList(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(),
		WithDestinations(nil, errors.New("resolver configuration unavailable")))
	model, _ = pressKey(t, model, "d")

	view := model.View()
	if !strings.Contains(view, "Resolver list unavailable") {
		t.Fatal("destination page hides the resolver read error")
	}
	if !strings.Contains(view, "No documented destinations") {
		t.Fatal("destination page does not report an empty list")
	}
}

func TestDestinationsPageASCIIAndCompact(t *testing.T) {
	model := fixtureModel(t, 60, 20, WithoutColor(), WithASCII(), WithDestinations(destinationFixture(), nil))
	model, _ = pressKey(t, model, "d")

	view := model.View()
	assertBlockShape(t, view, 60, 20)
	if strings.Contains(view, "─") {
		t.Fatal("ASCII destination page renders a Unicode rule")
	}
	if !strings.Contains(view, "DESTINATIONS") {
		t.Fatal("compact destination page lost its header")
	}
}

func TestDestinationsPageScrollsOnShortTerminals(t *testing.T) {
	many := destinationFixture()
	for i := 0; i < 2; i++ {
		many = append(many, destinationFixture()...)
	}

	model := fixtureModel(t, 60, 20, WithoutColor(), WithDestinations(many, nil))
	model, _ = pressKey(t, model, "d")
	if model.destinationMaxOffset() == 0 {
		t.Fatal("test requires a list longer than the terminal")
	}
	if model.destinationOffset != 0 {
		t.Fatalf("page opened at offset %d", model.destinationOffset)
	}

	model, _ = pressKey(t, model, "G")
	if model.destinationOffset != model.destinationMaxOffset() {
		t.Fatalf("G moved to %d, want %d", model.destinationOffset, model.destinationMaxOffset())
	}
	assertBlockShape(t, model.View(), 60, 20)

	model, _ = pressKey(t, model, "g")
	if model.destinationOffset != 0 {
		t.Fatalf("g moved to %d, want 0", model.destinationOffset)
	}

	model, _ = pressKey(t, model, "k")
	if model.destinationOffset != 0 {
		t.Fatalf("k scrolled past the top: %d", model.destinationOffset)
	}
}

func TestDestinationsPageDropsPurposeWhenItDoesNotFit(t *testing.T) {
	model := fixtureModel(t, 80, 24, WithoutColor(), WithDestinations(destinationFixture(), nil))
	model, _ = pressKey(t, model, "d")

	view := model.View()
	if strings.Contains(view, "PURPOSE") {
		t.Fatal("narrow layout kept a clipped purpose column")
	}
	for _, want := range []string{"KIND", "TRANSPORT", "ENDPOINT", "cloudflare-dns.com:443"} {
		if !strings.Contains(view, want) {
			t.Fatalf("narrow destination table is missing %q", want)
		}
	}
}

func TestDestinationTableColumnsAlign(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithDestinations(destinationFixture(), nil))
	model, _ = pressKey(t, model, "d")
	lines := strings.Split(model.View(), "\n")

	var header, row string
	for _, line := range lines {
		if strings.Contains(line, "KIND") && strings.Contains(line, "TRANSPORT") {
			header = line
		}
		if strings.Contains(line, "cloudflare-dns.com:443") {
			row = line
		}
	}
	if header == "" || row == "" {
		t.Fatalf("destination table not found:\n%s", model.View())
	}

	tests := []struct {
		column string
		value  string
	}{
		{column: "TRANSPORT", value: "https"},
		{column: "ENDPOINT", value: "cloudflare-dns.com:443"},
		{column: "PURPOSE", value: "public DoH comparison"},
	}
	for _, test := range tests {
		headerColumn := strings.Index(header, test.column)
		dataColumn := strings.Index(row, test.value)
		if headerColumn != dataColumn {
			t.Fatalf("%s starts at %d in the header and %d in the row:\n%s\n%s",
				test.column, headerColumn, dataColumn, header, row)
		}
	}
}
