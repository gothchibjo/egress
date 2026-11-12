package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

type fakeEchoConn struct {
	rtts       []time.Duration
	errs       []error
	exchanges  int
	closed     bool
	onExchange func(exchanges int)
}

func (f *fakeEchoConn) Exchange(_ context.Context, _ net.IP, seq int) (time.Duration, error) {
	f.exchanges++
	if f.onExchange != nil {
		f.onExchange(f.exchanges)
	}
	if seq < len(f.errs) && f.errs[seq] != nil {
		return 0, f.errs[seq]
	}
	if seq < len(f.rtts) {
		return f.rtts[seq], nil
	}
	return 0, errors.New("no fake reply")
}

func (f *fakeEchoConn) Close() error {
	f.closed = true
	return nil
}

type fakeDial struct {
	conn     echoConn
	err      error
	calls    int
	family   platform.Family
	elevated bool
	timeout  time.Duration
}

func (f *fakeDial) dial(family platform.Family, elevated bool, timeout time.Duration) (echoConn, error) {
	f.calls++
	f.family = family
	f.elevated = elevated
	f.timeout = timeout
	return f.conn, f.err
}

func staticResolver(ip string) func(context.Context, string) (net.IP, error) {
	return func(context.Context, string) (net.IP, error) {
		return net.ParseIP(ip), nil
	}
}

func staticGateway(gateway string) func(platform.Family) (platform.DefaultRoute, bool, error) {
	return func(family platform.Family) (platform.DefaultRoute, bool, error) {
		if gateway == "" {
			return platform.DefaultRoute{}, false, nil
		}
		return platform.DefaultRoute{Family: family, Gateway: gateway, Interface: "en0"}, true, nil
	}
}

func testICMPCheck(id domain.CheckID, host string, opts ICMPOptions, dial *fakeDial) ICMPCheck {
	return ICMPCheck{
		id:        id,
		host:      host,
		opts:      opts.normalized(),
		dial:      dial.dial,
		resolve:   staticResolver("93.184.216.34"),
		gatewayFn: staticGateway("192.168.1.1"),
		now:       func() time.Time { return fixedTime },
	}
}

