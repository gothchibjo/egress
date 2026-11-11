package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const schemaID = "https://github.com/gothchibjo/egress/schema/result-v1.json"

func sampleResult() domain.Result {
	return domain.Result{
		CheckID:      "dns.canary",
		Status:       domain.StatusPass,
		Target:       "example.com",
		Method:       "dns-query",
		Measurements: []domain.Measurement{{Name: "latency_ms", Value: "18.2", Unit: "ms"}},
		Timestamp:    time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
}

func TestEncodeResultsStableFields(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeResults(&buf, []domain.Result{sampleResult()}); err != nil {
		t.Fatalf("EncodeResults() error = %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if got := doc["schema_version"]; got != SchemaVersion {
		t.Fatalf("schema_version = %v, want %q", got, SchemaVersion)
	}

	results, ok := doc["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v, want one item", doc["results"])
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %#v", results[0])
	}

	for _, key := range []string{"check_id", "status", "target", "method", "measurements", "timestamp"} {
		if _, exists := first[key]; !exists {
			t.Errorf("missing required field %q", key)
		}
	}
	for _, key := range []string{"diagnostic", "error"} {
		if _, exists := first[key]; exists {
			t.Errorf("empty field %q must be omitted", key)
		}
	}

	measurements, ok := first["measurements"].([]any)
	if !ok || len(measurements) != 1 {
		t.Fatalf("measurements = %#v, want one item", first["measurements"])
	}
	measurement, ok := measurements[0].(map[string]any)
	if !ok {
		t.Fatalf("measurement is not an object: %#v", measurements[0])
	}
	for _, key := range []string{"name", "value", "unit"} {
		if _, exists := measurement[key]; !exists {
			t.Errorf("missing measurement field %q", key)
		}
	}
}

func TestEncodeResultsUsesEmptyArrays(t *testing.T) {
	result := sampleResult()
	result.Measurements = nil

	var buf bytes.Buffer
	if err := EncodeResults(&buf, []domain.Result{result}); err != nil {
		t.Fatalf("EncodeResults() error = %v", err)
	}
	if !strings.Contains(buf.String(), `"measurements":[]`) {
		t.Fatalf("nil measurements encoded as null: %s", buf.String())
	}
}

func TestEncodeResultsRejectsInvalidResult(t *testing.T) {
	result := sampleResult()
	result.Status = domain.StatusFail
	result.Measurements = nil

	var buf bytes.Buffer
	err := EncodeResults(&buf, []domain.Result{result})
	if !errors.Is(err, domain.ErrMissingEvidence) {
		t.Fatalf("EncodeResults() error = %v, want %v", err, domain.ErrMissingEvidence)
	}
	if buf.Len() != 0 {
		t.Fatalf("invalid result was written: %s", buf.String())
	}
}

func TestSchemaIsValidJSON(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(Schema(), &schema); err != nil {
		t.Fatalf("embedded schema is not JSON: %v", err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected dialect: %v", schema["$schema"])
	}
}

func TestEncodeResultsMatchesSchema(t *testing.T) {
	skipped := sampleResult()
	skipped.CheckID = "icmp.anchor"
	skipped.Status = domain.StatusSkipped
	skipped.Measurements = nil
	skipped.Diagnostic = "raw sockets unavailable without elevation"

	var buf bytes.Buffer
	if err := EncodeResults(&buf, []domain.Result{sampleResult(), skipped}); err != nil {
		t.Fatalf("EncodeResults() error = %v", err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	if err := compiler.AddResource(schemaID, bytes.NewReader(Schema())); err != nil {
		t.Fatalf("compile schema resource: %v", err)
	}
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}

	var instance any
	if err := json.Unmarshal(buf.Bytes(), &instance); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("output does not match schema: %v", err)
	}
}
