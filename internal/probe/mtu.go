package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

const (
	// DefaultMTUHost is the destination used for the path-MTU probe.
	DefaultMTUHost = "example.com"
	// MinMTU and MaxMTU bound the documented probe range.
	MinMTU = 576
	MaxMTU = 1500
)

var defaultMTUSizes = []int{576, 1280, 1400, 1500}

var errRawSocketsRequired = errors.New("raw ICMP sockets require elevated mode")

// MTUOptions configures the path-MTU probe.
type MTUOptions struct {
	// Elevated allows the raw DF socket. The probe never escalates on its own.
	Elevated bool
	// Sizes are the packet sizes to test; they are clamped to the documented
	// range.
	Sizes []int
	// Timeout bounds one size probe.
	Timeout time.Duration
	// Host is the destination hostname.
	Host string
}

func (o MTUOptions) normalized() MTUOptions {
	if o.Host == "" {
		o.Host = DefaultMTUHost
	}
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Second
	}
	sizes := make([]int, 0, len(o.Sizes))
	for _, size := range o.Sizes {
		if size >= MinMTU && size <= MaxMTU {
			sizes = append(sizes, size)
		}
	}
	if len(sizes) == 0 {
		sizes = append(sizes, defaultMTUSizes...)
	}
	sort.Ints(sizes)
	o.Sizes = sizes
	return o
}

// MTUProber sends one DF packet of the requested size.
type MTUProber interface {
	Probe(ctx context.Context, target net.IP, mtu int) error
	Close() error
}

type mtuProberFactory func(elevated bool, timeout time.Duration) (MTUProber, error)

// MTUCheck reports tested packet sizes and any suspected PMTU blackhole.
type MTUCheck struct {
	id      domain.CheckID
	host    string
	opts    MTUOptions
	factory mtuProberFactory
	resolve func(context.Context, string) (net.IP, error)
	now     func() time.Time
}

// NewMTUCheck returns the path-MTU probe with the documented sizes.
func NewMTUCheck(opts MTUOptions) MTUCheck {
	opts = opts.normalized()
	return MTUCheck{
		id:      "mtu.pmtu",
		host:    opts.Host,
		opts:    opts,
		factory: defaultMTUProberFactory,
		resolve: resolveOne,
		now:     time.Now,
	}
}

// ID returns the stable check id used by the dashboard and JSON output.
func (c MTUCheck) ID() domain.CheckID {
	return c.id
}

// Run probes the documented sizes in ascending order and stops at the first
// failure.
func (c MTUCheck) Run(ctx context.Context) domain.Result {
	result := domain.Result{
		CheckID:   c.id,
		Target:    c.host,
		Method:    "df icmp probe",
		Timestamp: c.now(),
	}
	if err := ctx.Err(); err != nil {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "MTU probe cancelled before it started"
		return result
	}
	if !c.opts.Elevated {
		result.Status = domain.StatusSkipped
		result.Measurements = []domain.Measurement{{Name: "socket", Value: "unavailable"}}
		result.Diagnostic = "raw ICMP DF probe requires elevated mode; run with elevation (sudo)"
		return result
	}

	target, err := c.resolve(ctx, c.host)
	if err != nil {
		result.Status = domain.StatusFail
		result.Error = err.Error()
		result.Diagnostic = "destination could not be resolved"
		return result
	}
	result.Measurements = []domain.Measurement{{Name: "address", Value: target.String()}}

	prober, err := c.factory(c.opts.Elevated, c.opts.Timeout)
	if err != nil {
		result.Status = domain.StatusSkipped
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "socket", Value: "unavailable"})
		result.Diagnostic = "raw DF socket unavailable: " + err.Error()
		return result
	}
	defer prober.Close()

	sizes := c.opts.Sizes
	baseline := sizes[0]
	if err := prober.Probe(ctx, target, baseline); err != nil {
		result.Status = domain.StatusSkipped
		result.Measurements = append(result.Measurements,
			domain.Measurement{Name: "mtu.tested", Value: strconv.Itoa(baseline)},
		)
		result.Diagnostic = fmt.Sprintf("path unreachable at MTU %d: %v", baseline, err)
		return result
	}

	tested := []int{baseline}
	largest := baseline
	failed := 0
	for _, size := range sizes[1:] {
		if ctx.Err() != nil {
			break
		}
		tested = append(tested, size)
		if err := prober.Probe(ctx, target, size); err != nil {
			failed = size
			break
		}
		largest = size
	}

	result.Measurements = append(result.Measurements,
		domain.Measurement{Name: "mtu.tested", Value: joinSizes(tested)},
		domain.Measurement{Name: "mtu.max", Value: strconv.Itoa(largest)},
	)
	if failed > 0 {
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "mtu.failed", Value: strconv.Itoa(failed)})
		result.Status = domain.StatusWarn
		result.Diagnostic = fmt.Sprintf("suspected PMTU blackhole at %d bytes", failed)
		return result
	}
	result.Measurements = append(result.Measurements, domain.Measurement{Name: "mtu.failed", Value: "none"})
	if ctx.Err() != nil {
		result.Status = domain.StatusWarn
		result.Diagnostic = "probe cancelled before completion"
		return result
	}
	result.Status = domain.StatusPass
	return result
}

func joinSizes(sizes []int) string {
	values := make([]string, 0, len(sizes))
	for _, size := range sizes {
		values = append(values, strconv.Itoa(size))
	}
	return strings.Join(values, ", ")
}
