package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

const (
	defaultLadderTimeout = 2 * time.Second
	traceStepBudget      = 20 * time.Second
	decoySNI             = "example.org"
)

// LadderTarget is the probe destination type, shared with the session layer.
type LadderTarget = domain.ProbeTarget

// TargetFromPreset builds a ladder target from a catalog preset.
func TargetFromPreset(preset domain.Preset) domain.ProbeTarget {
	return domain.ProbeTarget{
		ID:        preset.ID,
		Label:     preset.DisplayName,
		Host:      preset.Host,
		Ports:     append([]int(nil), preset.Ports...),
		Protocols: append([]domain.PresetProtocol(nil), preset.Protocols...),
	}
}

// TargetFromCustom builds a ladder target from a parsed custom target.
func TargetFromCustom(target domain.Target) domain.ProbeTarget {
	ports := []int{443}
	if target.Port > 0 {
		ports = []int{target.Port}
	}
	protocols := []domain.PresetProtocol{domain.PresetICMP, domain.PresetTCP, domain.PresetTLS, domain.PresetHTTP}
	if target.Protocol != "" {
		protocols = []domain.PresetProtocol{target.Protocol}
	}
	label := target.String()
	if label == "" {
		label = target.Host
	}
	return domain.ProbeTarget{ID: label, Label: label, Host: target.Host, Ports: ports, Protocols: protocols}
}

// Ladder runs the independent probe steps for one target.
type Ladder struct {
	tracer     Tracer
	tracerInfo TracerInfo
	timeout    time.Duration
	now        func() time.Time
	resolve    func(ctx context.Context, host string) ([]string, error)
	doh        DoHResolver
	dial       tcpDialer
	http       HTTPDoer
	echo       func(ctx context.Context, ip net.IP) (time.Duration, error)
	handshake  func(ctx context.Context, conn net.Conn, serverName string) error
	trace      TraceFunc
}

// TraceFunc produces the traceroute step. It arrives with REQ-005; until then
// the ladder reports the step as skipped.
type TraceFunc func(ctx context.Context, target LadderTarget) domain.StepResult

// NewLadder wires the real clients for one session.
func NewLadder(elevated bool) Ladder {
	timeout := defaultLadderTimeout
	tracer, tracerInfo := NewTracer(elevated)
	ladder := Ladder{
		tracer:     tracer,
		tracerInfo: tracerInfo,
		timeout:    timeout,
		now:        time.Now,
		resolve:    lookupAddresses,
		doh:        cloudflareDoH{client: &http.Client{Timeout: 3 * time.Second}, endpoint: DefaultDoHEndpoint},
		dial:       defaultTCPDialer(timeout),
		http:       &http.Client{Timeout: timeout, CheckRedirect: noRedirect},
		echo: func(ctx context.Context, ip net.IP) (time.Duration, error) {
			return echoOnce(ctx, ip, elevated, timeout)
		},
		handshake: tlsHandshake,
	}
	ladder.trace = TraceStep(tracer)
	return ladder
}

// Tracer returns the tracing method selected for this ladder.
func (l Ladder) Tracer() Tracer {
	return l.tracer
}

// TracerInfo reports the tracing method and session mode selected for this
// ladder.
func (l Ladder) TracerInfo() TracerInfo {
	return l.tracerInfo
}

// TraceStep adapts a tracer into the ladder's trace step. It runs under its own
// budget because a path discovery needs longer than one probe.
func TraceStep(tracer Tracer) TraceFunc {
	return func(ctx context.Context, target LadderTarget) domain.StepResult {
		traceCtx, cancel := context.WithTimeout(ctx, traceStepBudget)
		defer cancel()

		started := time.Now()
		hops, err := tracer.Trace(traceCtx, target.Host)
		step := domain.StepResult{
			ID:           "trace",
			Kind:         domain.StepTrace,
			Target:       target.Host,
			StartedAt:    started,
			DurationMS:   time.Since(started).Milliseconds(),
			Measurements: []domain.Measurement{},
		}
		if err != nil && len(hops) == 0 {
			step.Status = domain.StatusFail
			step.Diagnostic = "path discovery failed"
			step.Error = err.Error()
			return step
		}

		responding, last := 0, ""
		for _, hop := range hops {
			if hop.Address != "" {
				responding++
				last = hop.Address
			}
		}
		step.Measurements = append(step.Measurements,
			domain.Measurement{Name: "method", Value: tracer.Method()},
			domain.Measurement{Name: "hops", Value: strconv.Itoa(len(hops))},
			domain.Measurement{Name: "responding_hops", Value: strconv.Itoa(responding)},
		)
		if last != "" {
			step.Measurements = append(step.Measurements, domain.Measurement{Name: "last_hop", Value: last})
		}
		step.Hops = append([]domain.Hop(nil), hops...)
		step.Status = domain.StatusPass
		switch {
		case err != nil:
			step.Status = domain.StatusWarn
			step.Diagnostic = "path discovery truncated: " + err.Error()
		case responding == 0:
			step.Status = domain.StatusWarn
			step.Diagnostic = "no hop answered; ICMP may be filtered"
		}
		return step
	}
}

