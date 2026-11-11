package probe

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

type fakeMTUProber struct {
	failures map[int]error
	calls    []int
	closed   bool
	onProbe  func(mtu int)
}

func (f *fakeMTUProber) Probe(_ context.Context, _ net.IP, mtu int) error {
	f.calls = append(f.calls, mtu)
	if f.onProbe != nil {
		f.onProbe(mtu)
	}
	if err, ok := f.failures[mtu]; ok {
		return err
	}
	return nil
}

func (f *fakeMTUProber) Close() error {
	f.closed = true
	return nil
}

type fakeMTUFactory struct {
	prober *fakeMTUProber
	err    error
	calls  int
}

func (f *fakeMTUFactory) create(bool, time.Duration) (MTUProber, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.prober, nil
}

func testMTUCheck(opts MTUOptions, factory *fakeMTUFactory) MTUCheck {
	opts = opts.normalized()
	return MTUCheck{
		id:      "mtu.pmtu",
		host:    opts.Host,
		opts:    opts,
		factory: factory.create,
		resolve: staticResolver("93.184.216.34"),
		now:     func() time.Time { return fixedTime },
	}
}

func TestMTUCheckCleanPath(t *testing.T) {
	prober := &fakeMTUProber{}
	factory := &fakeMTUFactory{prober: prober}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
	if len(prober.calls) != 4 {
		t.Fatalf("probe calls = %v, want 4 sizes", prober.calls)
	}
	if !prober.closed {
		t.Fatal("prober was not closed")
	}
	want := map[string]string{
		"mtu.tested": "576, 1280, 1400, 1500",
		"mtu.max":    "1500",
		"mtu.failed": "none",
	}
	for name, value := range want {
		if got, _ := findMeasurement(result, name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestMTUCheckBlackhole(t *testing.T) {
	prober := &fakeMTUProber{failures: map[int]error{1400: errPacketTooBig}}
	factory := &fakeMTUFactory{prober: prober}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "PMTU blackhole") || !strings.Contains(result.Diagnostic, "1400") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if len(prober.calls) != 3 {
		t.Fatalf("probe calls = %v, want to stop at the failing size", prober.calls)
	}
	want := map[string]string{
		"mtu.tested": "576, 1280, 1400",
		"mtu.max":    "1280",
		"mtu.failed": "1400",
	}
	for name, value := range want {
		if got, _ := findMeasurement(result, name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestMTUCheckUnreachablePathSkips(t *testing.T) {
	prober := &fakeMTUProber{failures: map[int]error{576: timeoutError{}}}
	factory := &fakeMTUFactory{prober: prober}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "path unreachable") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "mtu.tested"); value != "576" {
		t.Fatalf("mtu.tested = %q", value)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestMTUCheckUnprivilegedSkips(t *testing.T) {
	prober := &fakeMTUProber{}
	factory := &fakeMTUFactory{prober: prober}
	check := testMTUCheck(MTUOptions{Elevated: false}, factory)
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "elevation") || !strings.Contains(result.Diagnostic, "sudo") {
		t.Fatalf("diagnostic = %q, want an elevation hint", result.Diagnostic)
	}
	if factory.calls != 0 {
		t.Fatal("unprivileged check opened a prober")
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestMTUCheckProberUnavailableSkips(t *testing.T) {
	factory := &fakeMTUFactory{err: errors.New("operation not permitted")}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "raw DF socket unavailable") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
}

func TestMTUCheckResolveFailure(t *testing.T) {
	factory := &fakeMTUFactory{prober: &fakeMTUProber{}}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
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

func TestMTUCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	factory := &fakeMTUFactory{prober: &fakeMTUProber{}}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
	result := check.Run(ctx)

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if factory.calls != 0 {
		t.Fatal("probe started after cancellation")
	}
}

func TestMTUCheckCancelledMidProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prober := &fakeMTUProber{onProbe: func(mtu int) {
		if mtu == 576 {
			cancel()
		}
	}}
	factory := &fakeMTUFactory{prober: prober}
	check := testMTUCheck(MTUOptions{Elevated: true}, factory)
	result := check.Run(ctx)

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "cancelled") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if len(prober.calls) != 1 {
		t.Fatalf("probe calls = %v, want only the baseline", prober.calls)
	}
}

func TestMTUOptionsClampSizes(t *testing.T) {
	opts := MTUOptions{Sizes: []int{100, 1400, 9000, 600}}.normalized()
	want := []int{600, 1400}
	if len(opts.Sizes) != len(want) {
		t.Fatalf("sizes = %v, want %v", opts.Sizes, want)
	}
	for i, size := range want {
		if opts.Sizes[i] != size {
			t.Fatalf("sizes = %v, want %v", opts.Sizes, want)
		}
	}

	if defaulted := (MTUOptions{Sizes: []int{100, 9000}}).normalized(); len(defaulted.Sizes) != len(defaultMTUSizes) {
		t.Fatalf("out-of-range sizes were not replaced: %v", defaulted.Sizes)
	}
}

func TestDefaultMTUProberFactoryRequiresElevation(t *testing.T) {
	if _, err := defaultMTUProberFactory(false, time.Second); !errors.Is(err, errRawSocketsRequired) {
		t.Fatalf("error = %v, want %v", err, errRawSocketsRequired)
	}
}
