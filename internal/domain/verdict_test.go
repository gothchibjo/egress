package domain

import (
	"errors"
	"testing"
	"time"
)

func verdictResult(id CheckID, target string, status Status) Result {
	result := Result{
		CheckID:   id,
		Status:    status,
		Target:    target,
		Method:    "test",
		Timestamp: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	switch status {
	case StatusPass:
		result.Measurements = []Measurement{{Name: "reachable", Value: "true"}}
	case StatusWarn, StatusFail:
		result.Measurements = []Measurement{{Name: "reachable", Value: "false"}}
	case StatusSkipped:
		result.Diagnostic = "capability unavailable in this mode"
	}
	return result
}

func TestAggregateVerdict(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		level   VerdictLevel
		counts  Counts
		finding CheckID
	}{
		{
			name:    "all pass is healthy",
			results: []Result{verdictResult("gateway", "192.168.1.1", StatusPass), verdictResult("dns", "example.com", StatusPass)},
			level:   VerdictHealthy,
			counts:  Counts{Pass: 2},
		},
		{
			name:    "a warning degrades",
			results: []Result{verdictResult("gateway", "192.168.1.1", StatusPass), verdictResult("http", "example.com", StatusWarn)},
			level:   VerdictDegraded,
			counts:  Counts{Pass: 1, Warn: 1},
			finding: "http",
		},
		{
			name:    "a core failure blocks",
			results: []Result{verdictResult("gateway", "192.168.1.1", StatusFail), verdictResult("dns", "example.com", StatusPass)},
			level:   VerdictBlocked,
			counts:  Counts{Pass: 1, Fail: 1},
			finding: "gateway",
		},
		{
			name:    "skipped does not hide a pass",
			results: []Result{verdictResult("icmp", "1.1.1.1", StatusSkipped), verdictResult("tcp", "8.8.8.8", StatusPass)},
			level:   VerdictHealthy,
			counts:  Counts{Pass: 1, Skipped: 1},
		},
		{
			name:    "all skipped cannot establish health",
			results: []Result{verdictResult("icmp", "1.1.1.1", StatusSkipped), verdictResult("tcp", "8.8.8.8", StatusSkipped)},
			level:   VerdictDegraded,
			counts:  Counts{Skipped: 2},
			finding: "icmp",
		},
		{
			name: "one failing anchor while another passes is degraded",
			results: []Result{
				verdictResult("icmp-anchor", "1.1.1.1", StatusFail),
				verdictResult("tcp-anchor", "8.8.8.8", StatusPass),
			},
			level:   VerdictDegraded,
			counts:  Counts{Pass: 1, Fail: 1},
			finding: "icmp-anchor",
		},
		{
			name:    "a single failing anchor alone is degraded",
			results: []Result{verdictResult("icmp-anchor", "1.1.1.1", StatusFail)},
			level:   VerdictDegraded,
			counts:  Counts{Fail: 1},
			finding: "icmp-anchor",
		},
		{
			name: "every anchor failing blocks",
			results: []Result{
				verdictResult("icmp-cloudflare", "1.1.1.1", StatusFail),
				verdictResult("tcp-google", "8.8.8.8", StatusFail),
				verdictResult("http-apple", "https://captive.apple.com/hotspot-detect.html", StatusFail),
			},
			level:   VerdictBlocked,
			counts:  Counts{Fail: 3},
			finding: "icmp-cloudflare",
		},
		{
			name: "one anchor failing with a skip elsewhere is degraded",
			results: []Result{
				verdictResult("icmp-anchor", "1.1.1.1", StatusFail),
				verdictResult("tcp-anchor", "8.8.8.8", StatusPass),
				verdictResult("http-canary", "captive.apple.com", StatusSkipped),
			},
			level:   VerdictDegraded,
			counts:  Counts{Pass: 1, Fail: 1, Skipped: 1},
			finding: "icmp-anchor",
		},
		{
			name: "most severe finding wins over order",
			results: []Result{
				verdictResult("dns", "example.com", StatusWarn),
				verdictResult("gateway", "192.168.1.1", StatusFail),
			},
			level:   VerdictBlocked,
			counts:  Counts{Warn: 1, Fail: 1},
			finding: "gateway",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verdict, err := AggregateVerdict(test.results)
			if err != nil {
				t.Fatalf("AggregateVerdict() error = %v", err)
			}
			if verdict.Level != test.level {
				t.Fatalf("level = %q, want %q", verdict.Level, test.level)
			}
			if verdict.Counts != test.counts {
				t.Fatalf("counts = %+v, want %+v", verdict.Counts, test.counts)
			}
			if test.finding == "" {
				if verdict.Finding != nil {
					t.Fatalf("finding = %q, want none", verdict.Finding.CheckID)
				}
				return
			}
			if verdict.Finding == nil {
				t.Fatalf("finding = none, want %q", test.finding)
			}
			if verdict.Finding.CheckID != test.finding {
				t.Fatalf("finding = %q, want %q", verdict.Finding.CheckID, test.finding)
			}
		})
	}
}

