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
				evidenceStep("dns", StepDNS, StatusWarn, measure("comparison", "differ"), measure("system.answers", "203.0.113.10, 203.0.113.11"), measure("doh.answers", "203.0.113.10")),
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
			wantPhrase: "every ladder step passed",
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
			texts := make([]string, 0, len(hypothesis.Evidence))
			for _, line := range hypothesis.Evidence {
				texts = append(texts, line.Text)
			}
			if test.wantPhrase != "" && !strings.Contains(strings.ToLower(hypothesis.Summary+" "+strings.Join(texts, " ")), strings.ToLower(test.wantPhrase)) {
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
		Evidence:   []EvidenceLine{active("x")},
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

func evidenceTexts(hypothesis Hypothesis) string {
	texts := make([]string, 0, len(hypothesis.Evidence))
	for _, line := range hypothesis.Evidence {
		texts = append(texts, line.Text)
	}
	return strings.ToLower(strings.Join(texts, " "))
}

func TestTimeoutOnlyEvidenceIsPassive(t *testing.T) {
	steps := []StepResult{
		evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
		evidenceStep("tcp.443", StepTCP, StatusFail, measure("outcome", "timeout")),
		evidenceStep("icmp", StepICMP, StatusFail, measure("outcome", "no_reply")),
	}
	hypothesis := ClassifyTarget(steps, Localization{})

	if hypothesis.Confidence == ConfidenceHigh {
		t.Fatalf("timeout-only case got high confidence: %+v", hypothesis)
	}
	for _, line := range hypothesis.Evidence {
		if line.Mode != ObservedPassive {
			t.Fatalf("timeout evidence is labelled %q: %q", line.Mode, line.Text)
		}
	}
	if !strings.Contains(evidenceTexts(hypothesis), "rate-limited") {
		t.Fatalf("ICMP rate limiting is not named: %+v", hypothesis.Evidence)
	}
}

func TestResetEvidenceIsActive(t *testing.T) {
	steps := []StepResult{
		evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
		evidenceStep("tcp.443", StepTCP, StatusFail, measure("outcome", "reset")),
	}
	hypothesis := ClassifyTarget(steps, Localization{})
	if hypothesis.Type != BlockTCPRST {
		t.Fatalf("type = %q", hypothesis.Type)
	}
	if len(hypothesis.Evidence) == 0 || hypothesis.Evidence[0].Mode != ObservedActive {
		t.Fatalf("reset is not labelled active: %+v", hypothesis.Evidence)
	}
}

func TestSNIEvidenceIncludesAnActiveSignal(t *testing.T) {
	steps := []StepResult{
		evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
		evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
		evidenceStep("tls.real", StepTLS, StatusFail, measure("outcome", "timeout")),
		evidenceStep("tls.decoy", StepTLS, StatusPass, measure("outcome", "handshake_complete")),
	}
	hypothesis := ClassifyTarget(steps, Localization{})
	if hypothesis.Confidence != ConfidenceHigh {
		t.Fatalf("confidence = %q, want high", hypothesis.Confidence)
	}
	hasActive := false
	for _, line := range hypothesis.Evidence {
		if line.Mode == ObservedActive {
			hasActive = true
		}
	}
	if !hasActive {
		t.Fatalf("high confidence without an active observation: %+v", hypothesis.Evidence)
	}
}

func TestDNSEvidenceNamesTheCDNCaveat(t *testing.T) {
	steps := []StepResult{
		evidenceStep("dns", StepDNS, StatusWarn,
			measure("comparison", "differ"),
			measure("system.answers", "203.0.113.10, 203.0.113.11"),
			measure("doh.answers", "203.0.113.10"),
		),
	}
	hypothesis := ClassifyTarget(steps, Localization{})
	if hypothesis.Type != BlockDNS {
		t.Fatalf("type = %q", hypothesis.Type)
	}
	text := evidenceTexts(hypothesis)
	if !strings.Contains(text, "cdn") || !strings.Contains(text, "geo") {
		t.Fatalf("CDN/geo caveat is missing: %s", text)
	}
	if !strings.Contains(text, "system answers") || !strings.Contains(text, "doh answers") {
		t.Fatalf("DNS verdict does not show both answer sets: %s", text)
	}
	if hypothesis.Confidence != ConfidenceMedium {
		t.Fatalf("confidence = %q, want medium", hypothesis.Confidence)
	}
}

func TestHighConfidenceNeedsAnActiveLine(t *testing.T) {
	hypothesis := newHypothesis(BlockIPDrop, ConfidenceHigh, "x",
		[]EvidenceLine{passive("timeouts only")}, evidenceSummary{})
	if hypothesis.Confidence != ConfidenceMedium {
		t.Fatalf("confidence = %q, want the downgrade to medium", hypothesis.Confidence)
	}
}

func TestHypothesisRejectsUnlabelledEvidence(t *testing.T) {
	hypothesis := Hypothesis{
		Type:       BlockDNS,
		Confidence: ConfidenceMedium,
		Summary:    "x",
		Evidence:   []EvidenceLine{{Text: "missing mode"}},
		Caveat:     SingleVantageCaveat,
	}
	if err := hypothesis.Validate(); err == nil {
		t.Fatal("accepted evidence without a mode")
	}
}

func TestPassingTargetIsNotReportedAsUnknown(t *testing.T) {
	steps := []StepResult{
		evidenceStep("dns", StepDNS, StatusPass, measure("comparison", "match")),
		evidenceStep("tcp.443", StepTCP, StatusPass, measure("outcome", "connected")),
	}
	hypothesis := ClassifyTarget(steps, Localization{})
	if !hypothesis.NoBlockage {
		t.Fatalf("passing target is not marked as no blockage: %+v", hypothesis)
	}
	if hypothesis.Type != BlockUnknown {
		t.Fatalf("taxonomy type = %q, want unknown", hypothesis.Type)
	}
	if err := hypothesis.Validate(); err != nil {
		t.Fatalf("hypothesis is invalid: %v", err)
	}
}

func TestNoDataIsStillUnknown(t *testing.T) {
	hypothesis := ClassifyTarget(nil, Localization{})
	if hypothesis.NoBlockage {
		t.Fatalf("empty evidence was marked as no blockage: %+v", hypothesis)
	}
}
