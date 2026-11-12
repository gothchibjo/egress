// Package report serializes diagnostic results for TUI export and CLI output.
package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// SchemaVersion identifies the stable JSON envelope.
const SchemaVersion = "v1"

// ErrExportExists reports that the export path is already taken.
var ErrExportExists = errors.New("export file already exists")

//go:embed schema/result-v1.schema.json
var resultSchema []byte

// Envelope is the stable JSON document shared by the TUI export and CLI output.
// Optional sections are omitted when the session did not produce them.
type Envelope struct {
	SchemaVersion  string                   `json:"schema_version"`
	Mode           string                   `json:"mode,omitempty"`
	Cancelled      bool                     `json:"cancelled,omitempty"`
	Results        []domain.Result          `json:"results"`
	Verdict        *domain.Verdict          `json:"verdict,omitempty"`
	Classification *domain.Classification   `json:"classification,omitempty"`
	Destinations   []domain.Destination     `json:"destinations,omitempty"`
	Context        *platform.NetworkContext `json:"context,omitempty"`
}

// BuildEnvelope assembles a complete document from one finished session so the
// CLI and the TUI export derive the verdict and the classification the same way.
func BuildEnvelope(results []domain.Result, mode string, destinations []domain.Destination) Envelope {
	envelope := Envelope{
		Results:      results,
		Mode:         mode,
		Destinations: destinations,
	}
	if verdict, err := domain.AggregateVerdict(results); err == nil {
		envelope.Verdict = &verdict
	}
	if classification, err := domain.ClassifyRun(results); err == nil && classification.Scope != domain.ScopeNone {
		envelope.Classification = &classification
	}
	return envelope
}

// Schema returns the JSON Schema that describes Envelope.
func Schema() []byte {
	return append([]byte(nil), resultSchema...)
}

// DefaultExportName returns the documented default export filename.
func DefaultExportName(now time.Time) string {
	return "egress-" + now.UTC().Format("20060102-150405") + ".json"
}

// WriteFile writes the envelope to path and refuses to replace an existing file.
func WriteFile(path string, envelope Envelope) error {
	return writeFile(path, envelope, false)
}

// WriteFileReplace writes the envelope to path, replacing an existing file.
func WriteFileReplace(path string, envelope Envelope) error {
	return writeFile(path, envelope, true)
}

func writeFile(path string, envelope Envelope, replace bool) error {
	// Encode first so a validation failure never leaves a partial file behind.
	var buffer bytes.Buffer
	if err := Encode(&buffer, envelope); err != nil {
		return err
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if replace {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrExportExists, path)
		}
		return fmt.Errorf("open export file: %w", err)
	}
	if _, err := file.Write(buffer.Bytes()); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("write export file: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("close export file: %w", err)
	}
	return nil
}

// Encode validates one envelope and writes it to w.
func Encode(w io.Writer, envelope Envelope) error {
	envelope.SchemaVersion = SchemaVersion

	normalized := make([]domain.Result, len(envelope.Results))
	for i, r := range envelope.Results {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("result %q: %w", r.CheckID, err)
		}
		if r.Measurements == nil {
			r.Measurements = []domain.Measurement{}
		}
		normalized[i] = r
	}
	envelope.Results = normalized

	for _, destination := range envelope.Destinations {
		if err := destination.Validate(); err != nil {
			return fmt.Errorf("destination %q: %w", destination.Host, err)
		}
	}
	if envelope.Verdict != nil {
		switch envelope.Verdict.Level {
		case domain.VerdictHealthy, domain.VerdictDegraded, domain.VerdictBlocked:
		default:
			return fmt.Errorf("verdict %q is not a documented level", envelope.Verdict.Level)
		}
	}
	if envelope.Classification != nil && envelope.Classification.Scope == "" {
		return errors.New("classification has no scope")
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(envelope); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}
	return nil
}

// EncodeResults writes an envelope that carries results and no destinations.
func EncodeResults(w io.Writer, results []domain.Result) error {
	return Encode(w, Envelope{Results: results})
}
