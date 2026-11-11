package probe

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type fakeTCPDial struct {
	calls   int
	err     error
	conn    net.Conn
	address string
}

func (f *fakeTCPDial) dial(_ context.Context, _, address string) (net.Conn, error) {
	f.calls++
	f.address = address
	if f.err != nil {
		return nil, f.err
	}
	return f.conn, nil
}

func testTCPCheck(host string, port int, opts TCPOptions, dial *fakeTCPDial) TCPCheck {
	return TCPCheck{
		id:   domain.CheckID("tcp.test"),
		host: host,
		port: port,
		opts: opts.normalized(),
		dial: dial.dial,
		now:  func() time.Time { return fixedTime },
	}
}

func TestTCPCheckConnected(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	check := newTCPCheck(domain.Anchor{ID: "local", Host: "127.0.0.1"}, port, TCPOptions{Timeout: time.Second})
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
	if value, _ := findMeasurement(result, "outcome"); value != "connected" {
		t.Fatalf("outcome = %q", value)
	}
	if value, _ := findMeasurement(result, "latency"); value == "" {
		t.Fatal("connected result has no latency")
	}
	if result.Target != net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) {
		t.Fatalf("target = %q", result.Target)
	}
}

func TestTCPCheckRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	check := newTCPCheck(domain.Anchor{ID: "local", Host: "127.0.0.1"}, port, TCPOptions{Timeout: time.Second})
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn: %+v", result.Status, result)
	}
	if value, _ := findMeasurement(result, "outcome"); value != "refused" {
		t.Fatalf("outcome = %q, want refused", value)
	}
	if !strings.Contains(result.Diagnostic, "actively refused") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestTCPCheckTimeout(t *testing.T) {
	dial := &fakeTCPDial{err: timeoutError{}}
	check := testTCPCheck("1.1.1.1", 443, TCPOptions{}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if value, _ := findMeasurement(result, "outcome"); value != "timeout" {
		t.Fatalf("outcome = %q, want timeout", value)
	}
	if !strings.Contains(result.Diagnostic, "passive") {
		t.Fatalf("diagnostic = %q, want a passive-timeout label", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestTCPCheckReset(t *testing.T) {
	dial := &fakeTCPDial{err: syscall.ECONNRESET}
	check := testTCPCheck("1.1.1.1", 443, TCPOptions{}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if value, _ := findMeasurement(result, "outcome"); value != "reset" {
		t.Fatalf("outcome = %q, want reset", value)
	}
	if !strings.Contains(result.Diagnostic, "reset") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
}

func TestTCPCheckOtherError(t *testing.T) {
	dial := &fakeTCPDial{err: errors.New("no route to host")}
	check := testTCPCheck("1.1.1.1", 443, TCPOptions{}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if value, _ := findMeasurement(result, "outcome"); value != "error" {
		t.Fatalf("outcome = %q, want error", value)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestTCPCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dial := &fakeTCPDial{conn: nil}
	check := testTCPCheck("1.1.1.1", 443, TCPOptions{}, dial)
	result := check.Run(ctx)

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if dial.calls != 0 {
		t.Fatal("connect attempted after cancellation")
	}
}

func TestTCPCheckDefaultTimeout(t *testing.T) {
	check := newTCPCheck(domain.Anchor{ID: "test", Host: "example.com"}, 443, TCPOptions{})
	if check.opts.Timeout != 2*time.Second {
		t.Fatalf("timeout = %v, want 2s", check.opts.Timeout)
	}
}

func TestNewTCPChecks(t *testing.T) {
	checks := NewTCPChecks(TCPOptions{})
	anchors := domain.BaselineAnchors()
	if len(checks) != len(anchors)*2 {
		t.Fatalf("checks = %d, want %d", len(checks), len(anchors)*2)
	}
	seen := make(map[domain.CheckID]bool, len(checks))
	targets := make(map[string]bool, len(checks))
	for _, check := range checks {
		if seen[check.ID()] {
			t.Fatalf("duplicate check id %q", check.ID())
		}
		seen[check.ID()] = true
		typed, ok := check.(TCPCheck)
		if !ok {
			t.Fatalf("check %T is not a TCPCheck", check)
		}
		targets[net.JoinHostPort(typed.host, strconv.Itoa(typed.port))] = true
	}
	for _, want := range []string{"1.1.1.1:443", "1.1.1.1:53", "captive.apple.com:443", "example.com:53"} {
		if !targets[want] {
			t.Fatalf("missing target %q", want)
		}
	}
}

func TestTCPChecksIntegration(t *testing.T) {
	if os.Getenv("EGRESS_INTEGRATION") == "" {
		t.Skip("set EGRESS_INTEGRATION=1 to run the live TCP checks")
	}
	checks := NewTCPChecks(TCPOptions{Timeout: time.Second})
	for _, check := range checks {
		result := check.Run(context.Background())
		if err := result.Validate(); err != nil {
			t.Fatalf("%s: invalid result: %v", check.ID(), err)
		}
		t.Logf("%s: %s %q", check.ID(), result.Status, result.Diagnostic)
	}
}
