package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

func compileSessionSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat = true
	if err := compiler.AddResource("session-v1", bytes.NewReader(SessionSchema())); err != nil {
		t.Fatalf("compile session schema resource: %v", err)
	}
	schema, err := compiler.Compile("session-v1")
	if err != nil {
		t.Fatalf("compile session schema: %v", err)
	}
	return schema
}

func sessionTarget(state string, steps []domain.StepResult) SessionTarget {
	path, hypothesis := domain.BuildTargetReport(steps)
	return SessionTarget{
		ID:         "alpha",
		Label:      "Alpha",
		Host:       "alpha.example",
		Ports:      []int{443},
		Protocols:  []domain.PresetProtocol{domain.PresetTCP, domain.PresetTLS, domain.PresetICMP},
		State:      state,
		Steps:      steps,
		Path:       path,
		Hypothesis: hypothesis,
	}
}

func sessionSteps() []domain.StepResult {
	started := time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)
	dns := domain.StepResult{
		ID: "dns", Kind: domain.StepDNS, Target: "alpha.example", StartedAt: started,
		DurationMS: 12, Status: domain.StatusPass,
		Measurements: []domain.Measurement{{Name: "system.answers", Value: "203.0.113.10"}},
	}
	tcp := domain.StepResult{
		ID: "tcp.443", Kind: domain.StepTCP, Target: "alpha.example", StartedAt: started.Add(20 * time.Millisecond),
		DurationMS: 30, Status: domain.StatusPass,
		Measurements: []domain.Measurement{{Name: "outcome", Value: "connected"}},
	}
	trace := domain.StepResult{
		ID: "trace", Kind: domain.StepTrace, Target: "alpha.example", StartedAt: started.Add(60 * time.Millisecond),
		DurationMS: 500, Status: domain.StatusPass,
		Measurements: []domain.Measurement{{Name: "method", Value: "udp"}, {Name: "address", Value: "203.0.113.10"}},
		Hops: []domain.Hop{
			{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3, RTTMedianMS: 10},
			{TTL: 2, Address: "203.0.113.10", Sent: 3, Received: 3, RTTMedianMS: 25},
		},
	}
	return []domain.StepResult{dns, tcp, trace}
}

func validSessionEnvelope() SessionEnvelope {
	started := time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)
	return BuildSessionEnvelope(
		[]SessionTarget{sessionTarget("done", sessionSteps())},
		started, started.Add(2*time.Second), "unprivileged", false, nil,
	)
}

func sessionDestinations() []domain.SessionDestination {
	return []domain.SessionDestination{
		{Kind: domain.SessionTarget, Host: "alpha.example", Address: "203.0.113.10", Port: 443,
			Transport: domain.SessionTLS, Purpose: "TLS handshake to the selected target"},
		{Kind: domain.SessionTarget, Host: "alpha.example", Address: "203.0.113.10",
			Transport: domain.SessionTraceUDP, Purpose: "TTL-limited trace to the selected target"},
		{Kind: domain.SessionResolver, Host: "192.0.2.53", Port: 53,
			Transport: domain.SessionUDPAndTCP, Purpose: "configured resolver for name lookups"},
		{Kind: domain.SessionDoH, Host: "cloudflare-dns.com", Port: 443,
			Transport: domain.SessionTLS, Purpose: "public DoH comparison for name lookups"},
	}
}

func TestSessionEnvelopeMatchesSchema(t *testing.T) {
	envelope := validSessionEnvelope()
	envelope.Destinations = sessionDestinations()
	envelope.Context = &platform.NetworkContext{
		Proxies: []platform.ProxySetting{{Name: "HTTPS_PROXY", Source: "https_proxy", Value: "http://***@proxy.example:8080"}},
		VPN:     []platform.VPNInterface{},
	}

	doc := encodeSessionToMap(t, envelope)
	if err := compileSessionSchema(t).Validate(doc); err != nil {
		t.Fatalf("session envelope does not match schema: %v", err)
	}
}

func encodeSessionToMap(t *testing.T, envelope SessionEnvelope) any {
	t.Helper()
	var buffer bytes.Buffer
	if err := EncodeSession(&buffer, envelope); err != nil {
		t.Fatalf("EncodeSession() error = %v", err)
	}
	var doc any
	if err := json.Unmarshal(buffer.Bytes(), &doc); err != nil {
		t.Fatalf("session output is not JSON: %v", err)
	}
	return doc
}