// Run executes the ladder and returns every step in order.
func (l Ladder) Run(ctx context.Context, target LadderTarget) []domain.StepResult {
	return l.run(ctx, target, nil)
}

// RunWithProgress executes the ladder and reports every finished step to
// onStep so the UI can show live progress.
func (l Ladder) RunWithProgress(ctx context.Context, target LadderTarget, onStep func(domain.StepResult)) []domain.StepResult {
	return l.run(ctx, target, onStep)
}

func (l Ladder) run(ctx context.Context, target LadderTarget, onStep func(domain.StepResult)) []domain.StepResult {
	steps := make([]domain.StepResult, 0, 9)
	order := 0
	add := func(step domain.StepResult) {
		order++
		step.Order = order
		if step.Measurements == nil {
			step.Measurements = []domain.Measurement{}
		}
		steps = append(steps, step)
		if onStep != nil {
			onStep(step)
		}
	}

	dnsStep, systemAnswers := l.runDNS(ctx, target)
	add(dnsStep)

	if hasProtocol(target.Protocols, domain.PresetTCP) {
		for _, port := range target.Ports {
			if ctx.Err() != nil {
				return steps
			}
			add(l.runTCP(ctx, target, port))
		}
	}
	if hasProtocol(target.Protocols, domain.PresetTLS) && len(target.Ports) > 0 {
		port := target.Ports[0]
		for _, variant := range []struct {
			id  string
			sni string
		}{
			{id: "real", sni: target.Host},
			{id: "decoy", sni: decoySNI},
			{id: "absent", sni: ""},
		} {
			if ctx.Err() != nil {
				return steps
			}
			add(l.runTLS(ctx, target, port, variant.id, variant.sni))
		}
	}
	if ctx.Err() == nil && hasProtocol(target.Protocols, domain.PresetHTTP) && len(target.Ports) > 0 {
		add(l.runHTTP(ctx, target, target.Ports[0]))
	}
	if ctx.Err() == nil && hasProtocol(target.Protocols, domain.PresetICMP) {
		add(l.runICMP(ctx, target, systemAnswers))
	}
	if ctx.Err() == nil {
		add(l.runTrace(ctx, target))
	}

	return steps
}

func (l Ladder) runTrace(ctx context.Context, target LadderTarget) domain.StepResult {
	if l.trace == nil {
		return l.step(ctx, domain.StepTrace, "trace", target.Host, func(context.Context) (domain.Status, []domain.Measurement, string, string) {
			return domain.StatusSkipped, nil, "traceroute method arrives with REQ-005", ""
		})
	}
	return l.trace(ctx, target)
}

func (l Ladder) runDNS(ctx context.Context, target LadderTarget) (domain.StepResult, []string) {
	if ip := net.ParseIP(target.Host); ip != nil {
		step := l.step(ctx, domain.StepDNS, "dns", target.Host, func(context.Context) (domain.Status, []domain.Measurement, string, string) {
			return domain.StatusSkipped, []domain.Measurement{{Name: "address", Value: target.Host}}, "target is an IP literal; resolution is not required", ""
		})
		return step, []string{target.Host}
	}

	var systemAnswers []string
	step := l.step(ctx, domain.StepDNS, "dns", target.Host, func(stepCtx context.Context) (domain.Status, []domain.Measurement, string, string) {
		answers, err := l.resolve(stepCtx, target.Host)
		if err != nil {
			return domain.StatusFail,
				[]domain.Measurement{{Name: "system.answers", Value: "none"}},
				"hostname could not be resolved by the system resolver",
				err.Error()
		}
		systemAnswers = answers
		measurements := []domain.Measurement{{Name: "system.answers", Value: joinOrNone(answers)}}
		if l.doh == nil {
			return domain.StatusPass, measurements, "", ""
		}

		reply, dohErr := l.doh.Resolve(stepCtx, target.Host)
		if dohErr != nil {
			measurements = append(measurements, domain.Measurement{Name: "comparison", Value: "doh_failed"})
			return domain.StatusWarn, measurements, "DoH comparison failed: " + dohErr.Error(), ""
		}
		measurements = append(measurements,
			domain.Measurement{Name: "doh.answers", Value: joinOrNone(reply.Answers)},
		)
		comparison := compareAnswers(answers, reply.Answers)
		measurements = append(measurements, domain.Measurement{Name: "comparison", Value: comparison})
		if comparison == "differ" {
			return domain.StatusWarn, measurements, "system resolver and DoH answers differ", ""
		}
		return domain.StatusPass, measurements, "", ""
	})
	return step, systemAnswers
}

