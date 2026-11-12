package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// CanaryHost is the hostname resolved by the DNS check.
const CanaryHost = "example.com"

const defaultDNSTimeout = 2 * time.Second

// DefaultDoHEndpoint is the public resolver used only as a diagnostic probe.
const DefaultDoHEndpoint = "https://cloudflare-dns.com/dns-query"

// DNSAnswer is one resolver response.
type DNSAnswer struct {
	RCode   string
	Answers []string
	Latency time.Duration
}

// DNSExchanger sends one query to one resolver.
type DNSExchanger interface {
	Exchange(ctx context.Context, host, server string) (DNSAnswer, error)
}

// DoHAnswer is one DNS-over-HTTPS response.
type DoHAnswer struct {
	RCode   string
	Answers []string
	Latency time.Duration
}

// DoHResolver resolves one host over HTTPS.
type DoHResolver interface {
	Resolve(ctx context.Context, host string) (DoHAnswer, error)
}

// ResolverSource supplies the configured nameservers.
type ResolverSource interface {
	Resolvers() ([]platform.Resolver, error)
}

type systemResolverSource struct {
	caps platform.Capabilities
}

func (s systemResolverSource) Resolvers() ([]platform.Resolver, error) {
	return s.caps.Resolvers()
}

// DNSCheck reports configured resolvers and canary resolution per resolver.
type DNSCheck struct {
	resolvers ResolverSource
	exchanger DNSExchanger
	doh       DoHResolver
	now       func() time.Time
}

// NewDNSCheck returns the canary resolution check against the host resolvers
// and the public DoH diagnostic resolver.
func NewDNSCheck() DNSCheck {
	timeout := defaultDNSTimeout
	return DNSCheck{
		resolvers: systemResolverSource{caps: platform.System()},
		exchanger: miekgExchanger{client: &dns.Client{Timeout: timeout}},
		doh:       cloudflareDoH{client: &http.Client{Timeout: 3 * time.Second}, endpoint: DefaultDoHEndpoint},
		now:       time.Now,
	}
}

// NewDNSCheckWithSource builds a system-only check with an injected resolver
// source and exchanger.
func NewDNSCheckWithSource(resolvers ResolverSource, exchanger DNSExchanger, now func() time.Time) DNSCheck {
	return NewDNSCheckWithClients(resolvers, exchanger, nil, now)
}

// NewDNSCheckWithClients builds the check with every dependency injected.
func NewDNSCheckWithClients(resolvers ResolverSource, exchanger DNSExchanger, doh DoHResolver, now func() time.Time) DNSCheck {
	timeout := defaultDNSTimeout
	if resolvers == nil {
		resolvers = systemResolverSource{caps: platform.System()}
	}
	if exchanger == nil {
		exchanger = miekgExchanger{client: &dns.Client{Timeout: timeout}}
	}
	if now == nil {
		now = time.Now
	}
	return DNSCheck{resolvers: resolvers, exchanger: exchanger, doh: doh, now: now}
}

// ID returns the stable check id used by the dashboard and JSON output.
func (c DNSCheck) ID() domain.CheckID {
	return "dns.canary"
}

// Run resolves the canary with every configured resolver and reports each
// answer, response code and latency.
func (c DNSCheck) Run(ctx context.Context) domain.Result {
	result := domain.Result{
		CheckID:   c.ID(),
		Target:    CanaryHost,
		Method:    "dns query",
		Timestamp: c.now(),
	}
	if err := ctx.Err(); err != nil {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "query cancelled before it started"
		return result
	}

	resolvers, err := c.resolvers.Resolvers()
	if err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			result.Status = domain.StatusSkipped
			result.Measurements = []domain.Measurement{{Name: "resolvers", Value: "unsupported"}}
			result.Diagnostic = "resolver configuration is not supported on this platform"
			return result
		}
		result.Status = domain.StatusFail
		result.Error = err.Error()
		result.Measurements = []domain.Measurement{{Name: "resolvers", Value: "unavailable"}}
		result.Diagnostic = "resolver configuration unavailable"
		return result
	}
	if len(resolvers) == 0 {
		result.Status = domain.StatusFail
		result.Measurements = []domain.Measurement{{Name: "resolvers", Value: "none"}}
		result.Diagnostic = "no resolver configured"
		return result
	}

	var failures []string
	succeeded := 0
	var systemAnswers []string
	for _, resolver := range resolvers {
		server := net.JoinHostPort(resolver.Address, "53")
		answer, err := c.exchanger.Exchange(ctx, CanaryHost, server)
		prefix := "resolver." + resolver.Address
		if err != nil {
			code := "error"
			if isTimeout(err) {
				code = "timeout"
			}
			result.Measurements = append(result.Measurements, domain.Measurement{
				Name:  prefix + ".rcode",
				Value: code,
			})
			failures = append(failures, resolver.Address+": "+code)
			continue
		}

		sort.Strings(answer.Answers)
		result.Measurements = append(result.Measurements,
			domain.Measurement{Name: prefix + ".rcode", Value: answer.RCode},
			domain.Measurement{Name: prefix + ".answers", Value: joinOrNone(answer.Answers)},
			domain.Measurement{Name: prefix + ".latency", Value: answer.Latency.Round(time.Millisecond).String()},
		)
		if answer.RCode == "NOERROR" && len(answer.Answers) > 0 {
			succeeded++
			if systemAnswers == nil {
				systemAnswers = append([]string(nil), answer.Answers...)
			}
			continue
		}
		failures = append(failures, resolver.Address+": "+answer.RCode)
	}

	switch {
	case succeeded == 0:
		result.Status = domain.StatusFail
		result.Diagnostic = "no resolver answered: " + strings.Join(failures, "; ")
	case succeeded < len(resolvers):
		result.Status = domain.StatusWarn
		result.Diagnostic = "some resolvers failed: " + strings.Join(failures, "; ")
	default:
		result.Status = domain.StatusPass
	}

	if c.doh != nil {
		measurements, comparison, note := c.compareDoH(ctx, systemAnswers)
		result.Measurements = append(result.Measurements, measurements...)
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "comparison", Value: comparison})
		if note != "" {
			if result.Diagnostic != "" {
				result.Diagnostic += "; "
			}
			result.Diagnostic += note
		}
		if result.Status == domain.StatusPass && (comparison == "differ" || comparison == "doh_failed") {
			result.Status = domain.StatusWarn
		}
	}
	return result
}

