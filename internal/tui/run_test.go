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
