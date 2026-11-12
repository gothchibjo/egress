package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	if err := compiler.AddResource(schemaID, bytes.NewReader(Schema())); err != nil {
		t.Fatalf("compile schema resource: %v", err)
	}
	schema, err := compiler.Compile(schemaID)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return schema
}

func encodeToMap(t *testing.T, envelope Envelope) any {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, envelope); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var doc any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	return doc
}
