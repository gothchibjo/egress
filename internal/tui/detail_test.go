package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gothchibjo/egress/internal/domain"
)

func longEvidence() domain.Result {
	result := testResult("local.interfaces", "local host", domain.StatusPass)
	measurements := make([]domain.Measurement, 0, 40)
	for i := 0; i < 40; i++ {
		measurements = append(measurements, domain.Measurement{
			Name:  fmt.Sprintf("interface.en%d.addresses", i),
			Value: "192.0.2.10/24, fe80::1/64",
		})
	}
	result.Measurements = measurements
	return result
}

func longEvidenceModel(t *testing.T, width, height int) Model {
	t.Helper()
	results := []domain.Result{longEvidence(), testResult("gateway.reach", "192.168.1.1", domain.StatusPass)}
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	return New(results, verdict, WithoutColor()).Resize(width, height)
}

func TestDashboardMarksTruncatedEvidence(t *testing.T) {
	model := longEvidenceModel(t, 100, 30)
	if view := model.View(); !strings.Contains(view, "enter for the full evidence") {
		t.Fatal("evidence pane does not mark the truncated content")
	}
}

func TestDetailScrollsLongEvidence(t *testing.T) {
	model := longEvidenceModel(t, 100, 30)
	model, _ = pressKey(t, model, "enter")

	max := model.detailMaxOffset()
	if max == 0 {
		t.Fatal("test requires evidence longer than the terminal")
	}
	total := len(model.detailLines(100))
	if view := model.View(); !strings.Contains(view, fmt.Sprintf("1-%d/%d", model.detailBodyHeight(), total)) {
		t.Fatalf("detail header does not show the visible range:\n%s", view)
	}

	model, _ = pressKey(t, model, "j")
	if model.detailOffset != 1 {
		t.Fatalf("j scrolled to %d, want 1", model.detailOffset)
	}
	if model.Selected() != 0 {
		t.Fatalf("scrolling changed the selected check to %d", model.Selected())
	}

	model, _ = pressKey(t, model, "G")
	if model.detailOffset != max {
		t.Fatalf("G scrolled to %d, want %d", model.detailOffset, max)
	}
	if view := model.View(); !strings.Contains(view, "interface.en39.addresses") {
		t.Fatal("the end of the evidence is not reachable")
	}

	model, _ = pressKey(t, model, "g")
	if model.detailOffset != 0 {
		t.Fatalf("g scrolled to %d, want 0", model.detailOffset)
	}

	model, _ = pressKey(t, model, "k")
	if model.detailOffset != 0 {
		t.Fatalf("k scrolled past the top: %d", model.detailOffset)
	}
}

func TestDetailPageScrollsByPage(t *testing.T) {
	model := longEvidenceModel(t, 100, 30)
	model, _ = pressKey(t, model, "enter")

	page := model.detailBodyHeight()
	want := page
	if max := model.detailMaxOffset(); want > max {
		want = max
	}
	model, _ = pressKey(t, model, "pgdown")
	if model.detailOffset != want {
		t.Fatalf("pgdown scrolled to %d, want %d", model.detailOffset, want)
	}
	model, _ = pressKey(t, model, "pgup")
	if model.detailOffset != 0 {
		t.Fatalf("pgup scrolled to %d, want 0", model.detailOffset)
	}
}

func TestDetailSwitchesChecksWithHL(t *testing.T) {
	model := longEvidenceModel(t, 100, 30)
	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "j")
	if model.detailOffset == 0 {
		t.Fatal("test requires a scrolled detail page")
	}

	model, _ = pressKey(t, model, "l")
	if model.Selected() != 1 {
		t.Fatalf("l selected %d, want 1", model.Selected())
	}
	if model.detailOffset != 0 {
		t.Fatalf("switching checks kept the old scroll offset %d", model.detailOffset)
	}

	model, _ = pressKey(t, model, "h")
	if model.Selected() != 0 {
		t.Fatalf("h selected %d, want 0", model.Selected())
	}
}

func TestDetailKeepsTerminalHeight(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {80, 24}, {60, 20}} {
		model := longEvidenceModel(t, size.width, size.height)
		model, _ = pressKey(t, model, "enter")
		assertBlockShape(t, model.View(), size.width, size.height)

		model, _ = pressKey(t, model, "G")
		assertBlockShape(t, model.View(), size.width, size.height)
	}
}
