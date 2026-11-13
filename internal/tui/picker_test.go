package tui

import (
	"strings"
	"testing"

	"github.com/gothchibjo/egress/internal/domain"
)

func pickerModel(t *testing.T, start func(presets []domain.Preset, custom []string)) Model {
	t.Helper()
	return fixtureModel(t, 100, 30, WithoutColor(), WithStartSelection(start))
}

func TestPickerListsPresetsByCategory(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")

	view := model.View()
	assertBlockShape(t, view, 100, 30)
	for _, want := range []string{
		"TARGETS", "0 selected",
		"VIDEO", "YouTube", "youtube.com",
		"MESSAGING", "Telegram", "telegram.org", "Discord", "discord.com",
		"DEVELOPER", "GitHub", "github.com",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("picker is missing %q:\n%s", want, view)
		}
	}

	model, _ = pressKey(t, model, "G")
	tail := model.View()
	for _, want := range []string{
		"Cloudflare DNS", "1.1.1.1", "Google DNS", "8.8.8.8",
		"Tor", "torproject.org", "Wikipedia", "wikipedia.org",
	} {
		if !strings.Contains(tail, want) {
			t.Fatalf("picker is missing %q after scrolling:\n%s", want, tail)
		}
	}
}

func TestPickerTogglesPresets(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")

	model, _ = pressKey(t, model, " ")
	if !strings.Contains(model.View(), "1 selected") {
		t.Fatalf("selection is not visible:\n%s", model.View())
	}
	if !strings.Contains(model.View(), "[x] YouTube") {
		t.Fatalf("checkbox is not checked:\n%s", model.View())
	}

	model, _ = pressKey(t, model, " ")
	if !strings.Contains(model.View(), "0 selected") {
		t.Fatalf("selection was not cleared:\n%s", model.View())
	}
	if !strings.Contains(model.View(), "[ ] YouTube") {
		t.Fatalf("checkbox is still checked:\n%s", model.View())
	}
}

func TestPickerFilterKeepsSelection(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")
	model, _ = pressKey(t, model, " ")

	model, _ = pressKey(t, model, "/")
	for _, key := range []string{"g", "i", "t"} {
		model, _ = pressKey(t, model, key)
	}
	view := model.View()
	if strings.Contains(view, "YouTube") {
		t.Fatalf("filter did not hide YouTube:\n%s", view)
	}
	if !strings.Contains(view, "GitHub") {
		t.Fatalf("filter did not keep GitHub:\n%s", view)
	}
	if !strings.Contains(view, "1 selected") {
		t.Fatalf("filter changed the selection count:\n%s", view)
	}

	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "/")
	model, _ = pressKey(t, model, "esc")
	if view := model.View(); !strings.Contains(view, "[x] YouTube") {
		t.Fatalf("clearing the filter lost the selection:\n%s", view)
	}
}

func TestPickerEmptySelectionDoesNotStart(t *testing.T) {
	started := 0
	model := pickerModel(t, func([]domain.Preset, []string) { started++ })
	model, _ = pressKey(t, model, "t")

	model, cmd := pressKey(t, model, "enter")
	if started != 0 {
		t.Fatalf("empty selection started %d runs", started)
	}
	if cmd == nil {
		t.Fatal("empty selection did not schedule the notice expiry")
	}
	if !strings.Contains(model.View(), "select at least one target") {
		t.Fatalf("empty selection is not explained:\n%s", model.View())
	}
}

func TestPickerStartsWithTheSelectedPresets(t *testing.T) {
	var gotPresets []domain.Preset
	var gotCustom []string
	model := pickerModel(t, func(presets []domain.Preset, custom []string) {
		gotPresets = presets
		gotCustom = custom
	})
	model, _ = pressKey(t, model, "t")

	model, _ = pressKey(t, model, " ")
	model, _ = pressKey(t, model, "j")
	model, _ = pressKey(t, model, " ")
	model, _ = pressKey(t, model, "enter")

	if len(gotPresets) != 2 {
		t.Fatalf("started with %d presets, want 2", len(gotPresets))
	}
	if gotPresets[0].ID != "youtube" || gotPresets[1].ID != "telegram" {
		t.Fatalf("started with %v", gotPresets)
	}
	if len(gotCustom) != 0 {
		t.Fatalf("unexpected custom targets: %v", gotCustom)
	}
}

