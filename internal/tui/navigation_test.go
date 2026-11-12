package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

func selectedStatus(t *testing.T, model Model) domain.Status {
	t.Helper()
	result, ok := model.SelectedResult()
	if !ok {
		t.Fatal("no result is selected")
	}
	return result.Status
}

func TestProblemJumpCyclesWarnAndFail(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, _ = pressKey(t, model, "g")

	model, _ = pressKey(t, model, "n")
	if status := selectedStatus(t, model); status != domain.StatusWarn {
		t.Fatalf("first n selected %q, want warn", status)
	}
	model, _ = pressKey(t, model, "n")
	if status := selectedStatus(t, model); status != domain.StatusFail {
		t.Fatalf("second n selected %q, want fail", status)
	}
	model, _ = pressKey(t, model, "n")
	if status := selectedStatus(t, model); status != domain.StatusWarn {
		t.Fatalf("third n selected %q, want wrap to warn", status)
	}

	model, _ = pressKey(t, model, "p")
	if status := selectedStatus(t, model); status != domain.StatusFail {
		t.Fatalf("p selected %q, want wrap to fail", status)
	}
	model, _ = pressKey(t, model, "p")
	if status := selectedStatus(t, model); status != domain.StatusWarn {
		t.Fatalf("p selected %q, want warn", status)
	}
}

func TestProblemJumpFromANonProblemRow(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model = model.Select(0)
	if status := selectedStatus(t, model); status != domain.StatusPass {
		t.Fatalf("fixture does not start on a passing check: %q", status)
	}

	model, _ = pressKey(t, model, "p")
	if status := selectedStatus(t, model); status != domain.StatusFail {
		t.Fatalf("p from a passing row selected %q, want the last problem", status)
	}
}

func TestProblemJumpWithoutProblems(t *testing.T) {
	results := []domain.Result{
		testResult("local.interfaces", "en0", domain.StatusPass),
		testResult("dns.canary", "example.com", domain.StatusPass),
	}
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	model := New(results, verdict, WithoutColor()).Resize(100, 30)

	model, cmd := pressKey(t, model, "n")
	if cmd == nil {
		t.Fatal("empty jump did not schedule the notice expiry")
	}
	if !strings.Contains(model.View(), "no failing or warning checks") {
		t.Fatalf("missing notice:\n%s", model.View())
	}
	if model.Selected() != 0 {
		t.Fatalf("selection moved to %d", model.Selected())
	}
}

func TestProblemJumpWorksOnDetail(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, _ = pressKey(t, model, "enter")

	model, _ = pressKey(t, model, "n")
	if status := selectedStatus(t, model); status != domain.StatusWarn {
		t.Fatalf("n on detail selected %q, want warn", status)
	}
	if model.detailOffset != 0 {
		t.Fatalf("jump kept the detail offset %d", model.detailOffset)
	}
	if !strings.Contains(model.View(), "DETAIL") {
		t.Fatal("jump left the detail page")
	}
}

func TestRerunClearsThePreviousRun(t *testing.T) {
	calls := 0
	model := fixtureModel(t, 100, 30, WithoutColor(), WithRerun(func() { calls++ }))
	model, _ = pressKey(t, model, "r")

	if calls != 1 {
		t.Fatalf("rerun calls = %d, want 1", calls)
	}
	if !model.running {
		t.Fatal("dashboard is not marked as running")
	}
	if len(model.results) != 0 {
		t.Fatalf("previous results survived the re-run: %d", len(model.results))
	}
	if view := model.View(); !strings.Contains(view, "RUNNING") {
		t.Fatalf("dashboard does not show RUNNING:\n%s", view)
	}
}

func TestRerunReplacesResults(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithRerun(func() {}))
	model, _ = pressKey(t, model, "r")

	replacement := []domain.Result{testResult("local.interfaces", "en0", domain.StatusPass)}
	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{
		StartedAt:  runStart,
		FinishedAt: runStart.Add(time.Second),
		Results:    replacement,
	}})

	if len(model.results) != len(replacement) {
		t.Fatalf("results = %d, want %d (runs were merged)", len(model.results), len(replacement))
	}
}

func TestRerunIsIgnoredWhileRunning(t *testing.T) {
	calls := 0
	model := fixtureModel(t, 100, 30, WithoutColor(), WithRerun(func() { calls++ }))
	model = model.StartRunning()

	model, cmd := pressKey(t, model, "r")
	if calls != 0 {
		t.Fatalf("re-run started while running: %d calls", calls)
	}
	if cmd == nil {
		t.Fatal("blocked re-run did not schedule the notice expiry")
	}
	if !strings.Contains(model.View(), "baseline is already running") {
		t.Fatalf("missing notice:\n%s", model.View())
	}
}

func TestRerunWithoutHook(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, _ = pressKey(t, model, "r")
	if !strings.Contains(model.View(), "re-run is unavailable") {
		t.Fatalf("missing notice:\n%s", model.View())
	}
}