func TestAggregateVerdictRequiresResults(t *testing.T) {
	if _, err := AggregateVerdict(nil); !errors.Is(err, ErrNoResults) {
		t.Fatalf("AggregateVerdict(nil) error = %v, want %v", err, ErrNoResults)
	}
}

func TestAggregateVerdictRejectsEvidenceFreeResult(t *testing.T) {
	result := verdictResult("http", "example.com", StatusFail)
	result.Measurements = nil

	if _, err := AggregateVerdict([]Result{result}); !errors.Is(err, ErrInvalidVerdictInput) {
		t.Fatalf("AggregateVerdict() error = %v, want %v", err, ErrInvalidVerdictInput)
	}
}

func TestAggregateVerdictIsDeterministic(t *testing.T) {
	results := []Result{
		verdictResult("icmp-anchor", "1.1.1.1", StatusFail),
		verdictResult("tcp-anchor", "8.8.8.8", StatusPass),
		verdictResult("dns", "example.com", StatusWarn),
	}

	first, err := AggregateVerdict(results)
	if err != nil {
		t.Fatalf("first AggregateVerdict() error = %v", err)
	}
	for i := 0; i < 10; i++ {
		next, err := AggregateVerdict(results)
		if err != nil {
			t.Fatalf("AggregateVerdict() error = %v", err)
		}
		if next.Level != first.Level || next.Counts != first.Counts {
			t.Fatalf("iteration %d = %+v, want %+v", i, next, first)
		}
		if (next.Finding == nil) != (first.Finding == nil) {
			t.Fatalf("iteration %d finding mismatch", i)
		}
		if next.Finding != nil && next.Finding.CheckID != first.Finding.CheckID {
			t.Fatalf("iteration %d finding = %q, want %q", i, next.Finding.CheckID, first.Finding.CheckID)
		}
	}
}

func TestAggregateVerdictFindingIsACopy(t *testing.T) {
	results := []Result{verdictResult("gateway", "192.168.1.1", StatusFail)}
	verdict, err := AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	verdict.Finding.Diagnostic = "mutated"
	if results[0].Diagnostic == "mutated" {
		t.Fatal("Finding aliases the input result")
	}
}

func TestAnchorForTarget(t *testing.T) {
	tests := []struct {
		target string
		anchor string
		ok     bool
	}{
		{target: "1.1.1.1", anchor: "cloudflare-dns", ok: true},
		{target: "8.8.8.8:53", anchor: "google-dns", ok: true},
		{target: "https://captive.apple.com/hotspot-detect.html", anchor: "apple-captive", ok: true},
		{target: "EXAMPLE.com.", anchor: "example-com", ok: true},
		{target: "example.com:443", anchor: "example-com", ok: true},
		{target: "http://user:pass@example.com:80/path", anchor: "example-com", ok: true},
		{target: "192.168.1.1", ok: false},
		{target: "www.example.com", ok: false},
		{target: "", ok: false},
	}

	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			anchor, ok := AnchorForTarget(test.target)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}
			if ok && anchor.ID != test.anchor {
				t.Fatalf("anchor = %q, want %q", anchor.ID, test.anchor)
			}
		})
	}
}