func TestPickerCustomTargetInline(t *testing.T) {
	var gotCustom []string
	model := pickerModel(t, func(_ []domain.Preset, custom []string) { gotCustom = custom })
	model, _ = pressKey(t, model, "t")

	model, _ = pressKey(t, model, "c")
	for _, key := range []string{"e", "x", "a", "m", "p", "l", "e", ".", "n", "e", "t"} {
		model, _ = pressKey(t, model, key)
	}
	model, _ = pressKey(t, model, "enter")

	if !strings.Contains(model.View(), "1 selected") {
		t.Fatalf("custom target is not counted:\n%s", model.View())
	}
	if !strings.Contains(model.View(), "example.net") {
		t.Fatalf("custom target is not listed:\n%s", model.View())
	}

	model, _ = pressKey(t, model, "enter")
	if len(gotCustom) != 1 || gotCustom[0] != "example.net" {
		t.Fatalf("started with custom targets %v", gotCustom)
	}
}

func TestPickerCustomTargetCanBeRemoved(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")
	model, _ = pressKey(t, model, "c")
	for _, key := range []string{"a", ".", "t", "e", "s", "t"} {
		model, _ = pressKey(t, model, key)
	}
	model, _ = pressKey(t, model, "enter")
	if !strings.Contains(model.View(), "1 selected") {
		t.Fatalf("custom target is not counted:\n%s", model.View())
	}

	model, _ = pressKey(t, model, " ")
	if !strings.Contains(model.View(), "0 selected") {
		t.Fatalf("custom target was not removed:\n%s", model.View())
	}
}

func TestPickerEscReturnsWithoutRunning(t *testing.T) {
	started := 0
	model := pickerModel(t, func([]domain.Preset, []string) { started++ })
	model, _ = pressKey(t, model, "t")
	model, _ = pressKey(t, model, " ")

	model, _ = pressKey(t, model, "esc")
	if strings.Contains(model.View(), "TARGETS") {
		t.Fatal("esc did not leave the picker")
	}
	if started != 0 {
		t.Fatalf("esc started %d runs", started)
	}
}

func TestPickerFilterSwallowsActions(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")
	cursor := model.picker.cursor

	model, _ = pressKey(t, model, "/")
	model, cmd := pressKey(t, model, "q")
	if cmd != nil {
		t.Fatal("q inside the filter quit the program")
	}
	if model.picker.filter != "q" {
		t.Fatalf("filter = %q, want q", model.picker.filter)
	}

	model, _ = pressKey(t, model, "j")
	if model.picker.filter != "qj" {
		t.Fatalf("filter = %q, want qj", model.picker.filter)
	}
	if model.picker.cursor != cursor {
		t.Fatalf("filtering moved the cursor to %d", model.picker.cursor)
	}
}

func TestPickerEmptyFilterResult(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")
	model, _ = pressKey(t, model, "/")
	for _, key := range []string{"z", "z", "z"} {
		model, _ = pressKey(t, model, key)
	}

	if !strings.Contains(model.View(), "no targets match the filter") {
		t.Fatalf("empty filter result is not explained:\n%s", model.View())
	}
}

func TestPickerKeepsTerminalHeight(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {80, 24}, {60, 20}} {
		model := fixtureModel(t, size.width, size.height, WithoutColor())
		model, _ = pressKey(t, model, "t")
		assertBlockShape(t, model.View(), size.width, size.height)

		model, _ = pressKey(t, model, "/")
		model, _ = pressKey(t, model, "g")
		assertBlockShape(t, model.View(), size.width, size.height)
	}
}

func TestPickerHelpReturns(t *testing.T) {
	model := pickerModel(t, nil)
	model, _ = pressKey(t, model, "t")
	model, _ = pressKey(t, model, "?")
	if !strings.Contains(model.View(), "HELP") {
		t.Fatal("help did not open from the picker")
	}
	model, _ = pressKey(t, model, "esc")
	if !strings.Contains(model.View(), "TARGETS") {
		t.Fatal("help did not return to the picker")
	}
}

func TestDashboardAdvertisesThePicker(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {80, 24}, {60, 20}} {
		model := fixtureModel(t, size.width, size.height, WithoutColor())
		if view := model.View(); !strings.Contains(view, "t targets") {
			t.Fatalf("dashboard at %dx%d does not advertise the picker:\n%s", size.width, size.height, view)
		}
	}
}

func TestPickerStartWithoutRunView(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	model, _ = pressKey(t, model, "t")
	model, _ = pressKey(t, model, " ")
	model, _ = pressKey(t, model, "enter")

	if !strings.Contains(model.View(), "the run view is not available yet") {
		t.Fatalf("missing run-view notice:\n%s", model.View())
	}
}
