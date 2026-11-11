package probe

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

const (
	// DefaultCaptiveURL is the documented captive-portal canary endpoint.
	DefaultCaptiveURL = "http://captive.apple.com/hotspot-detect.html"
	// captiveBodyFingerprint is the expected success marker in the response
	// body, compared after whitespace and case normalization.
	captiveBodyFingerprint  = "<body>success</body>"
	captiveTitleFingerprint = "<title>success</title>"

	captiveBodyLimit = 160
	captiveReadLimit = 8192
)

// HTTPDoer performs exactly one HTTP request.
type HTTPDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// PortalCheck requests the captive-portal canary once and classifies the
// response as healthy or as a portal.
type PortalCheck struct {
	id     domain.CheckID
	url    string
	client HTTPDoer
	now    func() time.Time
}

// NewPortalCheck returns the captive-portal check against the documented
// canary.
func NewPortalCheck() PortalCheck {
	return PortalCheck{
		id:     "http.captive",
		url:    DefaultCaptiveURL,
		client: defaultPortalClient(),
		now:    time.Now,
	}
}

// NewPortalCheckWithClient builds the check against a custom endpoint. The
// client copy captures redirects instead of following them.
func NewPortalCheckWithClient(url string, client *http.Client) PortalCheck {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	copied := *client
	if copied.Timeout == 0 {
		copied.Timeout = 5 * time.Second
	}
	copied.CheckRedirect = noRedirect
	return PortalCheck{
		id:     "http.captive",
		url:    url,
		client: &copied,
		now:    time.Now,
	}
}

func defaultPortalClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, CheckRedirect: noRedirect}
}

func noRedirect(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// ID returns the stable check id used by the dashboard and JSON output.
func (c PortalCheck) ID() domain.CheckID {
	return c.id
}

// Run makes exactly one request and compares it with the documented response.
func (c PortalCheck) Run(ctx context.Context) domain.Result {
	result := domain.Result{
		CheckID:   c.id,
		Target:    c.url,
		Method:    "http get",
		Timestamp: c.now(),
	}
	if err := ctx.Err(); err != nil {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "canary request cancelled before it started"
		return result
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		result.Status = domain.StatusFail
		result.Error = err.Error()
		result.Diagnostic = "canary request could not be built"
		return result
	}

	response, err := c.client.Do(request)
	if err != nil {
		result.Status = domain.StatusFail
		result.Error = err.Error()
		result.Diagnostic = "canary request failed"
		return result
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, captiveReadLimit))
	if err != nil {
		result.Status = domain.StatusFail
		result.Error = err.Error()
		result.Diagnostic = "canary response could not be read"
		return result
	}

	normalized := normalizeBody(body)
	fingerprint := "unexpected"
	if strings.Contains(normalized, captiveBodyFingerprint) && strings.Contains(normalized, captiveTitleFingerprint) {
		fingerprint = "match"
	}
	location := response.Header.Get("Location")

	result.Measurements = []domain.Measurement{
		{Name: "status", Value: response.Status},
		{Name: "fingerprint", Value: fingerprint},
		{Name: "body", Value: truncateBody(normalized)},
	}
	if location != "" {
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "location", Value: location})
	}

	switch {
	case response.StatusCode == http.StatusOK && fingerprint == "match":
		result.Status = domain.StatusPass
	case response.StatusCode >= 300 && response.StatusCode < 400:
		result.Status = domain.StatusWarn
		result.Diagnostic = "captive portal redirect"
		if location != "" {
			result.Diagnostic += ": " + location
		} else {
			result.Diagnostic += " without a location header"
		}
	case response.StatusCode == http.StatusOK:
		result.Status = domain.StatusWarn
		result.Diagnostic = "canary body does not match the expected fingerprint"
	default:
		result.Status = domain.StatusWarn
		result.Diagnostic = "canary returned " + response.Status
	}
	return result
}

func normalizeBody(body []byte) string {
	return strings.Join(strings.Fields(strings.ToLower(string(body))), " ")
}

func truncateBody(body string) string {
	runes := []rune(body)
	if len(runes) <= captiveBodyLimit {
		return body
	}
	return string(runes[:captiveBodyLimit])
}
