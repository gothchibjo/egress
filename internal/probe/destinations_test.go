package probe

import (
	"net/url"
	"testing"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

func hasDestination(destinations []domain.Destination, kind domain.DestinationKind, host string, port int, transport domain.Transport) bool {
	for _, destination := range destinations {
		if destination.Kind == kind && destination.Host == host && destination.Port == port && destination.Transport == transport {
			return true
		}
	}
	return false
}

func TestStaticDestinationsCoverEveryProbe(t *testing.T) {
	destinations := staticDestinations()
	options := TCPOptions{}.normalized()

	for _, anchor := range domain.BaselineAnchors() {
		if !hasDestination(destinations, domain.DestinationAnchor, anchor.Host, 0, domain.TransportICMP) {
			t.Fatalf("anchor %q has no ICMP destination", anchor.ID)
		}
		for _, port := range options.Ports {
			if !hasDestination(destinations, domain.DestinationAnchor, anchor.Host, port, domain.TransportTCP) {
				t.Fatalf("anchor %q has no TCP/%d destination", anchor.ID, port)
			}
		}
	}

	doh, err := url.Parse(DefaultDoHEndpoint)
	if err != nil {
		t.Fatalf("parse DoH endpoint: %v", err)
	}
	if !hasDestination(destinations, domain.DestinationDoH, doh.Hostname(), 443, domain.TransportHTTPS) {
		t.Fatal("DoH comparison endpoint is missing")
	}

	if !hasDestination(destinations, domain.DestinationPathMTU, DefaultMTUHost, 0, domain.TransportICMPDF) {
		t.Fatal("path MTU destination is missing")
	}

	canary, err := url.Parse(DefaultCaptiveURL)
	if err != nil {
		t.Fatalf("parse captive URL: %v", err)
	}
	if !hasDestination(destinations, domain.DestinationCaptive, canary.Hostname(), 80, domain.TransportHTTP) {
		t.Fatal("captive-portal destination is missing")
	}
}

func TestStaticDestinationsAreClosed(t *testing.T) {
	allowed := map[string]bool{
		DefaultMTUHost: true,
	}
	if canary, err := url.Parse(DefaultCaptiveURL); err == nil {
		allowed[canary.Hostname()] = true
	}
	if doh, err := url.Parse(DefaultDoHEndpoint); err == nil {
		allowed[doh.Hostname()] = true
	}
	for _, anchor := range domain.BaselineAnchors() {
		allowed[anchor.Host] = true
	}

	for _, destination := range staticDestinations() {
		if !allowed[destination.Host] {
			t.Fatalf("undocumented destination %q (%s)", destination.Host, destination.Kind)
		}
		if destination.Kind == domain.DestinationResolver {
			t.Fatalf("static catalog contains a resolver entry: %q", destination.Host)
		}
		if err := destination.Validate(); err != nil {
			t.Fatalf("static destination is invalid: %v", err)
		}
	}
}

func TestOutboundDestinationsAddConfiguredResolvers(t *testing.T) {
	resolvers, err := platform.System().Resolvers()
	if err != nil {
		t.Skipf("resolver configuration unavailable: %v", err)
	}

	destinations, err := OutboundDestinations()
	if err != nil {
		t.Fatalf("OutboundDestinations() error = %v", err)
	}
	for _, resolver := range resolvers {
		if !hasDestination(destinations, domain.DestinationResolver, resolver.Address, 53, domain.TransportUDPAndTCP) {
			t.Fatalf("resolver %q is missing from the documented destinations", resolver.Address)
		}
	}
	for _, destination := range destinations {
		if err := destination.Validate(); err != nil {
			t.Fatalf("destination is invalid: %v", err)
		}
	}
}

func TestOutboundDestinationsAreStable(t *testing.T) {
	first, firstErr := OutboundDestinations()
	second, secondErr := OutboundDestinations()
	if (firstErr == nil) != (secondErr == nil) {
		t.Fatalf("calls disagree on the resolver error: %v vs %v", firstErr, secondErr)
	}
	if len(first) != len(second) {
		t.Fatalf("calls returned %d and %d destinations", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("destination %d differs between calls: %+v vs %+v", i, first[i], second[i])
		}
	}
}
