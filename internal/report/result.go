// Package report serializes diagnostic results for TUI export and CLI output.
package report

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gothchibjo/egress/internal/domain"
)

// SchemaVersion identifies the stable JSON envelope.
const SchemaVersion = "v1"

//go:embed schema/result-v1.schema.json
var resultSchema []byte

// Envelope is the stable JSON document shared by the TUI export and CLI output.
// Destinations carries the documented outbound endpoints; it is omitted when
// the caller has no list to report.
type Envelope struct {
	SchemaVersion string               `json:"schema_version"`
	Results       []domain.Result      `json:"results"`
	Destinations  []domain.Destination `json:"destinations,omitempty"`
}

// Schema returns the JSON Schema that describes Envelope.
func Schema() []byte {
	return append([]byte(nil), resultSchema...)
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
