package probe

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

type fakeResolverSource struct {
	resolvers []platform.Resolver
	err       error
}

func (f fakeResolverSource) Resolvers() ([]platform.Resolver, error) {
	return f.resolvers, f.err
}

type exchangeCall struct {
	host   string
	server string
}

type fakeExchanger struct {
	answers map[string]DNSAnswer
	errs    map[string]error
	calls   []exchangeCall
}

func (f *fakeExchanger) Exchange(_ context.Context, host, server string) (DNSAnswer, error) {
	f.calls = append(f.calls, exchangeCall{host: host, server: server})
	if err, ok := f.errs[server]; ok {
		return DNSAnswer{}, err
	}
	if answer, ok := f.answers[server]; ok {
		return answer, nil
	}
	return DNSAnswer{}, errors.New("no fake answer for " + server)
}

func newDNSCheck(source ResolverSource, exchanger DNSExchanger) DNSCheck {
	return NewDNSCheckWithSource(source, exchanger, func() time.Time { return fixedTime })
}

type fakeDoH struct {
	answer DoHAnswer
	err    error
	calls  int
}

func (f *fakeDoH) Resolve(_ context.Context, _ string) (DoHAnswer, error) {
	f.calls++
	return f.answer, f.err
}

func TestDNSCheckPass(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: 12 * time.Millisecond},
		"8.8.8.8:53":     {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: 24 * time.Millisecond},
	}}
	check := newDNSCheck(fakeResolverSource{resolvers: []platform.Resolver{
		{Address: "192.168.1.1"},
		{Address: "8.8.8.8"},
	}}, exchanger)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
	if result.CheckID != "dns.canary" || result.Target != CanaryHost {
		t.Fatalf("unexpected identity: %+v", result)
	}
	if len(exchanger.calls) != 2 {
		t.Fatalf("exchanged %d times, want 2", len(exchanger.calls))
	}
	for _, call := range exchanger.calls {
		if call.host != CanaryHost {
			t.Fatalf("queried %q, want %q", call.host, CanaryHost)
		}
	}

	want := map[string]string{
		"resolver.192.168.1.1.rcode":   "NOERROR",
		"resolver.192.168.1.1.answers": "93.184.216.34",
		"resolver.192.168.1.1.latency": "12ms",
		"resolver.8.8.8.8.latency":     "24ms",
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
}

func TestDNSCheckNXDOMAIN(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NXDOMAIN", Latency: 5 * time.Millisecond},
	}}
	check := newDNSCheck(fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}}, exchanger)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "NXDOMAIN") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "resolver.192.168.1.1.rcode"); value != "NXDOMAIN" {
		t.Fatalf("rcode = %q", value)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckDistinguishesSERVFAILAndTimeout(t *testing.T) {
	exchanger := &fakeExchanger{
		answers: map[string]DNSAnswer{
			"192.168.1.1:53": {RCode: "SERVFAIL", Latency: time.Millisecond},
		},
		errs: map[string]error{
			"8.8.8.8:53": context.DeadlineExceeded,
		},
	}
	check := newDNSCheck(fakeResolverSource{resolvers: []platform.Resolver{
		{Address: "192.168.1.1"},
		{Address: "8.8.8.8"},
	}}, exchanger)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if value, _ := findMeasurement(result, "resolver.192.168.1.1.rcode"); value != "SERVFAIL" {
		t.Fatalf("SERVFAIL resolver rcode = %q", value)
	}
	if value, _ := findMeasurement(result, "resolver.8.8.8.8.rcode"); value != "timeout" {
		t.Fatalf("timed-out resolver rcode = %q", value)
	}
	if !strings.Contains(result.Diagnostic, "SERVFAIL") || !strings.Contains(result.Diagnostic, "timeout") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
}

func TestDNSCheckPartialFailureIsWarn(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: 3 * time.Millisecond},
		"8.8.8.8:53":     {RCode: "NXDOMAIN", Latency: 4 * time.Millisecond},
	}}
	check := newDNSCheck(fakeResolverSource{resolvers: []platform.Resolver{
		{Address: "192.168.1.1"},
		{Address: "8.8.8.8"},
	}}, exchanger)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "some resolvers failed") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckNoResolvers(t *testing.T) {
	check := newDNSCheck(fakeResolverSource{}, &fakeExchanger{})
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if result.Diagnostic != "no resolver configured" {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "resolvers"); value != "none" {
		t.Fatalf("resolvers measurement = %q", value)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckResolverConfigError(t *testing.T) {
	check := newDNSCheck(fakeResolverSource{err: errors.New("scutil failed")}, &fakeExchanger{})
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Error, "scutil failed") {
		t.Fatalf("error = %q", result.Error)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	check := newDNSCheck(fakeResolverSource{}, &fakeExchanger{})
	result := check.Run(ctx)
	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if result.Diagnostic == "" {
		t.Fatal("skipped result has no reason")
	}
}

func TestDNSCheckIPv6ServerAddress(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"[fe80::1%en0]:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: time.Millisecond},
	}}
	check := newDNSCheck(fakeResolverSource{resolvers: []platform.Resolver{{Address: "fe80::1%en0"}}}, exchanger)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if len(exchanger.calls) != 1 || exchanger.calls[0].server != "[fe80::1%en0]:53" {
		t.Fatalf("calls = %+v", exchanger.calls)
	}
}