func TestSessionEnvelopeCarriesTheWholeRun(t *testing.T) {
	envelope := validSessionEnvelope()
	envelope.Destinations = sessionDestinations()
	var buffer bytes.Buffer
	if err := EncodeSession(&buffer, envelope); err != nil {
		t.Fatalf("EncodeSession() error = %v", err)
	}
	for _, want := range []string{
		`"schema_version":"session-v1"`,
		`"catalog_version":"v1"`,
		`"mode":"unprivileged"`,
		`"state":"done"`,
		`"kind":"trace"`,
		`"hops":[{"ttl":1`,
		`"localization":{"kind":"complete"`,
		`"confidence":"low"`,
		`"caveat":"Single vantage point: one observation from one network. False-positive risks:`,
		`Re-run from another network to validate."`,
		`"kind":"target"`,
		`"address":"203.0.113.10"`,
		`"kind":"resolver"`,
	} {
		if !strings.Contains(buffer.String(), want) {
			t.Fatalf("session export is missing %q: %s", want, buffer.String())
		}
	}
}

func TestSessionEnvelopeKeepsCancelledTargetsAndPartialSteps(t *testing.T) {
	started := time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)
	partial := sessionTarget("cancelled", sessionSteps()[:1])
	envelope := BuildSessionEnvelope([]SessionTarget{partial}, started, started.Add(time.Second), "elevated", true, nil)

	var buffer bytes.Buffer
	if err := EncodeSession(&buffer, envelope); err != nil {
		t.Fatalf("EncodeSession() error = %v", err)
	}
	for _, want := range []string{`"cancelled":true`, `"state":"cancelled"`, `"id":"dns"`} {
		if !strings.Contains(buffer.String(), want) {
			t.Fatalf("partial session is missing %q: %s", want, buffer.String())
		}
	}
}

func TestSessionEnvelopeRejectsAnInvalidTarget(t *testing.T) {
	started := time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC)
	cases := map[string]SessionEnvelope{
		"no targets": BuildSessionEnvelope(nil, started, started, "unprivileged", false, nil),
		"no start": BuildSessionEnvelope([]SessionTarget{sessionTarget("done", sessionSteps())},
			time.Time{}, started, "unprivileged", false, nil),
		"bad state": BuildSessionEnvelope([]SessionTarget{sessionTarget("mystery", sessionSteps())},
			started, started, "unprivileged", false, nil),
		"invalid step": BuildSessionEnvelope([]SessionTarget{{ID: "alpha", Host: "alpha.example", State: "done",
			Steps: []domain.StepResult{{ID: "dns", Kind: domain.StepDNS, Status: domain.StatusFail}},
			Path:  domain.PathReport{Localization: domain.Localization{Kind: domain.PathUnknown, Summary: "no path data"}},
		}}, started, started, "unprivileged", false, nil),
	}
	for name, envelope := range cases {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			if err := EncodeSession(&buffer, envelope); !errors.Is(err, ErrInvalidSession) {
				t.Fatalf("EncodeSession() error = %v, want ErrInvalidSession", err)
			}
		})
	}
}

func TestSessionExportWritesNoSecrets(t *testing.T) {
	t.Setenv("EGRESS_TEST_SECRET", "do-not-export-this")
	envelope := validSessionEnvelope()
	var buffer bytes.Buffer
	if err := EncodeSession(&buffer, envelope); err != nil {
		t.Fatalf("EncodeSession() error = %v", err)
	}
	if strings.Contains(buffer.String(), "do-not-export-this") {
		t.Fatal("session export contains environment data")
	}
}

func TestWriteSessionFileRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "egress-session.json")
	envelope := validSessionEnvelope()

	if err := WriteSessionFile(path, envelope); err != nil {
		t.Fatalf("WriteSessionFile() error = %v", err)
	}
	if err := WriteSessionFile(path, envelope); !errors.Is(err, ErrExportExists) {
		t.Fatalf("second WriteSessionFile() error = %v, want ErrExportExists", err)
	}
	if err := WriteSessionFileReplace(path, envelope); err != nil {
		t.Fatalf("WriteSessionFileReplace() error = %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("replaced session export is missing: %v", err)
	}
}
