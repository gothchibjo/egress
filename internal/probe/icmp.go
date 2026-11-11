package probe

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// MaxICMPSamples caps the bounded echo sample per destination.
const MaxICMPSamples = 10

// ICMPOptions configures the ICMP reachability checks.
type ICMPOptions struct {
	// Elevated allows raw sockets. It must come from an explicitly elevated
	// session; the probe never escalates on its own.
	Elevated bool
	// Count is the bounded number of echo requests per destination.
	Count int
	// Timeout bounds one echo exchange.
	Timeout time.Duration
}

func (o ICMPOptions) normalized() ICMPOptions {
	if o.Count <= 0 {
		o.Count = 3
	}
	if o.Count > MaxICMPSamples {
		o.Count = MaxICMPSamples
	}
	if o.Timeout <= 0 {
		o.Timeout = time.Second
	}
	return o
}

// echoConn is the minimal ICMP transport used by the checks.
type echoConn interface {
	Exchange(ctx context.Context, target net.IP, seq int) (time.Duration, error)
	Close() error
}

type icmpDialer func(family platform.Family, elevated bool, timeout time.Duration) (echoConn, error)

// ICMPCheck probes one destination with a bounded echo sample.
type ICMPCheck struct {
	id         domain.CheckID
	host       string
	useGateway bool
	opts       ICMPOptions

	dial      icmpDialer
	resolve   func(context.Context, string) (net.IP, error)
	gatewayFn func(platform.Family) (platform.DefaultRoute, bool, error)
	now       func() time.Time
}

// NewICMPChecks builds one check for the gateway and one per curated anchor.
func NewICMPChecks(opts ICMPOptions) []app.Check {
	opts = opts.normalized()
	checks := make([]app.Check, 0, len(domain.BaselineAnchors())+1)
	checks = append(checks, newGatewayCheck(opts))
	for _, anchor := range domain.BaselineAnchors() {
		checks = append(checks, newAnchorCheck(anchor, opts))
	}
	return checks
}

func newGatewayCheck(opts ICMPOptions) ICMPCheck {
	return ICMPCheck{
		id:         "gateway.reach",
		host:       "default gateway",
		useGateway: true,
		opts:       opts,
		dial:       dialICMP,
		resolve:    resolveOne,
		gatewayFn:  platform.ReadDefaultRoute,
		now:        time.Now,
	}
}

func newAnchorCheck(anchor domain.Anchor, opts ICMPOptions) ICMPCheck {
	return ICMPCheck{
		id:        domain.CheckID("icmp.anchor." + anchor.ID),
		host:      anchor.Host,
		opts:      opts,
		dial:      dialICMP,
		resolve:   resolveOne,
		now:       time.Now,
		gatewayFn: platform.ReadDefaultRoute,
	}
}

// ID returns the stable check id used by the dashboard and JSON output.
func (c ICMPCheck) ID() domain.CheckID {
	return c.id
}