// compareDoH queries the DoH resolver, records its evidence and returns the
// comparison outcome against the first successful system answer.
func (c DNSCheck) compareDoH(ctx context.Context, systemAnswers []string) ([]domain.Measurement, string, string) {
	answer, err := c.doh.Resolve(ctx, CanaryHost)
	if err != nil {
		return []domain.Measurement{{Name: "doh.rcode", Value: "unavailable"}},
			"doh_failed", "DoH unavailable: " + err.Error()
	}
	sort.Strings(answer.Answers)
	measurements := []domain.Measurement{
		{Name: "doh.rcode", Value: answer.RCode},
		{Name: "doh.answers", Value: joinOrNone(answer.Answers)},
		{Name: "doh.latency", Value: answer.Latency.Round(time.Millisecond).String()},
	}
	if answer.RCode != "NOERROR" || len(answer.Answers) == 0 {
		return measurements, "doh_failed", "DoH returned " + answer.RCode
	}
	if len(systemAnswers) == 0 {
		return measurements, "no_system_answer", ""
	}
	if sameAnswers(systemAnswers, answer.Answers) {
		return measurements, "match", ""
	}
	return measurements, "differ", "system and DoH answers differ; CDN and geo-DNS can explain the difference"
}

func sameAnswers(left, right []string) bool {
	left = uniqueAnswers(left)
	right = uniqueAnswers(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !strings.EqualFold(left[i], right[i]) {
			return false
		}
	}
	return true
}

func uniqueAnswers(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, value)
	}
	return unique
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// miekgExchanger is the production resolver client.
type miekgExchanger struct {
	client *dns.Client
}

func (e miekgExchanger) Exchange(ctx context.Context, host, server string) (DNSAnswer, error) {
	query := new(dns.Msg)
	query.SetQuestion(dns.Fqdn(host), dns.TypeA)
	response, latency, err := e.client.ExchangeContext(ctx, query, server)
	if err != nil {
		return DNSAnswer{}, err
	}

	answers := make([]string, 0, len(response.Answer))
	for _, record := range response.Answer {
		switch value := record.(type) {
		case *dns.A:
			answers = append(answers, value.A.String())
		case *dns.AAAA:
			answers = append(answers, value.AAAA.String())
		case *dns.CNAME:
			answers = append(answers, value.Target)
		}
	}
	rcode, ok := dns.RcodeToString[response.Rcode]
	if !ok {
		rcode = fmt.Sprintf("RCODE%d", response.Rcode)
	}
	return DNSAnswer{RCode: rcode, Answers: answers, Latency: latency}, nil
}

// cloudflareDoH is the production DoH client using the JSON API.
type cloudflareDoH struct {
	client   *http.Client
	endpoint string
}

func (d cloudflareDoH) Resolve(ctx context.Context, host string) (DoHAnswer, error) {
	endpoint, err := url.Parse(d.endpoint)
	if err != nil {
		return DoHAnswer{}, fmt.Errorf("parse doh endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("name", host)
	query.Set("type", "A")
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return DoHAnswer{}, fmt.Errorf("build doh request: %w", err)
	}
	request.Header.Set("Accept", "application/dns-json")

	start := time.Now()
	response, err := d.client.Do(request)
	latency := time.Since(start)
	if err != nil {
		return DoHAnswer{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return DoHAnswer{}, fmt.Errorf("doh status %s", response.Status)
	}

	var payload struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return DoHAnswer{}, fmt.Errorf("decode doh response: %w", err)
	}

	answers := make([]string, 0, len(payload.Answer))
	for _, record := range payload.Answer {
		if record.Type == 1 || record.Type == 28 {
			answers = append(answers, record.Data)
		}
	}
	rcode, ok := dns.RcodeToString[payload.Status]
	if !ok {
		rcode = fmt.Sprintf("RCODE%d", payload.Status)
	}
	return DoHAnswer{RCode: rcode, Answers: answers, Latency: latency}, nil
}
