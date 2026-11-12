package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/domain"
)

func pressKey(t *testing.T, model Model, key string) (Model, tea.Cmd) {
	t.Helper()
	var msg tea.KeyMsg
	switch key {
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, cmd := model.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update(%q) returned %T", key, updated)
	}
	return next, cmd
}

func assertQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("command returned %T, want tea.QuitMsg", cmd())
	}
}

func TestKeyboardNavigation(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())

	model, _ = pressKey(t, model, "j")
	if model.Selected() != 1 {
		t.Fatalf("j selected %d, want 1", model.Selected())
	}
	model, _ = pressKey(t, model, "j")
	if model.Selected() != 2 {
		t.Fatalf("j selected %d, want 2", model.Selected())
	}
	model, _ = pressKey(t, model, "k")
	if model.Selected() != 1 {
		t.Fatalf("k selected %d, want 1", model.Selected())
	}
	model, _ = pressKey(t, model, "G")
	if model.Selected() != len(fixtureResults())-1 {
		t.Fatalf("G selected %d, want last", model.Selected())
	}
	model, _ = pressKey(t, model, "g")
	if model.Selected() != 0 {
		t.Fatalf("g selected %d, want 0", model.Selected())
	}
	model, _ = pressKey(t, model, "down")
	if model.Selected() != 1 {
		t.Fatalf("down selected %d, want 1", model.Selected())
	}
	model, _ = pressKey(t, model, "up")
	if model.Selected() != 0 {
		t.Fatalf("up selected %d, want 0", model.Selected())
	}
}

func TestEnterOpensDetailAndEscGoesBack(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor()).Select(3)
	model, cmd := pressKey(t, model, "enter")
	if cmd != nil {
		t.Fatal("opening detail returned a command")
	}
	view := model.View()
	if !strings.Contains(view, "DETAIL") || !strings.Contains(view, "icmp.anchor") {
		t.Fatal("detail page does not show the selected check")
	}
	if !strings.Contains(view, "esc back") {
		t.Fatal("detail footer has no back hint")
	}

	model, cmd = pressKey(t, model, "esc")
	if cmd != nil {
		t.Fatal("Esc returned a command")
	}
	view = model.View()
	if strings.Contains(view, "DETAIL") {
		t.Fatal("Esc did not leave the detail page")
	}
	if !strings.Contains(view, "LOCAL LINK") || !strings.Contains(view, "EVIDENCE") {
		t.Fatal("Esc did not return to the dashboard")
	}
	if model.Selected() != 3 {
		t.Fatalf("selection = %d after back, want 3", model.Selected())
	}
}

func TestEscOnlyNavigatesBack(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor()).Select(2)
	next, cmd := pressKey(t, model, "esc")
	if cmd != nil {
		t.Fatal("Esc on the dashboard returned a command")
	}
	if next.Selected() != 2 {
		t.Fatalf("Esc changed the selection to %d", next.Selected())
	}
	if view := next.View(); !strings.Contains(view, "LOCAL LINK") {
		t.Fatal("Esc left the dashboard")
	}
}

func TestHelpOverlayDocumentsEveryKey(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, cmd := pressKey(t, model, "?")
	if cmd != nil {
		t.Fatal("opening help returned a command")
	}
	view := model.View()
	for _, key := range []string{"↑", "↓", "k", "j", "g", "G", "enter", "esc", "q", "ctrl+c", "?"} {
		if !strings.Contains(view, key) {
			t.Fatalf("help does not document %q", key)
		}
	}
	for _, section := range []string{"NAVIGATION", "ACTIONS", "VERDICT", "healthy", "degraded", "blocked"} {
		if !strings.Contains(view, section) {
			t.Fatalf("help is missing %q", section)
		}
	}
	if !strings.Contains(view, "? close") {
		t.Fatal("help footer has no close hint")
	}

	model, cmd = pressKey(t, model, "?")
	if cmd != nil || strings.Contains(model.View(), "NAVIGATION") {
		t.Fatal("? did not close help")
	}
}

func TestHelpFitsMinimumTerminal(t *testing.T) {
	model := fixtureModel(t, 60, 20, WithoutColor())
	model, _ = pressKey(t, model, "?")

	view := model.View()
	assertBlockShape(t, view, 60, 20)
	for _, key := range []string{"enter", "ctrl+c", "r          run the baseline again"} {
		if !strings.Contains(view, key) {
			t.Fatalf("compact help lost %q", key)
		}
	}
	if !strings.Contains(view, "j/k scroll") {
		t.Fatal("compact help does not advertise scrolling")
	}

	model, _ = pressKey(t, model, "G")
	if !strings.Contains(model.View(), "blocked") {
		t.Fatal("scrolling does not reach the verdict semantics")
	}
	assertBlockShape(t, model.View(), 60, 20)
}

func TestHelpReturnsToPreviousPage(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor()).Select(3)
	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "?")
	model, _ = pressKey(t, model, "esc")

	view := model.View()
	if !strings.Contains(view, "DETAIL") || !strings.Contains(view, "icmp.anchor") {
		t.Fatal("closing help did not return to the detail page")
	}
}

func TestCtrlCRequiresConfirmation(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, cmd := pressKey(t, model, "ctrl+c")
	if cmd != nil {
		t.Fatal("Ctrl-C quit without confirmation")
	}
	if !strings.Contains(model.View(), "Quit egress?") {
		t.Fatal("Ctrl-C did not show the confirmation")
	}

	model, cmd = pressKey(t, model, "n")
	if cmd != nil {
		t.Fatal("n returned a command")
	}
	if strings.Contains(model.View(), "Quit egress?") {
		t.Fatal("n did not cancel the confirmation")
	}

	model, _ = pressKey(t, model, "ctrl+c")
	model, cmd = pressKey(t, model, "y")
	assertQuit(t, cmd)
}

func TestQCausesImmediateQuit(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	_, cmd := pressKey(t, model, "q")
	assertQuit(t, cmd)
}

func TestEnterWithoutResultsStaysOnDashboard(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30)
	model, cmd := pressKey(t, model, "enter")
	if cmd != nil {
		t.Fatal("enter returned a command")
	}
	if strings.Contains(model.View(), "DETAIL") {
		t.Fatal("detail opened without a selected result")
	}
}

func TestFooterHintsFollowContext(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	if view := model.View(); !strings.Contains(view, "enter open") || !strings.Contains(view, "? help") {
		t.Fatal("dashboard footer hints are missing")
	}

	model, _ = pressKey(t, model, "enter")
	if view := model.View(); !strings.Contains(view, "esc back") {
		t.Fatal("detail footer hints are missing")
	}

	model, _ = pressKey(t, model, "?")
	if view := model.View(); !strings.Contains(view, "? close") {
		t.Fatal("help footer hints are missing")
	}
}
