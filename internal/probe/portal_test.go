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
	"unicode/utf8"

	"github.com/gothchibjo/egress/internal/domain"
)

const captiveSuccessBody = `<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>`

type fakeDoer struct {
	calls int
	resp  *http.Response
	err   error
	req   *http.Request
}

func (f *fakeDoer) Do(request *http.Request) (*http.Response, error) {
	f.calls++
	f.req = request
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func testPortalCheck(url string, doer HTTPDoer) PortalCheck {
	return PortalCheck{
		id:     "http.captive",
		url:    url,
		client: doer,
		now:    func() time.Time { return fixedTime },
	}
}

func jsonResponse(status int, statusText, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     statusText,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestPortalCheckHealthy(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", request.Method)
		}
		if request.Header.Get("Authorization") != "" {
			t.Error("canary request carries an Authorization header")
		}
		io.WriteString(writer, captiveSuccessBody)
	}))
	defer server.Close()

	check := NewPortalCheckWithClient(server.URL+"/hotspot-detect.html", server.Client())
	result := check.Run(context.Background())

	if result.Status != domain.StatusPass {
		t.Fatalf("status = %q, want pass: %+v", result.Status, result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want exactly 1", requests)
	}
	if value, _ := findMeasurement(result, "fingerprint"); value != "match" {
		t.Fatalf("fingerprint = %q", value)
	}
	if value, _ := findMeasurement(result, "status"); value != "200 OK" {
		t.Fatalf("status measurement = %q", value)
	}
}

func TestPortalCheckRedirect(t *testing.T) {
	canaryRequests := 0
	portalRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/portal" {
			portalRequests++
			io.WriteString(writer, "login")
			return
		}
		canaryRequests++
		http.Redirect(writer, request, "/portal", http.StatusFound)
	}))
	defer server.Close()

	check := NewPortalCheckWithClient(server.URL+"/hotspot-detect.html", server.Client())
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn: %+v", result.Status, result)
	}
	if !strings.Contains(result.Diagnostic, "redirect") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "location"); value != "/portal" {
		t.Fatalf("location = %q", value)
	}
	if canaryRequests != 1 || portalRequests != 0 {
		t.Fatalf("canary requests = %d, portal requests = %d; redirects must not be followed", canaryRequests, portalRequests)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestPortalCheckAlteredBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		io.WriteString(writer, "<html><body><h1>Please sign in to the network</h1></body></html>")
	}))
	defer server.Close()

	check := NewPortalCheckWithClient(server.URL, server.Client())
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "does not match") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if value, _ := findMeasurement(result, "fingerprint"); value != "unexpected" {
		t.Fatalf("fingerprint = %q", value)
	}
	if value, _ := findMeasurement(result, "body"); !strings.Contains(value, "sign in") {
		t.Fatalf("body snippet = %q", value)
	}
}

func TestPortalCheckNonOKStatus(t *testing.T) {
	doer := &fakeDoer{resp: jsonResponse(http.StatusServiceUnavailable, "503 Service Unavailable", "unavailable")}
	check := testPortalCheck("http://canary.test/hotspot-detect.html", doer)
	result := check.Run(context.Background())

	if result.Status != domain.StatusWarn {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Diagnostic, "503") {
		t.Fatalf("diagnostic = %q", result.Diagnostic)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestPortalCheckTransportError(t *testing.T) {
	doer := &fakeDoer{err: errors.New("connection refused")}
	check := testPortalCheck("http://canary.test/hotspot-detect.html", doer)
	result := check.Run(context.Background())

	if result.Status != domain.StatusFail {
		t.Fatalf("status = %q, want fail", result.Status)
	}
	if !strings.Contains(result.Error, "connection refused") {
		t.Fatalf("error = %q", result.Error)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result is invalid: %v", err)
	}
}

func TestPortalCheckSnippetIsBounded(t *testing.T) {
	body := strings.Repeat("portal ", 200)
	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, "200 OK", body)}
	check := testPortalCheck("http://canary.test", doer)
	result := check.Run(context.Background())

	value, _ := findMeasurement(result, "body")
	if utf8.RuneCountInString(value) > captiveBodyLimit {
		t.Fatalf("body snippet has %d runes, limit %d", utf8.RuneCountInString(value), captiveBodyLimit)
	}
	if strings.ContainsAny(value, "\n\r") {
		t.Fatal("body snippet contains a line break")
	}
}

func TestPortalCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	doer := &fakeDoer{resp: jsonResponse(http.StatusOK, "200 OK", captiveSuccessBody)}
	check := testPortalCheck("http://canary.test", doer)
	result := check.Run(ctx)

	if result.Status != domain.StatusSkipped {
		t.Fatalf("status = %q, want skipped", result.Status)
	}
	if doer.calls != 0 {
		t.Fatal("canary request was made after cancellation")
	}
}

func TestNewPortalCheckDefaults(t *testing.T) {
	check := NewPortalCheck()
	if check.ID() != "http.captive" {
		t.Fatalf("id = %q", check.ID())
	}
	if check.url != DefaultCaptiveURL {
		t.Fatalf("url = %q", check.url)
	}
	if check.client == nil {
		t.Fatal("portal check has no HTTP client")
	}
}

func TestPortalCheckIntegration(t *testing.T) {
	if os.Getenv("EGRESS_INTEGRATION") == "" {
		t.Skip("set EGRESS_INTEGRATION=1 to run the live captive portal check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := NewPortalCheck().Run(ctx)
	if err := result.Validate(); err != nil {
		t.Fatalf("live result is invalid: %v", err)
	}
	t.Logf("status=%s diagnostic=%q measurements=%+v", result.Status, result.Diagnostic, result.Measurements)
}
