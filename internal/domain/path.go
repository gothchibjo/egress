package domain

import "fmt"

// LocalizationKind names the path finding.
type LocalizationKind string

const (
	// PathComplete means the path reached the target.
	PathComplete LocalizationKind = "complete"
	// PathDrop means the replies stop after the last responding hop.
	PathDrop LocalizationKind = "drop"
	// PathInconclusive means the evidence cannot separate a break from silence.
	PathInconclusive LocalizationKind = "inconclusive"
	// PathUnknown means there is no path data at all.
	PathUnknown LocalizationKind = "unknown"
)

// Localization is the path finding with its caveats.
type Localization struct {
	Kind         LocalizationKind `json:"kind"`
	AfterTTL     int              `json:"after_ttl,omitempty"`
	FromTTL      int              `json:"from_ttl,omitempty"`
	ToTTL        int              `json:"to_ttl,omitempty"`
	Summary      string           `json:"summary"`
	Alternatives []string         `json:"alternatives,omitempty"`
}

// PathOptions carries the evidence that cross-checks the hop table.
type PathOptions struct {
	// TargetIP is the address the trace aimed at.
	TargetIP string
	// Truncated reports that the trace ended before the target or the hop limit.
	Truncated bool
	// LadderReachable reports that a ladder step reached the target.
	LadderReachable bool
}

// PathReport is the hop table with its localization.
type PathReport struct {
	Hops         []Hop        `json:"hops"`
	Localization Localization `json:"localization"`
}

// BuildPathReport pairs the hop table with its localization.
func BuildPathReport(hops []Hop, opts PathOptions) PathReport {
	return PathReport{Hops: append([]Hop(nil), hops...), Localization: LocalizePath(hops, opts)}
}

// LocalizePath derives the path finding from the hop table. A silent hop is
// never presented as proof of blocking: every finding carries the alternative
// explanations.
func LocalizePath(hops []Hop, opts PathOptions) Localization {
	if len(hops) == 0 {
		return Localization{
			Kind:    PathUnknown,
			Summary: "no path data",
			Alternatives: []string{
				"the trace could not start",
				"single vantage point",
			},
		}
	}

	maxTTL := hops[len(hops)-1].TTL
	lastTTL := 0
	targetTTL := 0
	for _, hop := range hops {
		if hop.Address != "" {
			lastTTL = hop.TTL
		}
		for _, address := range hop.AllAddresses() {
			if opts.TargetIP != "" && address == opts.TargetIP {
				targetTTL = hop.TTL
			}
		}
	}

	if lastTTL == 0 {
		return Localization{
			Kind:    PathInconclusive,
			FromTTL: hops[0].TTL,
			ToTTL:   maxTTL,
			Summary: fmt.Sprintf("inconclusive between hops %d and %d: no hop answered", hops[0].TTL, maxTTL),
			Alternatives: []string{
				"routers may not answer this probe type",
				"the target may answer other protocols only",
				"single vantage point",
			},
		}
	}

	if targetTTL > 0 {
		return Localization{
			Kind:     PathComplete,
			AfterTTL: targetTTL,
			Summary:  fmt.Sprintf("path complete at hop %d", targetTTL),
			Alternatives: []string{
				"the path may differ for other protocols or ports",
				"single vantage point",
			},
		}
	}

	fromTTL := lastTTL + 1
	if opts.Truncated || opts.LadderReachable {
		reason := "path discovery was truncated before the target"
		alternatives := []string{
			"the trace budget may be too short for this path",
			"ICMP may be rate-limited",
			"single vantage point",
		}
		if opts.LadderReachable {
			reason = "the target answered a transport step, but these hops returned no probe reply"
			alternatives = []string{
				"a router may rate-limit or block probe replies",
				"the path may differ between TCP and ICMP",
				"single vantage point",
			}
		}
		return Localization{
			Kind:         PathInconclusive,
			FromTTL:      fromTTL,
			ToTTL:        maxTTL,
			Summary:      fmt.Sprintf("inconclusive between hops %d and %d: %s", fromTTL, maxTTL, reason),
			Alternatives: alternatives,
		}
	}

	return Localization{
		Kind:     PathDrop,
		AfterTTL: lastTTL,
		ToTTL:    maxTTL,
		Summary:  fmt.Sprintf("drop after hop %d: no reply from the hops that follow", lastTTL),
		Alternatives: []string{
			"a silent router may rate-limit or block ICMP",
			"the target may not answer this probe",
			"the path may change over time",
			"single vantage point",
		},
	}
}
