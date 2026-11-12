package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

var runStart = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

func applyMessage(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := model.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update(%T) returned %T", msg, updated)
	}
	return next
}

func TestRunProgressMessage(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30)
	model = applyMessage(t, model, RunProgressMsg{Progress: app.Progress{
		CheckID: "dns.canary",
		Index:   1,
		Total:   17,
		Elapsed: 1500 * time.Millisecond,
	}})

	view := model.View()
	for _, want := range []string{"RUNNING", "running dns.canary (2/17)", "1.5s", "Baseline running"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view is missing %q", want)
		}
	}
}

func TestRunCompleteMessage(t *testing.T) {
	results := fixtureResults()
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{
		StartedAt:  runStart,
		FinishedAt: runStart.Add(2 * time.Second),
		Results:    results,
	}})

	view := model.View()
	for _, want := range []string{"DEGRADED", "pass 3", "2s", "dns.canary"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view is missing %q", want)
		}
	}
	for _, unwanted := range []string{"RUNNING", "Baseline running"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("view still contains %q", unwanted)
		}
	}
	if model.Selected() != 0 {
		t.Fatalf("selection = %d after a completed run, want 0", model.Selected())
	}
}

func TestRunReplacesPreviousRun(t *testing.T) {
	first := app.Run{
		StartedAt:  runStart,
		FinishedAt: runStart.Add(time.Second),
		Results:    []domain.Result{testResult("local.interfaces", "en0", domain.StatusPass)},
	}
	second := app.Run{
		StartedAt:  runStart.Add(time.Minute),
		FinishedAt: runStart.Add(time.Minute + time.Second),
		Results:    []domain.Result{testResult("gateway.reach", "192.168.1.1", domain.StatusFail)},
	}

	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30)
	model = applyMessage(t, model, RunCompleteMsg{Run: first})
	model = applyMessage(t, model, RunCompleteMsg{Run: second})

	view := model.View()
	if !strings.Contains(view, "gateway.reach") || !strings.Contains(view, "BLOCKED") {
		t.Fatal("the second run is not displayed")
	}
	if strings.Contains(view, "local.interfaces") {
		t.Fatal("the previous run was merged into the dashboard")
	}
}

func TestStartRunningKeepsPreviousRunVisible(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30)
	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{
		StartedAt:  runStart,
		FinishedAt: runStart.Add(time.Second),
		Results:    []domain.Result{testResult("local.interfaces", "en0", domain.StatusPass)},
	}})
	model = model.StartRunning()

	view := model.View()
	if !strings.Contains(view, "RUNNING") {
		t.Fatal("dashboard does not show the new run")
	}
	if !strings.Contains(view, "local.interfaces") {
		t.Fatal("dashboard lost the previous run while the new one runs")
	}
}

func TestRunCompleteError(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	model = applyMessage(t, model, RunCompleteMsg{Err: errors.New("runner exploded")})

	view := model.View()
	for _, want := range []string{"ERROR", "runner exploded", "Baseline failed."} {
		if !strings.Contains(view, want) {
			t.Fatalf("view is missing %q", want)
		}
	}
}

func TestRunCompleteWithoutResults(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{StartedAt: runStart, FinishedAt: runStart.Add(time.Second)}})

	view := model.View()
	if !strings.Contains(view, "NO VERDICT") {
		t.Fatal("empty run reports a verdict")
	}
	if !strings.Contains(view, "No baseline results yet.") {
		t.Fatal("empty run has no placeholder")
	}
}

func TestCancelKeysRequestRunnerCancellation(t *testing.T) {
	current, all := 0, 0
	model := New(nil, domain.Verdict{}, WithoutColor(), WithCancelHooks(
		func() { current++ },
		func() { all++ },
	)).Resize(100, 30)

	model, _ = pressKey(t, model, "x")
	model, _ = pressKey(t, model, "X")
	if current != 0 || all != 0 {
		t.Fatal("cancel keys fired while no baseline was running")
	}

	model = model.StartRunning()
	model, _ = pressKey(t, model, "x")
	if current != 1 {
		t.Fatalf("x calls = %d, want 1", current)
	}
	if view := model.View(); !strings.Contains(view, "cancelling current check") {
		t.Fatal("dashboard does not report the current-check cancellation")
	}

	model, _ = pressKey(t, model, "X")
	if all != 1 {
		t.Fatalf("X calls = %d, want 1", all)
	}
	if view := model.View(); !strings.Contains(view, "cancelling baseline") {
		t.Fatal("dashboard does not report the baseline cancellation")
	}

	model, _ = pressKey(t, model, "esc")
	if current != 1 || all != 1 {
		t.Fatal("Esc cancelled the baseline")
	}
}

func TestFooterShowsCancelHintsWhileRunning(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	view := model.View()
	for _, hint := range []string{"x cancel check", "X cancel run"} {
		if !strings.Contains(view, hint) {
			t.Fatalf("running footer is missing %q", hint)
		}
	}
}

func TestConfirmQuitCancelsRunningBaseline(t *testing.T) {
	cancelled := 0
	model := New(nil, domain.Verdict{}, WithoutColor(), WithCancelHooks(
		func() {},
		func() { cancelled++ },
	)).Resize(100, 30).StartRunning()

	model, _ = pressKey(t, model, "ctrl+c")
	model, cmd := pressKey(t, model, "y")
	if cancelled != 1 {
		t.Fatalf("cancel-all calls = %d, want 1", cancelled)
	}
	assertQuit(t, cmd)
}

func TestCancelledRunIsLabelled(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{
		StartedAt:  runStart,
		FinishedAt: runStart.Add(time.Second),
		Results:    []domain.Result{testResult("local.interfaces", "en0", domain.StatusPass)},
		Cancelled:  true,
	}})

	view := model.View()
	if !strings.Contains(view, "CANCELLED") {
		t.Fatal("cancelled run is not labelled")
	}
	if !strings.Contains(view, "pass 1") {
		t.Fatal("partial results are not shown")
	}
}

func TestHelpDocumentsCancelKeys(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	model, _ = pressKey(t, model, "?")
	view := model.View()
	for _, want := range []string{"x / X", "cancel check / baseline"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help is missing %q", want)
		}
	}
}