func (l Ladder) runTCP(ctx context.Context, target LadderTarget, port int) domain.StepResult {
	address := net.JoinHostPort(target.Host, strconv.Itoa(port))
	id := fmt.Sprintf("tcp.%d", port)
	return l.step(ctx, domain.StepTCP, id, address, func(stepCtx context.Context) (domain.Status, []domain.Measurement, string, string) {
		start := l.now()
		conn, err := l.dial(stepCtx, "tcp", address)
		latency := l.now().Sub(start)
		if err != nil {
			outcome, passive := classifyTCPError(err)
			diagnostic := "connection failed"
			status := domain.StatusFail
			switch outcome {
			case "refused":
				status, diagnostic = domain.StatusWarn, "connection actively refused"
			case "reset":
				status, diagnostic = domain.StatusWarn, "connection reset by peer"
			case "timeout":
				if passive {
					diagnostic = "connection timed out (passive)"
				} else {
					diagnostic = "connection timed out"
				}
			}
			return status,
				[]domain.Measurement{{Name: "outcome", Value: outcome}, {Name: "latency", Value: formatRTT(latency)}},
				diagnostic, err.Error()
		}
		conn.Close()
		return domain.StatusPass,
			[]domain.Measurement{{Name: "outcome", Value: "connected"}, {Name: "latency", Value: formatRTT(latency)}},
			"", ""
	})
}

func (l Ladder) runTLS(ctx context.Context, target LadderTarget, port int, variant, sni string) domain.StepResult {
	address := net.JoinHostPort(target.Host, strconv.Itoa(port))
	id := "tls." + variant
	sniLabel := sni
	if sniLabel == "" {
		sniLabel = "none"
	}
	return l.step(ctx, domain.StepTLS, id, address, func(stepCtx context.Context) (domain.Status, []domain.Measurement, string, string) {
		conn, err := l.dial(stepCtx, "tcp", address)
		if err != nil {
			outcome, _ := classifyTCPError(err)
			return domain.StatusFail,
				[]domain.Measurement{{Name: "sni", Value: sniLabel}, {Name: "outcome", Value: outcome}},
				"TCP connection for the TLS probe failed", err.Error()
		}

		start := l.now()
		handshakeErr := l.handshake(stepCtx, conn, sni)
		latency := l.now().Sub(start)
		if handshakeErr != nil {
			var alert tls.AlertError
			if errors.As(handshakeErr, &alert) {
				return domain.StatusWarn,
					[]domain.Measurement{{Name: "sni", Value: sniLabel}, {Name: "outcome", Value: "tls_alert"}, {Name: "latency", Value: formatRTT(latency)}},
					"server answered with a TLS alert: " + alert.Error(), ""
			}
			outcome, _ := classifyTCPError(handshakeErr)
			return domain.StatusFail,
				[]domain.Measurement{{Name: "sni", Value: sniLabel}, {Name: "outcome", Value: outcome}, {Name: "latency", Value: formatRTT(latency)}},
				"TLS handshake failed", handshakeErr.Error()
		}
		return domain.StatusPass,
			[]domain.Measurement{{Name: "sni", Value: sniLabel}, {Name: "outcome", Value: "handshake_complete"}, {Name: "latency", Value: formatRTT(latency)}},
			"", ""
	})
}

