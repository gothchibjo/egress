package domain

import (
	"testing"
	"time"
)

func reportStep(id string, kind StepKind, status Status, measurements ...Measurement) StepResult {
	return StepResult{
		ID:           id,
		Kind:         kind,
		Target:       "example.com",
		StartedAt:    time.Unix(0, 0),
		Status:       status,
		Measurements: measurements,
	}
}

func TestBuildTargetReportCompletePath(t *testing.T) {
	trace := reportStep("trace", StepTrace, StatusPass, measure("method", "udp"), measure("hops", "2"))
	trace.Hops = []Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3},
		{TTL: 2, Address: "203.0.113.10", Sent: 3, Received: 3},
	}
	steps := []StepResult{
		reportStep("dns", StepDNS, StatusPass, measure("system.answers", "203.0.113.10, 203.0.113.11")),
		reportStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
		reportStep("icmp", StepICMP, StatusPass, measure("address", "203.0.113.10")),
		trace,
	}

	path, hypothesis := BuildTargetReport(steps)
	if path.Localization.Kind != PathComplete {
		t.Fatalf("localization = %+v", path.Localization)
	}
	if len(path.Hops) != 2 {
		t.Fatalf("hops = %d", len(path.Hops))
	}
	if err := hypothesis.Validate(); err != nil {
		t.Fatalf("hypothesis is invalid: %v", err)
	}
}

func TestBuildTargetReportDrop(t *testing.T) {
	trace := reportStep("trace", StepTrace, StatusWarn, measure("method", "udp"))
	trace.Hops = []Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3},
		{TTL: 2, Sent: 3},
		{TTL: 3, Sent: 3},
	}
	steps := []StepResult{
		reportStep("dns", StepDNS, StatusPass, measure("system.answers", "203.0.113.10")),
		reportStep("tcp.443", StepTCP, StatusFail, measure("outcome", "timeout")),
		reportStep("icmp", StepICMP, StatusFail, measure("outcome", "no_reply")),
		trace,
	}

	path, hypothesis := BuildTargetReport(steps)
	if path.Localization.Kind != PathDrop {
		t.Fatalf("localization = %+v", path.Localization)
	}
	if hypothesis.Type != BlockIPDrop {
		t.Fatalf("hypothesis = %+v", hypothesis)
	}
}
