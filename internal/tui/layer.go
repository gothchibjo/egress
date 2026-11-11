// Package tui renders the interactive dashboard. It depends on the domain
// model and never changes it.
package tui

import (
	"strings"

	"github.com/gothchibjo/egress/internal/domain"
)

// Layer is one path layer of the baseline, ordered from the local host
// outward.
type Layer string

const (
	LayerLocal     Layer = "local link"
	LayerGateway   Layer = "gateway"
	LayerDNS       Layer = "dns"
	LayerTransport Layer = "transport"
	LayerHTTP      Layer = "http"
	LayerOther     Layer = "other"
)

var layerOrder = []Layer{
	LayerLocal,
	LayerGateway,
	LayerDNS,
	LayerTransport,
	LayerHTTP,
	LayerOther,
}

var layerPrefixes = map[Layer][]string{
	LayerLocal:     {"local", "link", "route", "interface"},
	LayerGateway:   {"gateway"},
	LayerDNS:       {"dns"},
	LayerTransport: {"icmp", "tcp", "mtu", "latency", "transport"},
	LayerHTTP:      {"http", "captive"},
}

// LayerFor classifies a check id by its namespace prefix. A check is named
// "<layer>.<name>", for example "dns.canary" or "tcp.anchor"; an unknown
// prefix falls back to LayerOther. Classification is presentation-only and
// never changes a result.
func LayerFor(id domain.CheckID) Layer {
	name := strings.ToLower(string(id))
	if i := strings.Index(name, "."); i >= 0 {
		name = name[:i]
	}
	for _, layer := range layerOrder {
		for _, prefix := range layerPrefixes[layer] {
			if name == prefix {
				return layer
			}
		}
	}
	return LayerOther
}

// row is one result together with the layer it is displayed under.
type row struct {
	layer  Layer
	result domain.Result
}

// rows groups results by layer in path order. Within a layer the baseline
// order is kept, so the row index equals the result index in the aggregate.
func rows(results []domain.Result) []row {
	grouped := make([]row, 0, len(results))
	for _, layer := range layerOrder {
		for _, result := range results {
			if LayerFor(result.CheckID) == layer {
				grouped = append(grouped, row{layer: layer, result: result})
			}
		}
	}
	return grouped
}
