//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
	"github.com/gothchibjo/egress/internal/report"
)

var testStart = time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC)

type stubRunner struct {
	run app.Run
	err error
}

func (s stubRunner) Run(context.Context, func(app.Progress)) (app.Run, error) {
	return s.run, s.err
}

func (s stubRunner) CancelCurrent() bool { return false }
func (s stubRunner) CancelAll() bool     { return false }

func passResult(id domain.CheckID, target string) domain.Result {
	return domain.Result{
		CheckID:      id,
		Status:       domain.StatusPass,
		Target:       target,
		Method:       "test",
		Measurements: []domain.Measurement{{Name: "reachable", Value: "true"}},
		Timestamp:    testStart,
	}
}

func warnResult(id domain.CheckID, target string) domain.Result {
	result := passResult(id, target)
	result.Status = domain.StatusWarn
	return result
}

func failResult(id domain.CheckID, target string) domain.Result {
	result := passResult(id, target)
	result.Status = domain.StatusFail
	result.Measurements = []domain.Measurement{{Name: "outcome", Value: "timeout"}}
	return result
}

func stubSession(run app.Run, err error) func(bool) baselineRunner {
	return func(bool) baselineRunner { return stubRunner{run: run, err: err} }
}

// withStubs replaces the host-dependent lookups for the duration of a test.
func withStubs(t *testing.T, destinations []domain.Destination) {
	t.Helper()
	previousPrivilege, previousDestinations := detectPrivilege, readDestinations
	t.Cleanup(func() {
		detectPrivilege = previousPrivilege
		readDestinations = previousDestinations
	})
	detectPrivilege = func() platform.Privilege { return platform.Privilege{} }
	readDestinations = func() ([]domain.Destination, error) { return destinations, nil }
}

func TestIsTerminalRejectsPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	if isTerminal(reader) {
		t.Fatal("pipe reported as a terminal")
	}
}

func TestRunJSONEmitsEnvelope(t *testing.T) {
	withStubs(t, nil)
	baseline := app.Run{
		StartedAt:  testStart,
		FinishedAt: testStart.Add(time.Second),
		Results:    []domain.Result{passResult("local.interfaces", "en0"), passResult("dns.canary", "example.com")},
	}

	var stdout, stderr bytes.Buffer
	code := runWith([]string{"--json"}, &stdout, &stderr, stubSession(baseline, nil))
	if code != exitHealthy {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, exitHealthy, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b") {
		t.Fatal("JSON output contains a terminal control sequence")
	}

	var doc report.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if doc.SchemaVersion != report.SchemaVersion {
		t.Fatalf("schema_version = %q", doc.SchemaVersion)
	}
	if doc.Mode != "unprivileged" {
		t.Fatalf("mode = %q", doc.Mode)
	}
	if doc.Verdict == nil || doc.Verdict.Level != domain.VerdictHealthy {
		t.Fatalf("verdict = %+v", doc.Verdict)
	}
	if len(doc.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(doc.Results))
	}
}

func TestRunDefaultsToJSONWithoutTerminal(t *testing.T) {
	withStubs(t, nil)
	baseline := app.Run{Results: []domain.Result{passResult("local.interfaces", "en0")}}

	var stdout, stderr bytes.Buffer
	code := runWith(nil, &stdout, &stderr, stubSession(baseline, nil))
	if code != exitHealthy {
		t.Fatalf("exit code = %d", code)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("non-terminal stdout is not JSON: %q", stdout.String())
	}
}

func TestRunJSONExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		results []domain.Result
		want    int
	}{
		{
			name:    "healthy",
			results: []domain.Result{passResult("local.interfaces", "en0"), passResult("dns.canary", "example.com")},
			want:    exitHealthy,
		},
		{
			name:    "degraded",
			results: []domain.Result{passResult("local.interfaces", "en0"), warnResult("tcp.anchor.cloudflare-dns.443", "1.1.1.1:443")},
			want:    exitDegraded,
		},
		{
			name:    "blocked",
			results: []domain.Result{passResult("local.interfaces", "en0"), failResult("tcp.anchor.custom.443", "198.51.100.10:443")},
			want:    exitBlocked,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withStubs(t, nil)
			baseline := app.Run{Results: test.results}

			var stdout, stderr bytes.Buffer
			code := runWith([]string{"--json"}, &stdout, &stderr, stubSession(baseline, nil))
			if code != test.want {
				t.Fatalf("exit code = %d, want %d", code, test.want)
			}
		})
	}
}

func TestRunJSONEmitsDestinations(t *testing.T) {
	destinations := []domain.Destination{
		{
			Kind:      domain.DestinationDoH,
			Host:      "cloudflare-dns.com",
			Port:      443,
			Transport: domain.TransportHTTPS,
			Purpose:   "public DoH comparison for the canary",
		},
	}
	withStubs(t, destinations)
	baseline := app.Run{Results: []domain.Result{passResult("local.interfaces", "en0")}}

	var stdout, stderr bytes.Buffer
	runWith([]string{"--json"}, &stdout, &stderr, stubSession(baseline, nil))

	var doc report.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if len(doc.Destinations) != 1 || doc.Destinations[0].Host != "cloudflare-dns.com" {
		t.Fatalf("destinations = %+v", doc.Destinations)
	}
}

func TestRunReportFailureExitsWithError(t *testing.T) {
	withStubs(t, nil)

	var stdout, stderr bytes.Buffer
	code := runWith([]string{"--json"}, &stdout, &stderr, stubSession(app.Run{}, errors.New("check runner failed")))
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed run wrote JSON: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "check runner failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunSchemaFlagPrintsSchema(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWith([]string{"--schema"}, &stdout, &stderr, stubSession(app.Run{}, nil))
	if code != exitHealthy {
		t.Fatalf("exit code = %d", code)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("schema output is not JSON: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), report.SchemaVersion) && !strings.Contains(stdout.String(), "schema_version") {
		t.Fatalf("schema output does not describe the envelope: %q", stdout.String())
	}
}

func TestRunUnknownFlagFailsWithUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWith([]string{"--nope"}, &stdout, &stderr, stubSession(app.Run{}, nil))
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if stdout.Len() != 0 {
		t.Fatalf("usage error wrote to stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunHelpFlagPrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWith([]string{"-h"}, &stdout, &stderr, stubSession(app.Run{}, nil))
	if code != exitHealthy {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "usage:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunRejectsPositionalArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWith([]string{"now"}, &stdout, &stderr, stubSession(app.Run{}, nil))
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
