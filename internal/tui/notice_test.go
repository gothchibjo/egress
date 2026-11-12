package tui

import (
	"strings"
	"testing"

	"github.com/gothchibjo/egress/internal/domain"
)

func TestNoticeClearsOnTheNextAction(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)
	model, _ = pressKey(t, model, "e")
	if !strings.Contains(model.View(), "wrote ") {
		t.Fatal("export notice is missing")
	}

	model, _ = pressKey(t, model, "j")
	view := model.View()
	if strings.Contains(view, "wrote ") {
		t.Fatal("notice survived the next action")
	}
	if !strings.Contains(view, "selected") {
		t.Fatal("footer did not return to the selection status")
	}
}

func TestNoticeExpiresAfterTimeout(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)
	model, _ = pressKey(t, model, "e")

	id := model.noticeID
	model = applyMessage(t, model, noticeExpiredMsg{ID: id})
	if strings.Contains(model.View(), "wrote ") {
		t.Fatalf("notice did not expire:\n%s", model.View())
	}
}

func TestStaleNoticeTimerKeepsTheNewerMessage(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)
	model, _ = pressKey(t, model, "e")
	staleID := model.noticeID

	model, _ = pressKey(t, model, "r") // no re-run hook: sets a newer notice
	if !strings.Contains(model.View(), "re-run is unavailable") {
		t.Fatal("newer notice is missing")
	}

	model = applyMessage(t, model, noticeExpiredMsg{ID: staleID})
	if !strings.Contains(model.View(), "re-run is unavailable") {
		t.Fatal("an old timer cleared the newer notice")
	}
}

func TestNoticeSharesTheFooterStatusLine(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)
	model, _ = pressKey(t, model, "e")

	view := model.View()
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if strings.Contains(line, "wrote ") {
			if !strings.Contains(line, "q quit") {
				t.Fatalf("notice is not on the footer status line: %q", line)
			}
			return
		}
	}
	t.Fatal("notice is not rendered")
}

func failureResult(status domain.Status, diagnostic, errText string) domain.Result {
	result := testResult("tcp.anchor.custom.443", "198.51.100.10:443", status)
	result.Measurements = []domain.Measurement{{Name: "outcome", Value: "timeout"}}
	result.Diagnostic = diagnostic
	result.Error = errText
	return result
}

func evidenceModel(t *testing.T, result domain.Result) Model {
	t.Helper()
	verdict, err := domain.AggregateVerdict([]domain.Result{result})
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	return New([]domain.Result{result}, verdict, WithoutColor()).Resize(100, 30)
}

func TestEvidenceMarksFailureLines(t *testing.T) {
	tests := []struct {
		name   string
		result domain.Result
		marker string
	}{
		{
			name:   "fail with diagnostic",
			result: failureResult(domain.StatusFail, "destination could not be resolved", ""),
			marker: "! diagnostic",
		},
		{
			name:   "fail with error",
			result: failureResult(domain.StatusFail, "", "dial tcp 198.51.100.10:443: i/o timeout"),
			marker: "! error",
		},
		{
			name:   "warn with diagnostic",
			result: failureResult(domain.StatusWarn, "some resolvers failed", ""),
			marker: "! diagnostic",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := evidenceModel(t, test.result)
			view := model.View()
			if !strings.Contains(view, test.marker) {
				t.Fatalf("failure line is not marked:\n%s", view)
			}

			model, _ = pressKey(t, model, "enter")
			if !strings.Contains(model.View(), test.marker) {
				t.Fatalf("detail page does not mark the failure line:\n%s", model.View())
			}
		})
	}
}

func TestEvidenceLeavesPassingResultsUnmarked(t *testing.T) {
	result := testResult("dns.canary", "example.com", domain.StatusPass)
	result.Diagnostic = "DoH comparison matched"

	model := evidenceModel(t, result)
	view := model.View()
	if strings.Contains(view, "! diagnostic") {
		t.Fatalf("passing result is marked as a failure:\n%s", view)
	}
	if !strings.Contains(view, "DoH comparison matched") {
		t.Fatalf("diagnostic is not rendered:\n%s", view)
	}
}
