package probe

import (
	"context"
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

// TCPOptions configures the TCP reachability checks.
type TCPOptions struct {
	// Timeout bounds one connect attempt.
	Timeout time.Duration
	// Ports are the destination ports probed on every anchor.
	Ports []int
}

func (o TCPOptions) normalized() TCPOptions {
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Second
	}
	if len(o.Ports) == 0 {
		o.Ports = []int{443, 53}
	}
	return o
}

type tcpDialer func(ctx context.Context, network, address string) (net.Conn, error)

// TCPCheck attempts one TCP connection to one anchor and port.
type TCPCheck struct {
	id   domain.CheckID
	host string
	port int
	opts TCPOptions
	dial tcpDialer
	now  func() time.Time
}

// NewTCPChecks builds one check per anchor and documented port.
func NewTCPChecks(opts TCPOptions) []app.Check {
	opts = opts.normalized()
	anchors := domain.BaselineAnchors()
	checks := make([]app.Check, 0, len(anchors)*len(opts.Ports))
	for _, anchor := range anchors {
		for _, port := range opts.Ports {
			checks = append(checks, newTCPCheck(anchor, port, opts))
		}
	}
	return checks
}

func newTCPCheck(anchor domain.Anchor, port int, opts TCPOptions) TCPCheck {
	opts = opts.normalized()
	return TCPCheck{
		id:   domain.CheckID("tcp.anchor." + anchor.ID + "." + strconv.Itoa(port)),
		host: anchor.Host,
		port: port,
		opts: opts,
		dial: defaultTCPDialer(opts.Timeout),
		now:  time.Now,
	}
}

func defaultTCPDialer(timeout time.Duration) tcpDialer {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: timeout}
		return dialer.DialContext(ctx, network, address)
	}
}

// ID returns the stable check id used by the dashboard and JSON output.
func (c TCPCheck) ID() domain.CheckID {
	return c.id
}

// Run attempts the connection and reports the outcome class and latency.
func (c TCPCheck) Run(ctx context.Context) domain.Result {
	address := net.JoinHostPort(c.host, strconv.Itoa(c.port))
	result := domain.Result{
		CheckID:   c.id,
		Target:    address,
		Method:    "tcp connect",
		Timestamp: c.now(),
	}
	if err := ctx.Err(); err != nil {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "connect cancelled before it started"
		return result
	}

	start := time.Now()
	conn, err := c.dial(ctx, "tcp", address)
	latency := time.Since(start)
	if err != nil {
		outcome, passive := classifyTCPError(err)
		result.Measurements = []domain.Measurement{{Name: "outcome", Value: outcome}}
		result.Error = err.Error()
		switch outcome {
		case "refused":
			result.Status = domain.StatusWarn
			result.Diagnostic = "connection actively refused"
		case "reset":
			result.Status = domain.StatusWarn
			result.Diagnostic = "connection reset by peer"
		case "timeout":
			result.Status = domain.StatusFail
			if passive {
				result.Diagnostic = "connection timed out (passive)"
			} else {
				result.Diagnostic = "connection timed out"
			}
		default:
			result.Status = domain.StatusFail
			result.Diagnostic = "connection failed"
		}
		return result
	}
	conn.Close()

	result.Status = domain.StatusPass
	result.Measurements = []domain.Measurement{
		{Name: "outcome", Value: "connected"},
		{Name: "latency", Value: formatRTT(latency)},
	}
	return result
}

func classifyTCPError(err error) (string, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "refused", false
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return "reset", false
	}
	return "error", false
}
