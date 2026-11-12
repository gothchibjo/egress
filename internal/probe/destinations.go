package probe

import (
	"net/url"
	"sort"
	"strconv"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// OutboundDestinations returns every documented endpoint the baseline may
// contact. The static part is derived from the same constants the probes use,
// so the documented list cannot drift from the traffic; the configured
// nameservers are read from the platform and added at runtime.
func OutboundDestinations() ([]domain.Destination, error) {
	destinations := staticDestinations()

	resolvers, err := platform.System().Resolvers()
	if err == nil {
		for _, resolver := range resolvers {
			destinations = append(destinations, domain.Destination{
				Kind:      domain.DestinationResolver,
				Host:      resolver.Address,
				Port:      53,
				Transport: domain.TransportUDPAndTCP,
				Purpose:   "configured resolver for the canary query",
			})
		}
	}
	return normalizeDestinations(destinations), err
}

// staticDestinations lists the endpoints that do not depend on host state.
func staticDestinations() []domain.Destination {
	var destinations []domain.Destination

	if endpoint, err := url.Parse(DefaultDoHEndpoint); err == nil {
		destinations = append(destinations, domain.Destination{
			Kind:      domain.DestinationDoH,
			Host:      endpoint.Hostname(),
			Port:      urlPort(endpoint, 443),
			Transport: domain.TransportHTTPS,
			Purpose:   "public DoH comparison for the canary",
		})
	}

	options := TCPOptions{}.normalized()
	for _, anchor := range domain.BaselineAnchors() {
		destinations = append(destinations, domain.Destination{
			Kind:      domain.DestinationAnchor,
			Host:      anchor.Host,
			Transport: domain.TransportICMP,
			Purpose:   "ICMP reachability and latency sample",
		})
		for _, port := range options.Ports {
			destinations = append(destinations, domain.Destination{
				Kind:      domain.DestinationAnchor,
				Host:      anchor.Host,
				Port:      port,
				Transport: domain.TransportTCP,
				Purpose:   "TCP reachability to the anchor",
			})
		}
	}

	destinations = append(destinations, domain.Destination{
		Kind:      domain.DestinationPathMTU,
		Host:      DefaultMTUHost,
		Transport: domain.TransportICMPDF,
		Purpose:   "path MTU probe with the don't-fragment flag",
	})

	if canary, err := url.Parse(DefaultCaptiveURL); err == nil {
		destinations = append(destinations, domain.Destination{
			Kind:      domain.DestinationCaptive,
			Host:      canary.Hostname(),
			Port:      urlPort(canary, 80),
			Transport: domain.TransportHTTP,
			Purpose:   "captive-portal canary request",
		})
	}

	return destinations
}

// urlPort returns the explicit port or the scheme default.
func urlPort(endpoint *url.URL, fallback int) int {
	if raw := endpoint.Port(); raw != "" {
		if port, err := strconv.Atoi(raw); err == nil {
			return port
		}
	}
	return fallback
}

// normalizeDestinations removes exact duplicates and orders the list by kind,
// host, port and transport so the TUI and JSON render a stable document.
func normalizeDestinations(destinations []domain.Destination) []domain.Destination {
	seen := make(map[string]bool, len(destinations))
	out := make([]domain.Destination, 0, len(destinations))
	for _, destination := range destinations {
		key := string(destination.Kind) + "\x00" + destination.Host + "\x00" +
			strconv.Itoa(destination.Port) + "\x00" + string(destination.Transport)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, destination)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if kindOrder(out[i].Kind) != kindOrder(out[j].Kind) {
			return kindOrder(out[i].Kind) < kindOrder(out[j].Kind)
		}
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Transport < out[j].Transport
	})
	return out
}

// kindOrder is the render and review order: resolvers, DoH, anchors, path MTU,
// captive portal.
func kindOrder(kind domain.DestinationKind) int {
	switch kind {
	case domain.DestinationResolver:
		return 0
	case domain.DestinationDoH:
		return 1
	case domain.DestinationAnchor:
		return 2
	case domain.DestinationPathMTU:
		return 3
	case domain.DestinationCaptive:
		return 4
	default:
		return 5
	}
}