func TestDNSCheckSortsAnswers(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"b.example", "a.example"}, Latency: time.Millisecond},
	}}
	check := newDNSCheck(fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}}, exchanger)
	result := check.Run(context.Background())

	value, _ := findMeasurement(result, "resolver.192.168.1.1.answers")
	if value != "a.example, b.example" {
		t.Fatalf("answers = %q", value)
	}
}

func TestDNSCheckDoHMatch(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: time.Millisecond},
	}}
	doh := &fakeDoH{answer: DoHAnswer{RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: 20 * time.Millisecond}}
	check := NewDNSCheckWithClients(
		fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}},
		exchanger,
		doh,
		func() time.Time { return fixedTime },
	)
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if value, _ := findMeasurement(result, "comparison"); value != "match" {
		t.Fatalf("comparison = %q, want match", value)
	}
	if value, _ := findMeasurement(result, "doh.answers"); value != "93.184.216.34" {
		t.Fatalf("doh.answers = %q", value)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckDoHDiffer(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: time.Millisecond},
	}}
	doh := &fakeDoH{answer: DoHAnswer{RCode: "NOERROR", Answers: []string{"203.0.113.10"}, Latency: 20 * time.Millisecond}}
	check := NewDNSCheckWithClients(
		fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}},
		exchanger,
		doh,
		func() time.Time { return fixedTime },
	)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if value, _ := findMeasurement(result, "comparison"); value != "differ" {
		t.Fatalf("comparison = %q, want differ", value)
	}
	if !strings.Contains(result.Diagnostic, "CDN") || !strings.Contains(result.Diagnostic, "geo-DNS") {
		t.Fatalf("diagnostic = %q, want the CDN and geo-DNS caveat", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckDoHFailureKeepsSystemAnswer(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: time.Millisecond},
	}}
	doh := &fakeDoH{err: errors.New("tls handshake failed")}
	check := NewDNSCheckWithClients(
		fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}},
		exchanger,
		doh,
		func() time.Time { return fixedTime },
	)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if value, _ := findMeasurement(result, "comparison"); value != "doh_failed" {
		t.Fatalf("comparison = %q, want doh_failed", value)
	}
	if value, _ := findMeasurement(result, "resolver.192.168.1.1.answers"); value != "93.184.216.34" {
		t.Fatalf("system answer was erased: %q", value)
	}
	if value, _ := findMeasurement(result, "doh.rcode"); value != "unavailable" {
		t.Fatalf("doh.rcode = %q", value)
	}
	if !strings.Contains(result.Diagnostic, "DoH unavailable") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestDNSCheckDoHWithoutSystemAnswer(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NXDOMAIN", Latency: time.Millisecond},
	}}
	doh := &fakeDoH{answer: DoHAnswer{RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: 20 * time.Millisecond}}
	check := NewDNSCheckWithClients(
		fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}},
		exchanger,
		doh,
		func() time.Time { return fixedTime },
	)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if value, _ := findMeasurement(result, "comparison"); value != "no_system_answer" {
		t.Fatalf("comparison = %q, want no_system_answer", value)
	}
	if value, _ := findMeasurement(result, "doh.answers"); value != "93.184.216.34" {
		t.Fatalf("doh answer is missing: %q", value)
	}
}

