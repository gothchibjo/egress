package domain

import (
	"strings"
	"testing"
	"time"
)

func evidenceStep(id string, kind StepKind, status Status, measurements ...Measurement) StepResult {
	return StepResult{
		ID:           id,
		Kind:         kind,
		Target:       "example.com",
		StartedAt:    time.Unix(0, 0),
		Status:       status,
		Measurements: measurements,
	}
}

func measure(name, value string) Measurement {
	return Measurement{Name: name, Value: value}
}

func TestClassifyTargetScenarios(t *testing.T) {
	tests := []struct {
		name       string
		steps      []StepResult
		path       Localization
		wantType   BlockageType
		wantConf   Confidence
		wantPhrase string
	}{
		{
			name: "sni filtering",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
				evidenceStep("tls.real", StepTLS, StatusFail, measure("outcome", "timeout")),
				evidenceStep("tls.decoy", StepTLS, StatusPass, measure("outcome", "handshake_complete")),
				evidenceStep("tls.absent", StepTLS, StatusPass, measure("outcome", "handshake_complete")),
				evidenceStep("http", StepHTTP, StatusPass),
			},
			wantType:   BlockSNIFilter,
			wantConf:   ConfidenceHigh,
			wantPhrase: "decoy SNI",
		},
		{
			name: "dns divergence",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusWarn, measure("comparison", "differ"), measure("doh.answers", "203.0.113.10")),
				evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
				evidenceStep("tls.real", StepTLS, StatusPass),
				evidenceStep("http", StepHTTP, StatusPass),
			},
			wantType:   BlockDNS,
			wantConf:   ConfidenceMedium,
			wantPhrase: "differ",
		},
		{
			name: "system resolver fails while doh answers",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusFail, measure("doh.answers", "203.0.113.10")),
			},
			wantType:   BlockDNS,
			wantConf:   ConfidenceMedium,
			wantPhrase: "DoH answered",
		},
		{
			name: "tcp reset",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusFail, measure("outcome", "reset")),
			},
			wantType:   BlockTCPRST,
			wantConf:   ConfidenceMedium,
			wantPhrase: "reset",
		},
		{
			name: "port divergence",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
				evidenceStep("tcp.53", StepTCP, StatusFail, measure("outcome", "timeout")),
			},
			wantType:   BlockPortBlock,
			wantConf:   ConfidenceMedium,
			wantPhrase: "ports",
		},
		{
			name: "http block",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
				evidenceStep("tls.real", StepTLS, StatusPass),
				evidenceStep("http", StepHTTP, StatusFail, measure("status", "403")),
			},
			wantType:   BlockHTTP,
			wantConf:   ConfidenceLow,
			wantPhrase: "HTTP",
		},
		{
			name: "ip drop with path corroboration",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusFail, measure("outcome", "timeout")),
				evidenceStep("icmp", StepICMP, StatusFail, measure("outcome", "no_reply")),
			},
			path:       Localization{Kind: PathDrop, AfterTTL: 5, Summary: "drop after hop 5: no reply from the hops that follow"},
			wantType:   BlockIPDrop,
			wantConf:   ConfidenceMedium,
			wantPhrase: "hop",
		},
		{
			name: "timeout only stays low",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusFail, measure("outcome", "timeout")),
			},
			wantType: BlockIPDrop,
			wantConf: ConfidenceLow,
		},
		{
			name: "everything passes",
			steps: []StepResult{
				evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
				evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
				evidenceStep("tls.real", StepTLS, StatusPass),
				evidenceStep("http", StepHTTP, StatusPass),
				evidenceStep("icmp", StepICMP, StatusPass),
				evidenceStep("trace", StepTrace, StatusSkipped),
			},
			wantType:   BlockUnknown,
			wantConf:   ConfidenceLow,
			wantPhrase: "no blockage detected",
		},
		{
			name:       "no data",
			steps:      nil,
			wantType:   BlockUnknown,
			wantConf:   ConfidenceLow,
			wantPhrase: "insufficient evidence",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hypothesis := ClassifyTarget(test.steps, test.path)
			if hypothesis.Type != test.wantType {
				t.Fatalf("type = %q, want %q", hypothesis.Type, test.wantType)
			}
			if hypothesis.Confidence != test.wantConf {
				t.Fatalf("confidence = %q, want %q", hypothesis.Confidence, test.wantConf)
			}
			if test.wantPhrase != "" && !strings.Contains(strings.ToLower(hypothesis.Summary+" "+strings.Join(hypothesis.Evidence, " ")), strings.ToLower(test.wantPhrase)) {
				t.Fatalf("hypothesis does not mention %q: %+v", test.wantPhrase, hypothesis)
			}
			if err := hypothesis.Validate(); err != nil {
				t.Fatalf("hypothesis is invalid: %v", err)
			}
			if len(hypothesis.Rejected) != 7 {
				t.Fatalf("rejected alternatives = %d, want 7", len(hypothesis.Rejected))
			}
			for _, rejected := range hypothesis.Rejected {
				if rejected.Type == hypothesis.Type {
					t.Fatalf("hypothesis rejects its own type: %+v", rejected)
				}
				if strings.TrimSpace(rejected.Reason) == "" {
					t.Fatalf("rejected %q has no reason", rejected.Type)
				}
			}
			if hypothesis.Caveat != SingleVantageCaveat {
				t.Fatalf("caveat = %q", hypothesis.Caveat)
			}
		})
	}
}

func TestHypothesisRejectsUnknownTaxonomy(t *testing.T) {
	hypothesis := Hypothesis{
		Type:       "censorship",
		Confidence: ConfidenceHigh,
		Summary:    "x",
		Evidence:   []string{"x"},
		Caveat:     SingleVantageCaveat,
	}
	if err := hypothesis.Validate(); err == nil {
		t.Fatal("accepted a type outside the taxonomy")
	}
}

func TestHypothesisRequiresEvidence(t *testing.T) {
	hypothesis := Hypothesis{
		Type:       BlockDNS,
		Confidence: ConfidenceHigh,
		Summary:    "x",
		Caveat:     SingleVantageCaveat,
	}
	if err := hypothesis.Validate(); err == nil {
		t.Fatal("accepted a hypothesis without evidence")
	}
}