func TestICMPCheckPass(t *testing.T) {
	conn := &fakeEchoConn{rtts: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{Count: 3}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
	if result.Target != "example.com" {
		t.Fatalf("target = %q, want example.com", result.Target)
	}
	want := map[string]string{
		"address":          "93.184.216.34",
		"packets.sent":     "3",
		"packets.received": "3",
		"packet_loss":      "0%",
		"sample.size":      "3",
		"rtt.min":          "10ms",
		"rtt.avg":          "20ms",
		"rtt.median":       "20ms",
		"rtt.max":          "30ms",
		"jitter":           "10ms",
	}
	for name, value := range want {
		got, ok := findMeasurement(result, name)
		if !ok {
			t.Fatalf("measurement %q is missing", name)
		}
		if got != value {
			t.Fatalf("measurement %q = %q, want %q", name, got, value)
		}
	}
	if !conn.closed {
		t.Fatal("ICMP connection was not closed")
	}
	if dial.family != platform.FamilyIPv4 {
		t.Fatalf("socket family = %v, want ipv4", dial.family)
	}
}

func TestICMPCheckPartialLossWarns(t *testing.T) {
	conn := &fakeEchoConn{
		rtts: []time.Duration{10 * time.Millisecond},
		errs: []error{nil, context.DeadlineExceeded, context.DeadlineExceeded},
	}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{Count: 3}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if value, _ := findMeasurement(result, "packet_loss"); value != "67%" {
		t.Fatalf("packet_loss = %q, want 67%%", value)
	}
	if !strings.Contains(result.Diagnostic, "rate-limited") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestICMPCheckTotalLossFails(t *testing.T) {
	conn := &fakeEchoConn{errs: []error{context.DeadlineExceeded, context.DeadlineExceeded, context.DeadlineExceeded}}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{Count: 3}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if value, _ := findMeasurement(result, "packet_loss"); value != "100%" {
		t.Fatalf("packet_loss = %q, want 100%%", value)
	}
	if !strings.Contains(result.Diagnostic, "rate-limited") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestICMPCheckUnprivilegedDialFailureSkips(t *testing.T) {
	dial := &fakeDial{err: errors.New("operation not permitted")}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "elevation") || !strings.Contains(result.Diagnostic, "sudo") {
		t.Fatalf("diagnostic = %q, want an elevation hint", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "socket"); value != "unavailable" {
		t.Fatalf("socket measurement = %q", value)
	}
	if dial.elevated {
		t.Fatal("unprivileged check requested a raw socket")
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestICMPCheckElevatedUsesRaw(t *testing.T) {
	conn := &fakeEchoConn{rtts: []time.Duration{time.Millisecond}}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{Elevated: true, Count: 1}, dial)
	check.Run(context.Background())

	if !dial.elevated {
		t.Fatal("elevated check did not request a raw socket")
	}
}

func TestICMPCheckGateway(t *testing.T) {
	conn := &fakeEchoConn{rtts: []time.Duration{time.Millisecond}}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("gateway.reach", "default gateway", ICMPOptions{Count: 1}, dial)
	check.useGateway = true
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass", result.Status)
	}
	if result.Target != "192.168.1.1" {
		t.Fatalf("target = %q, want the gateway", result.Target)
	}
	if value, _ := findMeasurement(result, "address"); value != "192.168.1.1" {
		t.Fatalf("address = %q", value)
	}
	if value, _ := findMeasurement(result, "rtt.avg"); value == "" {
		t.Fatal("gateway result has no round-trip time")
	}
}

func TestICMPCheckGatewayUnreachable(t *testing.T) {
	conn := &fakeEchoConn{errs: []error{context.DeadlineExceeded, context.DeadlineExceeded}}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("gateway.reach", "default gateway", ICMPOptions{Count: 2}, dial)
	check.useGateway = true
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "no ICMP replies") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestICMPCheckGatewayMissingSkips(t *testing.T) {
	dial := &fakeDial{conn: &fakeEchoConn{}}
	check := testICMPCheck("gateway.reach", "default gateway", ICMPOptions{}, dial)
	check.useGateway = true
	check.gatewayFn = staticGateway("")
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if result.Diagnostic != "no default gateway" {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if dial.calls != 0 {
		t.Fatal("probe opened a socket without a gateway")
	}
}

func TestICMPCheckGatewayOnLinkSkips(t *testing.T) {
	dial := &fakeDial{conn: &fakeEchoConn{}}
	check := testICMPCheck("gateway.reach", "default gateway", ICMPOptions{}, dial)
	check.useGateway = true
	check.gatewayFn = func(family platform.Family) (platform.DefaultRoute, bool, error) {
		return platform.DefaultRoute{Family: family, Interface: "en0"}, true, nil
	}
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "no gateway address") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
}

func TestICMPCheckResolveFailure(t *testing.T) {
	dial := &fakeDial{conn: &fakeEchoConn{}}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{}, dial)
	check.resolve = func(context.Context, string) (net.IP, error) {
		return nil, errors.New("lookup failed")
	}
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Error, "lookup failed") {
		t.Fatalf("error = %q", result.Error)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestICMPCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dial := &fakeDial{conn: &fakeEchoConn{}}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{}, dial)
	result := check.Run(ctx)

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if dial.calls != 0 {
		t.Fatal("probe opened a socket after cancellation")
	}
}

func TestICMPCheckCancelledMidSample(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conn := &fakeEchoConn{
		rtts: []time.Duration{time.Millisecond},
		onExchange: func(exchanges int) {
			if exchanges == 1 {
				cancel()
			}
		},
	}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{Count: 3}, dial)
	result := check.Run(ctx)

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "cancelled") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if values, _ := findMeasurement(result, "packets.sent"); values != "1" {
		t.Fatalf("packets.sent = %q, want 1", values)
	}
}

