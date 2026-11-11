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
type Envelope struct {
	SchemaVersion string          `json:"schema_version"`
	Results       []domain.Result `json:"results"`
}

// Schema returns the JSON Schema that describes Envelope.
func Schema() []byte {
	return append([]byte(nil), resultSchema...)
}

// EncodeResults validates results and writes one Envelope to w.
func EncodeResults(w io.Writer, results []domain.Result) error {
	normalized := make([]domain.Result, len(results))
	for i, r := range results {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("result %q: %w", r.CheckID, err)
		}
		if r.Measurements == nil {
			r.Measurements = []domain.Measurement{}
		}
		normalized[i] = r
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(Envelope{SchemaVersion: SchemaVersion, Results: normalized}); err != nil {
		return fmt.Errorf("encode results: %w", err)
	}
	return nil
}
