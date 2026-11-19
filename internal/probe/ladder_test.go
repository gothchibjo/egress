package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

type fakeConn struct{}

func (fakeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (fakeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (fakeConn) Close() error                     { return nil }
func (fakeConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (fakeConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (fakeConn) SetDeadline(time.Time) error      { return nil }
func (fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (fakeConn) SetWriteDeadline(time.Time) error { return nil }

func ladderHTTPResponse() *http.Response {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func testLadder() Ladder {
	return Ladder{
		timeout: 200 * time.Millisecond,
		now:     time.Now,
		resolve: func(context.Context, string) ([]string, error) {
			return []string{"93.184.216.34"}, nil
		},
		doh: &fakeDoH{answer: DoHAnswer{Answers: []string{"93.184.216.34"}}},
		dial: func(context.Context, string, string) (net.Conn, error) {
			return fakeConn{}, nil
		},
		http: &fakeDoer{resp: ladderHTTPResponse()},
		echo: func(context.Context, net.IP) (time.Duration, error) {
			return 12 * time.Millisecond, nil
		},
		handshake: func(context.Context, net.Conn, string) error { return nil },
	}
}

func presetTarget(t *testing.T, id string) LadderTarget {
	t.Helper()
	for _, preset := range domain.PresetCatalog() {
		if preset.ID == id {
			return TargetFromPreset(preset)
		}
	}
	t.Fatalf("preset %q not found", id)
	return LadderTarget{}
}

func TestLadderRunsEveryStepInOrder(t *testing.T) {
	ladder := testLadder()
	steps := ladder.Run(context.Background(), presetTarget(t, "youtube"))

	wantIDs := []string{"dns", "tcp.443", "tls.real", "tls.decoy", "tls.absent", "http", "icmp", "trace"}
	if len(steps) != len(wantIDs) {
		t.Fatalf("steps = %d, want %d: %+v", len(steps), len(wantIDs), steps)
	}
	for i, step := range steps {
		if step.ID != wantIDs[i] {
			t.Fatalf("step %d = %q, want %q", i, step.ID, wantIDs[i])
		}
		if step.Order != i+1 {
			t.Fatalf("step %q has order %d, want %d", step.ID, step.Order, i+1)
		}
		if step.StartedAt.IsZero() || step.Target == "" {
			t.Fatalf("step %q misses timing or target: %+v", step.ID, step)
		}
		if err := step.Validate(); err != nil {
			t.Fatalf("step %q is invalid: %v", step.ID, err)
		}
	}

	if steps[len(steps)-1].Status != domain.StatusSkipped {
		t.Fatalf("trace step = %q, want skipped until REQ-005", steps[len(steps)-1].Status)
	}
}

func TestLadderSkipsDNSForIPLiteral(t *testing.T) {
	ladder := testLadder()
	target := LadderTarget{ID: "ip", Label: "1.2.3.4", Host: "1.2.3.4", Ports: []int{443}, Protocols: []domain.PresetProtocol{domain.PresetTCP, domain.PresetICMP}}

	steps := ladder.Run(context.Background(), target)
	if steps[0].Kind != domain.StepDNS || steps[0].Status != domain.StatusSkipped {
		t.Fatalf("dns step = %+v", steps[0])
	}
	if !strings.Contains(steps[0].Diagnostic, "IP literal") {
		t.Fatalf("dns diagnostic = %q", steps[0].Diagnostic)
	}
	if err := steps[0].Validate(); err != nil {
		t.Fatalf("dns step invalid: %v", err)
	}
}

func TestLadderDNSFailureDoesNotStopTCP(t *testing.T) {
	ladder := testLadder()
	ladder.resolve = func(context.Context, string) ([]string, error) {
		return nil, errors.New("no such host")
	}
	dialed := 0
	ladder.dial = func(context.Context, string, string) (net.Conn, error) {
		dialed++
		return fakeConn{}, nil
	}

	steps := ladder.Run(context.Background(), presetTarget(t, "github"))
	if steps[0].Status != domain.StatusFail {
		t.Fatalf("dns step = %+v", steps[0])
	}
	if dialed == 0 {
		t.Fatal("TCP step did not run after the DNS failure")
	}
	if steps[1].Kind != domain.StepTCP || steps[1].Status != domain.StatusPass {
		t.Fatalf("tcp step = %+v", steps[1])
	}
}

func TestLadderStepFailureDoesNotCancelLaterSteps(t *testing.T) {
	ladder := testLadder()
	ladder.dial = func(context.Context, string, string) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	}
	ladder.http = &fakeDoer{err: errors.New("connection refused")}

	steps := ladder.Run(context.Background(), presetTarget(t, "github"))
	if len(steps) != 8 {
		t.Fatalf("steps = %d, want 8", len(steps))
	}
	for _, step := range steps {
		if step.Kind == domain.StepTCP || step.Kind == domain.StepTLS || step.Kind == domain.StepHTTP {
			if step.Status == domain.StatusPass {
				t.Fatalf("step %q passed without a connection", step.ID)
			}
			if step.Status == domain.StatusSkipped {
				t.Fatalf("step %q skipped instead of failing", step.ID)
			}
		}
	}
	if steps[6].Kind != domain.StepICMP || steps[6].Status != domain.StatusPass {
		t.Fatalf("icmp step = %+v", steps[6])
	}
}

func TestLadderTLSVariantsAreDistinguishable(t *testing.T) {
	ladder := testLadder()
	var names []string
	ladder.handshake = func(_ context.Context, _ net.Conn, serverName string) error {
		names = append(names, serverName)
		return nil
	}

	steps := ladder.Run(context.Background(), presetTarget(t, "github"))
	wantNames := []string{"github.com", decoySNI, ""}
	if len(names) != len(wantNames) {
		t.Fatalf("handshakes = %d, want %d", len(names), len(wantNames))
	}
	for i, want := range wantNames {
		if names[i] != want {
			t.Fatalf("handshake %d used SNI %q, want %q", i, names[i], want)
		}
	}

	ids := []string{steps[2].ID, steps[3].ID, steps[4].ID}
	for i, want := range []string{"tls.real", "tls.decoy", "tls.absent"} {
		if ids[i] != want {
			t.Fatalf("tls step %d = %q, want %q", i, ids[i], want)
		}
	}
}

func TestLadderTLSAlertIsAWarning(t *testing.T) {
	ladder := testLadder()
	ladder.handshake = func(context.Context, net.Conn, string) error {
		return tls.AlertError(112)
	}

	steps := ladder.Run(context.Background(), presetTarget(t, "github"))
	step := steps[2]
	if step.Status != domain.StatusWarn {
		t.Fatalf("tls step = %q, want warn", step.Status)
	}
	if !strings.Contains(step.Diagnostic, "alert") {
		t.Fatalf("diagnostic = %q", step.Diagnostic)
	}
}

func TestLadderTimeoutsAreBounded(t *testing.T) {
	ladder := testLadder()
	ladder.timeout = 50 * time.Millisecond
	ladder.dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	steps := ladder.Run(context.Background(), presetTarget(t, "github"))
	for _, step := range steps {
		if step.DurationMS > 500 {
			t.Fatalf("step %q took %d ms", step.ID, step.DurationMS)
		}
	}
	if steps[1].Status != domain.StatusFail {
		t.Fatalf("tcp step = %+v", steps[1])
	}
	if steps[6].Kind != domain.StepICMP {
		t.Fatalf("later steps did not run: %+v", steps)
	}
}

func TestTargetFromPresetAndCustom(t *testing.T) {
	preset := presetTarget(t, "cloudflare-dns")
	if preset.Host != "1.1.1.1" || len(preset.Ports) != 2 {
		t.Fatalf("preset target = %+v", preset)
	}

	custom := TargetFromCustom(domain.Target{Host: "example.net", Port: 8443, Protocol: domain.PresetTLS})
	if custom.Host != "example.net" || len(custom.Ports) != 1 || custom.Ports[0] != 8443 {
		t.Fatalf("custom target = %+v", custom)
	}
	if len(custom.Protocols) != 1 || custom.Protocols[0] != domain.PresetTLS {
		t.Fatalf("custom protocols = %+v", custom.Protocols)
	}
}

func TestLadderHonoursProtocolFilter(t *testing.T) {
	ladder := testLadder()
	target := LadderTarget{
		ID: "icmp-only", Label: "icmp-only", Host: "example.net",
		Ports: []int{443}, Protocols: []domain.PresetProtocol{domain.PresetICMP},
	}

	steps := ladder.Run(context.Background(), target)
	if len(steps) != 3 {
		t.Fatalf("steps = %d, want dns, icmp and trace: %+v", len(steps), steps)
	}
	if steps[1].Kind != domain.StepICMP {
		t.Fatalf("second step = %q, want icmp", steps[1].Kind)
	}
}

func TestLadderIntegration(t *testing.T) {
	if os.Getenv("EGRESS_INTEGRATION") == "" {
		t.Skip("set EGRESS_INTEGRATION=1 to run the live ladder")
	}

	ladder := NewLadder(false)
	target := TargetFromCustom(domain.Target{Host: "example.com"})
	steps := ladder.Run(context.Background(), target)

	if len(steps) != 8 {
		t.Fatalf("steps = %d, want 8", len(steps))
	}
	passed := make(map[domain.StepKind]bool)
	for _, step := range steps {
		if err := step.Validate(); err != nil {
			t.Fatalf("step %q is invalid: %v", step.ID, err)
		}
		if step.DurationMS < 0 {
			t.Fatalf("step %q has a negative duration", step.ID)
		}
		if step.Status == domain.StatusPass {
			passed[step.Kind] = true
		}
		if step.Kind == domain.StepDNS && step.Status == domain.StatusFail {
			t.Fatalf("dns step failed on the live network: %+v", step)
		}
		if step.Kind == domain.StepICMP && step.Status == domain.StatusSkipped {
			t.Fatalf("icmp step was skipped: %+v", step)
		}
	}
	for _, kind := range []domain.StepKind{domain.StepTCP, domain.StepTLS, domain.StepHTTP} {
		if !passed[kind] {
			t.Fatalf("step %q did not pass on the live network", kind)
		}
	}
}

func TestLadderIncludesTheTraceStep(t *testing.T) {
	ladder := testLadder()
	ladder.trace = TraceStep(fakeTracer{hops: []domain.Hop{{TTL: 1, Address: "93.184.216.34", Sent: 3, Received: 3}}})

	steps := ladder.Run(context.Background(), presetTarget(t, "github"))
	trace := steps[len(steps)-1]
	if trace.Kind != domain.StepTrace || trace.ID != "trace" {
		t.Fatalf("last step = %+v", trace)
	}
	if trace.Status != domain.StatusPass {
		t.Fatalf("trace status = %q", trace.Status)
	}
	if measurementValue(trace.Measurements, "method") != "fake" {
		t.Fatalf("trace method = %q", measurementValue(trace.Measurements, "method"))
	}
	if measurementValue(trace.Measurements, "hops") != "1" {
		t.Fatalf("trace hops = %q", measurementValue(trace.Measurements, "hops"))
	}
}

func TestLadderStopsWhenCancelled(t *testing.T) {
	ladder := testLadder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	steps := ladder.Run(ctx, presetTarget(t, "github"))
	if len(steps) != 1 || steps[0].Kind != domain.StepDNS {
		t.Fatalf("steps after cancellation = %d, want only dns: %+v", len(steps), steps)
	}
}

func TestLadderPlanMatchesStartEvents(t *testing.T) {
	ladder := testLadder()
	target := presetTarget(t, "youtube")
	plan := ladder.Plan(target)

	started := make([]string, 0, len(plan))
	ladder.RunWithProgress(context.Background(), target, func(step domain.StepPlan) {
		started = append(started, step.ID)
	}, nil)

	if len(started) != len(plan) {
		t.Fatalf("start events = %d, plan = %d", len(started), len(plan))
	}
	for i := range plan {
		if started[i] != plan[i].ID {
			t.Fatalf("start %d = %q, plan = %q", i, started[i], plan[i].ID)
		}
	}
}