func TestICMPCheckSingleResponse(t *testing.T) {
	conn := &fakeEchoConn{rtts: []time.Duration{15 * time.Millisecond}}
	dial := &fakeDial{conn: conn}
	check := testICMPCheck("icmp.anchor.example-com", "example.com", ICMPOptions{Count: 1}, dial)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass", result.Status)
	}
	for name, want := range map[string]string{
		"sample.size": "1",
		"rtt.median":  "15ms",
		"jitter":      "0s",
	} {
		if got, _ := findMeasurement(result, name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestLatencyStatistics(t *testing.T) {
	tests := []struct {
		name    string
		ordered []time.Duration
		sorted  []time.Duration
		median  time.Duration
		jitter  time.Duration
	}{
		{
			name:    "odd sample",
			ordered: []time.Duration{30 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond},
			sorted:  []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond},
			median:  20 * time.Millisecond,
			jitter:  15 * time.Millisecond,
		},
		{
			name:    "even sample",
			ordered: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond},
			sorted:  []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond},
			median:  25 * time.Millisecond,
			jitter:  10 * time.Millisecond,
		},
		{
			name:    "single response",
			ordered: []time.Duration{15 * time.Millisecond},
			sorted:  []time.Duration{15 * time.Millisecond},
			median:  15 * time.Millisecond,
			jitter:  0,
		},
		{
			name:    "two responses",
			ordered: []time.Duration{10 * time.Millisecond, 30 * time.Millisecond},
			sorted:  []time.Duration{10 * time.Millisecond, 30 * time.Millisecond},
			median:  20 * time.Millisecond,
			jitter:  20 * time.Millisecond,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := medianDuration(test.sorted); got != test.median {
				t.Fatalf("median = %v, want %v", got, test.median)
			}
			if got := meanJitter(test.ordered); got != test.jitter {
				t.Fatalf("jitter = %v, want %v", got, test.jitter)
			}
		})
	}
}

func TestICMPOptionsCapSample(t *testing.T) {
	if got := (ICMPOptions{Count: 100}).normalized().Count; got != MaxICMPSamples {
		t.Fatalf("count = %d, want %d", got, MaxICMPSamples)
	}
	if got := (ICMPOptions{}).normalized().Count; got != 3 {
		t.Fatalf("default count = %d, want 3", got)
	}
}

func TestNewICMPChecks(t *testing.T) {
	checks := NewICMPChecks(ICMPOptions{})
	if len(checks) != len(domain.BaselineAnchors())+1 {
		t.Fatalf("checks = %d, want %d", len(checks), len(domain.BaselineAnchors())+1)
	}
	if checks[0].ID() != "gateway.reach" {
		t.Fatalf("first check = %q, want gateway.reach", checks[0].ID())
	}
	seen := make(map[domain.CheckID]bool, len(checks))
	for _, check := range checks {
		if seen[check.ID()] {
			t.Fatalf("duplicate check id %q", check.ID())
		}
		seen[check.ID()] = true
	}
	for _, anchor := range domain.BaselineAnchors() {
		id := domain.CheckID("icmp.anchor." + anchor.ID)
		if !seen[id] {
			t.Fatalf("missing check %q", id)
		}
	}
}

func TestICMPChecksIntegration(t *testing.T) {
	if os.Getenv("EGRESS_INTEGRATION") == "" {
		t.Skip("set EGRESS_INTEGRATION=1 to run the live ICMP checks")
	}
	checks := NewICMPChecks(ICMPOptions{Count: 2, Timeout: time.Second})
	for _, check := range checks {
		result := check.Run(context.Background())
		if err := result.Validate(); err != nil {
			t.Fatalf("%s: invalid result: %v", check.ID(), err)
		}
		fmt.Printf("%s: %s %q\n", check.ID(), result.Status, result.Diagnostic)
	}
}

func TestICMPCheckUnsupportedGateway(t *testing.T) {
	dial := &fakeDial{conn: &fakeEchoConn{}}
	check := testICMPCheck("gateway.reach", "default gateway", ICMPOptions{}, dial)
	check.useGateway = true
	check.gatewayFn = func(platform.Family) (platform.DefaultRoute, bool, error) {
		return platform.DefaultRoute{}, false, platform.ErrUnsupported
	}
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "not supported") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if dial.calls != 0 {
		t.Fatal("probe opened a socket on an unsupported platform")
	}
}