func (l Ladder) runHTTP(ctx context.Context, target LadderTarget, port int) domain.StepResult {
	scheme := "https"
	if len(target.Protocols) == 1 && target.Protocols[0] == domain.PresetHTTP {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s/", scheme, net.JoinHostPort(target.Host, strconv.Itoa(port)))
	return l.step(ctx, domain.StepHTTP, "http", url, func(stepCtx context.Context) (domain.Status, []domain.Measurement, string, string) {
		request, err := http.NewRequestWithContext(stepCtx, http.MethodGet, url, nil)
		if err != nil {
			return domain.StatusFail, nil, "request could not be built", err.Error()
		}
		start := l.now()
		response, err := l.http.Do(request)
		latency := l.now().Sub(start)
		if err != nil {
			outcome, _ := classifyTCPError(err)
			return domain.StatusFail,
				[]domain.Measurement{{Name: "outcome", Value: outcome}, {Name: "latency", Value: formatRTT(latency)}},
				"HTTP request failed", err.Error()
		}
		defer response.Body.Close()
		io.Copy(io.Discard, io.LimitReader(response.Body, 1<<16))

		measurements := []domain.Measurement{
			{Name: "status", Value: response.Status},
			{Name: "latency", Value: formatRTT(latency)},
		}
		if location := response.Header.Get("Location"); location != "" {
			measurements = append(measurements, domain.Measurement{Name: "location", Value: location})
		}
		return domain.StatusPass, measurements, "", ""
	})
}

func (l Ladder) runICMP(ctx context.Context, target LadderTarget, answers []string) domain.StepResult {
	var ip net.IP
	if parsed := net.ParseIP(target.Host); parsed != nil {
		ip = parsed
	} else if len(answers) > 0 {
		ip = net.ParseIP(answers[0])
	}
	if ip == nil {
		return l.step(ctx, domain.StepICMP, "icmp", target.Host, func(context.Context) (domain.Status, []domain.Measurement, string, string) {
			return domain.StatusSkipped, nil, "no resolved address to probe", ""
		})
	}

	return l.step(ctx, domain.StepICMP, "icmp", ip.String(), func(stepCtx context.Context) (domain.Status, []domain.Measurement, string, string) {
		rtt, err := l.echo(stepCtx, ip)
		if err != nil {
			return domain.StatusFail,
				[]domain.Measurement{{Name: "address", Value: ip.String()}, {Name: "outcome", Value: "no_reply"}},
				"no ICMP reply; ICMP may be rate-limited or blocked",
				err.Error()
		}
		return domain.StatusPass,
			[]domain.Measurement{{Name: "address", Value: ip.String()}, {Name: "rtt", Value: formatRTT(rtt)}},
			"", ""
	})
}

// step runs one probe under its own timeout and records its timing.
func (l Ladder) step(ctx context.Context, kind domain.StepKind, id, target string, run func(context.Context) (domain.Status, []domain.Measurement, string, string)) domain.StepResult {
	started := l.now()
	stepCtx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	status, measurements, diagnostic, errText := run(stepCtx)
	if measurements == nil {
		measurements = []domain.Measurement{}
	}
	return domain.StepResult{
		ID:           id,
		Kind:         kind,
		Target:       target,
		StartedAt:    started,
		DurationMS:   l.now().Sub(started).Milliseconds(),
		Status:       status,
		Measurements: measurements,
		Diagnostic:   diagnostic,
		Error:        errText,
	}
}

func lookupAddresses(ctx context.Context, host string) ([]string, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.IP.String())
	}
	return values, nil
}

func echoOnce(ctx context.Context, ip net.IP, elevated bool, timeout time.Duration) (time.Duration, error) {
	family := platform.FamilyIPv4
	if ip.To4() == nil {
		family = platform.FamilyIPv6
	}
	conn, err := dialICMP(family, elevated, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	return conn.Exchange(ctx, ip, 1)
}

func tlsHandshake(ctx context.Context, conn net.Conn, serverName string) error {
	client := tls.Client(conn, &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	})
	defer client.Close()
	return client.HandshakeContext(ctx)
}

func hasProtocol(protocols []domain.PresetProtocol, want domain.PresetProtocol) bool {
	for _, protocol := range protocols {
		if protocol == want {
			return true
		}
	}
	return false
}

func compareAnswers(system, doh []string) string {
	first := append([]string(nil), system...)
	second := append([]string(nil), doh...)
	sort.Strings(first)
	sort.Strings(second)
	if strings.Join(first, ",") == strings.Join(second, ",") {
		return "match"
	}
	return "differ"
}
