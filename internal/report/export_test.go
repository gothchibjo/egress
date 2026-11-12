package report

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

func localPass() domain.Result {
	return domain.Result{
		CheckID:      "local.interfaces",
		Status:       domain.StatusPass,
		Target:       "en0",
		Method:       "platform inspection",
		Measurements: []domain.Measurement{{Name: "platform", Value: "test"}},
		Timestamp:    time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC),
	}
}

func dnsFailure() domain.Result {
	return domain.Result{
		CheckID:      "dns.canary",
		Status:       domain.StatusFail,
		Target:       "example.com",
		Method:       "dns query",
		Measurements: []domain.Measurement{{Name: "resolver.192.0.2.53.rcode", Value: "timeout"}},
		Timestamp:    time.Date(2026, 11, 12, 20, 0, 1, 0, time.UTC),
	}
}

func TestBuildEnvelopeDerivesVerdictAndScope(t *testing.T) {
	envelope := BuildEnvelope([]domain.Result{localPass(), dnsFailure()}, "unprivileged", nil)

	if envelope.Verdict == nil {
		t.Fatal("envelope has no verdict")
	}
	// A failing anchor target is downgraded to degraded, but the scope still
	// points at name resolution.
	if envelope.Verdict.Level != domain.VerdictDegraded {
		t.Fatalf("verdict = %q, want degraded", envelope.Verdict.Level)
	}
	if envelope.Classification == nil {
		t.Fatal("envelope has no classification")
	}
	if envelope.Classification.Scope != domain.ScopeDNS {
		t.Fatalf("scope = %q, want dns", envelope.Classification.Scope)
	}
	if envelope.Mode != "unprivileged" {
		t.Fatalf("mode = %q", envelope.Mode)
	}
}

func TestBuildEnvelopeBlockedByNonAnchorFailure(t *testing.T) {
	failed := domain.Result{
		CheckID:      "tcp.anchor.custom.443",
		Status:       domain.StatusFail,
		Target:       "198.51.100.10:443",
		Method:       "tcp connect",
		Measurements: []domain.Measurement{{Name: "outcome", Value: "timeout"}},
		Timestamp:    time.Date(2026, 11, 12, 20, 0, 2, 0, time.UTC),
	}
	envelope := BuildEnvelope([]domain.Result{localPass(), failed}, "elevated", nil)

	if envelope.Verdict == nil || envelope.Verdict.Level != domain.VerdictBlocked {
		t.Fatalf("verdict = %+v, want blocked", envelope.Verdict)
	}
	if envelope.Classification == nil || envelope.Classification.Scope != domain.ScopeUpstream {
		t.Fatalf("classification = %+v, want upstream", envelope.Classification)
	}
}

func TestBuildEnvelopeOmitsHealthyClassification(t *testing.T) {
	envelope := BuildEnvelope([]domain.Result{localPass()}, "elevated", nil)
	if envelope.Classification != nil {
		t.Fatalf("healthy run carries a classification: %+v", envelope.Classification)
	}
	if envelope.Verdict == nil || envelope.Verdict.Level != domain.VerdictHealthy {
		t.Fatalf("verdict = %+v", envelope.Verdict)
	}
}

func TestEnvelopeWithVerdictMatchesSchema(t *testing.T) {
	envelope := BuildEnvelope([]domain.Result{localPass(), dnsFailure()}, "unprivileged", sampleDestinations())
	envelopeJSON := encodeToMap(t, envelope)

	schema := compileSchema(t)
	if err := schema.Validate(envelopeJSON); err != nil {
		t.Fatalf("envelope does not match schema: %v", err)
	}
}

func TestDefaultExportName(t *testing.T) {
	now := time.Date(2026, 11, 12, 20, 37, 35, 0, time.FixedZone("MSK", 3*60*60))
	if got := DefaultExportName(now); got != "egress-20261112-173735.json" {
		t.Fatalf("DefaultExportName() = %q", got)
	}
}

func TestWriteFileRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "egress-20261112-203735.json")
	envelope := BuildEnvelope([]domain.Result{localPass()}, "unprivileged", nil)

	if err := WriteFile(path, envelope); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := WriteFile(path, envelope); !errors.Is(err, ErrExportExists) {
		t.Fatalf("second WriteFile() error = %v, want %v", err, ErrExportExists)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		t.Fatalf("export permissions = %v, want owner-only", mode)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	if doc["schema_version"] != SchemaVersion {
		t.Fatalf("schema_version = %v", doc["schema_version"])
	}
}

func TestWriteFileReplaceOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "egress.json")
	first := BuildEnvelope([]domain.Result{localPass()}, "unprivileged", nil)
	second := BuildEnvelope([]domain.Result{localPass(), dnsFailure()}, "elevated", nil)

	if err := WriteFile(path, first); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := WriteFileReplace(path, second); err != nil {
		t.Fatalf("WriteFileReplace() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var doc struct {
		Mode    string `json:"mode"`
		Results []any  `json:"results"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	if doc.Mode != "elevated" || len(doc.Results) != 2 {
		t.Fatalf("export was not replaced: %+v", doc)
	}
}

func TestWriteFileRejectsInvalidEnvelopeAndLeavesNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "egress.json")
	broken := Envelope{Results: []domain.Result{{CheckID: "local.interfaces", Status: domain.StatusFail}}}

	if err := WriteFile(path, broken); err == nil {
		t.Fatal("WriteFile() accepted an invalid envelope")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid export left a file: %v", err)
	}
}
