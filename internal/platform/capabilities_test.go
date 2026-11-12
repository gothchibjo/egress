package platform

import "testing"

func TestSystemCapabilities(t *testing.T) {
	caps := System()
	if caps.Platform() == "" {
		t.Fatal("capabilities have no platform name")
	}
	if _, err := caps.Interfaces(); err != nil {
		t.Fatalf("Interfaces() error = %v", err)
	}
	if caps.Tracing().Supported {
		t.Fatal("tracing is reported as supported in v1")
	}
	if caps.Tracing().Reason == "" {
		t.Fatal("unsupported tracing has no reason")
	}
}