func TestDNSCheckDoHReturnsSERVFAIL(t *testing.T) {
	exchanger := &fakeExchanger{answers: map[string]DNSAnswer{
		"192.168.1.1:53": {RCode: "NOERROR", Answers: []string{"93.184.216.34"}, Latency: time.Millisecond},
	}}
	doh := &fakeDoH{answer: DoHAnswer{RCode: "SERVFAIL", Latency: 20 * time.Millisecond}}
	check := NewDNSCheckWithClients(
		fakeResolverSource{resolvers: []platform.Resolver{{Address: "192.168.1.1"}}},
		exchanger,
		doh,
		func() time.Time { return fixedTime },
	)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if value, _ := findMeasurement(result, "comparison"); value != "doh_failed" {
		t.Fatalf("comparison = %q, want doh_failed", value)
	}
	if !strings.Contains(result.Diagnostic, "SERVFAIL") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
}

func TestCloudflareDoHResolve(t *testing.T) {
	var queryName, queryType, accept string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		queryName = request.URL.Query().Get("name")
		queryType = request.URL.Query().Get("type")
		accept = request.Header.Get("Accept")
		writer.Header().Set("Content-Type", "application/dns-json")
		io.WriteString(writer, `{"Status":0,"Answer":[{"name":"example.com.","type":1,"ttl":60,"data":"93.184.216.34"},{"name":"example.com.","type":5,"ttl":60,"data":"example.com."}]}`)
	}))
	defer server.Close()

	client := cloudflareDoH{client: server.Client(), endpoint: server.URL}
	answer, err := client.Resolve(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if answer.RCode != "NOERROR" {
		t.Fatalf("rcode = %q", answer.RCode)
	}
	if len(answer.Answers) != 1 || answer.Answers[0] != "93.184.216.34" {
		t.Fatalf("answers = %+v", answer.Answers)
	}
	if queryName != "example.com" || queryType != "A" {
		t.Fatalf("query = name %q type %q", queryName, queryType)
	}
	if accept != "application/dns-json" {
		t.Fatalf("Accept = %q", accept)
	}
}

func TestCloudflareDoHHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := cloudflareDoH{client: server.Client(), endpoint: server.URL}
	if _, err := client.Resolve(context.Background(), "example.com"); err == nil {
		t.Fatal("Resolve() accepted an HTTP error")
	}
}

func TestDNSCheckIntegration(t *testing.T) {
	if os.Getenv("EGRESS_INTEGRATION") == "" {
		t.Skip("set EGRESS_INTEGRATION=1 to run the live DNS check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := NewDNSCheck().Run(ctx)
	if err := result.Validate(); err != nil {
		t.Fatalf("live result is invalid: %v", err)
	}
	if _, ok := findMeasurement(result, "comparison"); !ok {
		t.Fatal("live run did not record a DoH comparison")
	}
	t.Logf("status=%s diagnostic=%q measurements=%+v", result.Status, result.Diagnostic, result.Measurements)
}

func TestDNSCheckUnsupportedPlatform(t *testing.T) {
	check := newDNSCheck(fakeResolverSource{err: platform.ErrUnsupported}, &fakeExchanger{})
	result := check.Run(context.Background())

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "not supported") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "resolvers"); value != "unsupported" {
		t.Fatalf("resolvers = %q", value)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}
