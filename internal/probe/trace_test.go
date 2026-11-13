package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

func quotedIPv4(protocol byte, port int) []byte {
	buffer := make([]byte, 28)
	buffer[0] = 0x45 // IPv4, IHL 5
	buffer[9] = protocol
	binary.BigEndian.PutUint16(buffer[22:24], uint16(port))
	return buffer
}

func quotedIPv6(protocol byte, port int) []byte {
	buffer := make([]byte, 48)
	buffer[0] = 0x60 // IPv6
	buffer[6] = protocol
	binary.BigEndian.PutUint16(buffer[42:44], uint16(port))
	return buffer
}

func measurementValue(values []domain.Measurement, name string) string {
	for _, value := range values {
		if value.Name == name {
			return value.Value
		}
	}
	return ""
}

func TestQuotedUDPPort(t *testing.T) {
	if port, ok := quotedUDPPort(quotedIPv4(17, 33444), 1); !ok || port != 33444 {
		t.Fatalf("IPv4 quoted port = %d, ok = %v", port, ok)
	}
	if _, ok := quotedUDPPort(quotedIPv4(1, 33444), 1); ok {
		t.Fatal("accepted a quoted ICMP packet as UDP")
	}
	if port, ok := quotedUDPPort(quotedIPv6(17, 33455), 58); !ok || port != 33455 {
		t.Fatalf("IPv6 quoted port = %d, ok = %v", port, ok)
	}
	if _, ok := quotedUDPPort([]byte{0x45}, 1); ok {
		t.Fatal("accepted a truncated quoted packet")
	}
}

func TestRTTStats(t *testing.T) {
	min, median, max := rttStats([]int64{40, 10, 30, 20})
	if min != 10 || median != 30 || max != 40 {
		t.Fatalf("stats = %d/%d/%d", min, median, max)
	}
	if min, median, max := rttStats(nil); min != 0 || median != 0 || max != 0 {
		t.Fatalf("empty stats = %d/%d/%d", min, median, max)
	}
}

func TestNewTracerSelectsTheMethod(t *testing.T) {
	tracer, info := NewTracer(false)
	if tracer.Method() != "udp" || info.Mode != domain.ModeLimited {
		t.Fatalf("limited tracer = %q / %+v", tracer.Method(), info)
	}
	if info.Reason == "" {
		t.Fatal("limited tracer has no reason")
	}
}

func TestNewTracerElevatedFallsBackWithoutRaw(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: the raw socket is available")
	}
	tracer, info := NewTracer(true)
	if tracer.Method() != "udp" {
		t.Fatalf("tracer = %q, want the UDP fallback", tracer.Method())
	}
	if info.Mode != domain.ModeLimited {
		t.Fatalf("mode = %q, want limited after the fallback", info.Mode)
	}
	if info.Reason == "" {
		t.Fatal("fallback has no reason")
	}
}

func TestICMPTracerRequiresPrivileges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: the raw socket is available")
	}
	if _, err := newICMPTracer(); err == nil {
		t.Fatal("raw ICMP tracer was created without privileges")
	}
}

type fakeTracer struct {
	hops []domain.Hop
	err  error
}

func (f fakeTracer) Method() string { return "fake" }

func (f fakeTracer) Trace(context.Context, string) ([]domain.Hop, error) {
	return f.hops, f.err
}

func TestTraceStepReportsHops(t *testing.T) {
	step := TraceStep(fakeTracer{hops: []domain.Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3},
		{TTL: 2, Address: "198.51.100.1", Sent: 3, Received: 3},
	}})(context.Background(), LadderTarget{Host: "example.net"})

	if step.Status != domain.StatusPass {
		t.Fatalf("status = %q", step.Status)
	}
	if err := step.Validate(); err != nil {
		t.Fatalf("step is invalid: %v", err)
	}
	for _, want := range []string{"method", "hops", "responding_hops", "last_hop"} {
		if measurementValue(step.Measurements, want) == "" {
			t.Fatalf("missing measurement %q", want)
		}
	}
}

func TestTraceStepKeepsPartialPath(t *testing.T) {
	step := TraceStep(fakeTracer{
		hops: []domain.Hop{{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3}},
		err:  context.DeadlineExceeded,
	})(context.Background(), LadderTarget{Host: "example.net"})

	if step.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", step.Status)
	}
	if !strings.Contains(step.Diagnostic, "truncated") {
		t.Fatalf("diagnostic = %q", step.Diagnostic)
	}
	if err := step.Validate(); err != nil {
		t.Fatalf("step is invalid: %v", err)
	}
}

func TestTraceStepWithoutResponsesWarns(t *testing.T) {
	step := TraceStep(fakeTracer{hops: []domain.Hop{{TTL: 1, Sent: 3}}})(context.Background(), LadderTarget{Host: "example.net"})
	if step.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", step.Status)
	}
	if err := step.Validate(); err != nil {
		t.Fatalf("step is invalid: %v", err)
	}
}

func TestTraceStepReportsFailure(t *testing.T) {
	step := TraceStep(fakeTracer{err: errors.New("network unreachable")})(context.Background(), LadderTarget{Host: "example.net"})
	if step.Status != domain.StatusFail || step.Error == "" {
		t.Fatalf("step = %+v", step)
	}
	if err := step.Validate(); err != nil {
		t.Fatalf("step is invalid: %v", err)
	}
}

func TestUDPTracerIntegration(t *testing.T) {
	if os.Getenv("EGRESS_INTEGRATION") == "" {
		t.Skip("set EGRESS_INTEGRATION=1 to run the live traceroute")
	}

	tracer := UDPTracer{MaxHops: 15, Window: 750 * time.Millisecond}
	hops, err := tracer.Trace(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if len(hops) == 0 {
		t.Fatal("no hops returned")
	}
	reached := false
	responded := 0
	for _, hop := range hops {
		if hop.Sent != defaultHopProbes {
			t.Fatalf("hop %d sent %d probes, want %d", hop.TTL, hop.Sent, defaultHopProbes)
		}
		if hop.Address != "" {
			responded++
		}
		if hop.Address == "1.1.1.1" {
			reached = true
		}
	}
	if responded == 0 {
		t.Fatal("no hop answered")
	}
	if !reached {
		t.Fatalf("the target was not reached within %d hops", len(hops))
	}
}
