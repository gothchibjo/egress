package domain

import "strings"

// FailureScope locates the first failing path layer of a baseline run.
type FailureScope string

const (
	ScopeNone      FailureScope = "none"
	ScopeLocalLink FailureScope = "local-link"
	ScopeGateway   FailureScope = "gateway"
	ScopeDNS       FailureScope = "dns"
	ScopeUpstream  FailureScope = "upstream"
)

// Classification explains where a run stopped working and what was still
// testable locally.
type Classification struct {
	Scope   FailureScope `json:"scope"`
	Summary string       `json:"summary"`
}

// ClassifyRun derives the failure scope from the checks already collected.
// It names the first failing layer in path order and avoids claims that the
// evidence cannot support.
func ClassifyRun(results []Result) (Classification, error) {
	if len(results) == 0 {
		return Classification{}, ErrNoResults
	}

	var localFailed, gatewayFailed, dnsFailed, upstreamFailed bool
	for _, result := range results {
		layer := checkLayer(result.CheckID)
		switch layer {
		case "local":
			localFailed = localFailed || result.Status == StatusFail
		case "gateway":
			gatewayFailed = gatewayFailed || result.Status == StatusFail
		case "dns":
			dnsFailed = dnsFailed || result.Status == StatusFail || result.Status == StatusWarn
		case "upstream":
			upstreamFailed = upstreamFailed || result.Status == StatusFail
		}
	}

	switch {
	case localFailed:
		return Classification{
			Scope:   ScopeLocalLink,
			Summary: "Local link problem: interfaces or the default route failed. Local inspection ran; gateway, DNS and upstream were not reached reliably.",
		}, nil
	case gatewayFailed:
		return Classification{
			Scope:   ScopeGateway,
			Summary: "Local link was usable but the default gateway did not answer. Local interfaces and routes were inspected.",
		}, nil
	case dnsFailed:
		return Classification{
			Scope:   ScopeDNS,
			Summary: "Local link and gateway were usable but name resolution failed. Local inspection and gateway probing ran.",
		}, nil
	case upstreamFailed:
		return Classification{
			Scope:   ScopeUpstream,
			Summary: "Local link, gateway and name resolution were usable; remote probes failed.",
		}, nil
	default:
		return Classification{
			Scope:   ScopeNone,
			Summary: "Local link, gateway, DNS and upstream checks completed without failures.",
		}, nil
	}
}

// checkLayer maps a check id namespace to a path layer. Unknown ids are
// ignored so custom checks cannot change the scope.
func checkLayer(id CheckID) string {
	name := strings.ToLower(string(id))
	if i := strings.Index(name, "."); i >= 0 {
		name = name[:i]
	}
	switch name {
	case "local", "link", "route", "interface":
		return "local"
	case "gateway":
		return "gateway"
	case "dns":
		return "dns"
	case "icmp", "tcp", "mtu", "latency", "http", "captive":
		return "upstream"
	default:
		return ""
	}
}