// Run sends the bounded echo sample and reports reachability, latency and loss.
func (c ICMPCheck) Run(ctx context.Context) domain.Result {
	result := domain.Result{
		CheckID:   c.id,
		Target:    c.host,
		Method:    "icmp echo",
		Timestamp: c.now(),
	}
	if err := ctx.Err(); err != nil {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "ICMP sample cancelled before it started"
		return result
	}

	target, skipped, err := c.target(ctx)
	if skipped != "" {
		result.Status = domain.StatusSkipped
		result.Diagnostic = skipped
		result.Measurements = []domain.Measurement{{Name: "socket", Value: "unavailable"}}
		return result
	}
	if err != nil {
		result.Status = domain.StatusFail
		result.Error = err.Error()
		result.Diagnostic = "destination could not be resolved"
		return result
	}

	family := platform.FamilyIPv4
	if target.To4() == nil {
		family = platform.FamilyIPv6
	}
	if c.useGateway {
		result.Target = target.String()
	}
	result.Measurements = append(result.Measurements, domain.Measurement{Name: "address", Value: target.String()})

	conn, dialErr := c.dial(family, c.opts.Elevated, c.opts.Timeout)
	if dialErr != nil {
		result.Status = domain.StatusSkipped
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "socket", Value: "unavailable"})
		result.Diagnostic = icmpSocketHint(dialErr, family, c.opts.Elevated)
		return result
	}
	defer conn.Close()

	sent := 0
	rtts := make([]time.Duration, 0, c.opts.Count)
	var lastErr error
	for sequence := 0; sequence < c.opts.Count; sequence++ {
		if ctx.Err() != nil {
			break
		}
		rtt, err := conn.Exchange(ctx, target, sequence)
		sent++
		if err != nil {
			lastErr = err
			continue
		}
		rtts = append(rtts, rtt)
	}

	if sent == 0 {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "ICMP sample cancelled before the first reply"
		return result
	}

	received := len(rtts)
	loss := float64(sent-received) / float64(sent) * 100
	result.Measurements = append(result.Measurements,
		domain.Measurement{Name: "packets.sent", Value: fmt.Sprintf("%d", sent)},
		domain.Measurement{Name: "packets.received", Value: fmt.Sprintf("%d", received)},
		domain.Measurement{Name: "packet_loss", Value: fmt.Sprintf("%.0f%%", loss)},
	)
	if received > 0 {
		ordered := append([]time.Duration(nil), rtts...)
		sorted := append([]time.Duration(nil), rtts...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		var total time.Duration
		for _, rtt := range sorted {
			total += rtt
		}
		result.Measurements = append(result.Measurements,
			domain.Measurement{Name: "sample.size", Value: strconv.Itoa(received)},
			domain.Measurement{Name: "rtt.min", Value: formatRTT(sorted[0])},
			domain.Measurement{Name: "rtt.avg", Value: formatRTT(total / time.Duration(received))},
			domain.Measurement{Name: "rtt.median", Value: formatRTT(medianDuration(sorted))},
			domain.Measurement{Name: "rtt.max", Value: formatRTT(sorted[received-1])},
			domain.Measurement{Name: "jitter", Value: formatRTT(meanJitter(ordered))},
		)
	}

	switch {
	case received == 0:
		result.Status = domain.StatusFail
		result.Diagnostic = "no ICMP replies; ICMP may be rate-limited or blocked"
		if lastErr != nil {
			result.Diagnostic += " (" + lastErr.Error() + ")"
		}
	case received < sent:
		result.Status = domain.StatusWarn
		result.Diagnostic = "partial ICMP loss; ICMP may be rate-limited or blocked"
	default:
		result.Status = domain.StatusPass
	}
	if ctx.Err() != nil {
		result.Status = domain.StatusWarn
		if result.Diagnostic != "" {
			result.Diagnostic += "; "
		}
		result.Diagnostic += "sample cancelled before completion"
	}
	return result
}

// target resolves the destination or reports a skip reason.
func (c ICMPCheck) target(ctx context.Context) (net.IP, string, error) {
	if !c.useGateway {
		ip, err := c.resolve(ctx, c.host)
		return ip, "", err
	}

	for _, family := range []platform.Family{platform.FamilyIPv4, platform.FamilyIPv6} {
		route, found, err := c.gatewayFn(family)
		if err != nil {
			return nil, "", fmt.Errorf("read %s default route: %w", family, err)
		}
		if !found {
			continue
		}
		if route.Gateway == "" {
			return nil, "default route has no gateway address", nil
		}
		ip := net.ParseIP(route.Gateway)
		if ip == nil {
			return nil, "", fmt.Errorf("invalid gateway address %q", route.Gateway)
		}
		return ip, "", nil
	}
	return nil, "no default gateway", nil
}

func resolveOne(ctx context.Context, host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no address for %s", host)
	}
	for _, address := range addresses {
		if address.IP.To4() != nil {
			return address.IP, nil
		}
	}
	return addresses[0].IP, nil
}

func formatRTT(rtt time.Duration) string {
	return rtt.Round(100 * time.Microsecond).String()
}

// medianDuration returns the middle value of a sorted sample.
func medianDuration(sorted []time.Duration) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// meanJitter returns the mean absolute difference between consecutive
// round-trip times in sample order.
func meanJitter(ordered []time.Duration) time.Duration {
	if len(ordered) < 2 {
		return 0
	}
	var total time.Duration
	for i := 1; i < len(ordered); i++ {
		difference := ordered[i] - ordered[i-1]
		if difference < 0 {
			difference = -difference
		}
		total += difference
	}
	return total / time.Duration(len(ordered)-1)
}

func icmpSocketHint(err error, family platform.Family, elevated bool) string {
	kind := "unprivileged datagram"
	if elevated {
		kind = "raw"
	}
	return fmt.Sprintf("ICMP %s socket unavailable: %v; run with elevation (sudo) to enable raw ICMP", kind, err)
}
