package tui

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/report"
)

var exportNamePattern = regexp.MustCompile(`^egress-[0-9]{8}-[0-9]{6}\.json$`)

type exportCall struct {
	path     string
	envelope report.Envelope
	replace  bool
}

func exportModel(t *testing.T, calls *[]exportCall, result error) Model {
	t.Helper()
	return fixtureModel(t, 100, 30,
		WithoutColor(),
		WithMode("unprivileged"),
		WithDestinations(destinationFixture(), nil),
		WithExporter(func(path string, envelope report.Envelope, replace bool) error {
			*calls = append(*calls, exportCall{path: path, envelope: envelope, replace: replace})
			if replace {
				return nil
			}
			return result
		}),
	)
}

func TestExportDefaultWritesTimestampedFile(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)

	model, _ = pressKey(t, model, "e")
	if len(calls) != 1 {
		t.Fatalf("exporter called %d times, want 1", len(calls))
	}
	if !exportNamePattern.MatchString(calls[0].path) {
		t.Fatalf("default path = %q", calls[0].path)
	}
	if calls[0].replace {
		t.Fatal("default export asked to replace a file")
	}
	if calls[0].envelope.Mode != "unprivileged" {
		t.Fatalf("envelope mode = %q", calls[0].envelope.Mode)
	}
	if calls[0].envelope.Verdict == nil {
		t.Fatal("envelope has no verdict")
	}
	if len(calls[0].envelope.Results) != len(model.results) {
		t.Fatalf("envelope carries %d results, want %d", len(calls[0].envelope.Results), len(model.results))
	}
	if len(calls[0].envelope.Destinations) == 0 {
		t.Fatal("envelope has no destinations")
	}

	if view := model.View(); !strings.Contains(view, "wrote "+calls[0].path) {
		t.Fatalf("dashboard does not show the written path:\n%s", view)
	}
}

func TestExportRefusesToOverwriteWithoutConfirmation(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, report.ErrExportExists)

	model, _ = pressKey(t, model, "e")
	if len(calls) != 1 || calls[0].replace {
		t.Fatalf("existing file was written without confirmation: %+v", calls)
	}
	if view := model.View(); !strings.Contains(view, "overwrite? y/n") {
		t.Fatalf("dashboard does not ask for confirmation:\n%s", view)
	}

	model, _ = pressKey(t, model, "n")
	if len(calls) != 1 {
		t.Fatalf("cancelling the confirmation wrote the file: %+v", calls)
	}
	if !strings.Contains(model.View(), "export cancelled") {
		t.Fatal("cancelled export is not reported")
	}
}

func TestExportOverwriteConfirmationWritesFile(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, report.ErrExportExists)

	model, _ = pressKey(t, model, "e")
	model, _ = pressKey(t, model, "y")
	if len(calls) != 2 {
		t.Fatalf("exporter called %d times, want 2", len(calls))
	}
	if !calls[1].replace {
		t.Fatal("confirmed overwrite did not replace the file")
	}
	if calls[1].path != calls[0].path {
		t.Fatalf("overwrite path = %q, want %q", calls[1].path, calls[0].path)
	}
	if !strings.Contains(model.View(), "wrote ") {
		t.Fatal("confirmed export is not reported")
	}
}

func TestExportPromptAcceptsAPath(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)

	model, _ = pressKey(t, model, "E")
	if view := model.View(); !strings.Contains(view, "export to: egress-") {
		t.Fatalf("path prompt did not open:\n%s", view)
	}
	model, _ = pressKey(t, model, "j")
	if !strings.HasSuffix(model.exportInput, "j") {
		t.Fatalf("typed rune did not reach the prompt: %q", model.exportInput)
	}
	model, _ = pressKey(t, model, "enter")
	if len(calls) != 1 {
		t.Fatalf("exporter called %d times, want 1", len(calls))
	}
	if !strings.HasSuffix(calls[0].path, "j") {
		t.Fatalf("exported path = %q, want the typed path", calls[0].path)
	}
	if model.exportPrompt {
		t.Fatal("prompt stayed open after a successful export")
	}
}

func TestExportPromptCancelDoesNotWrite(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)

	model, _ = pressKey(t, model, "E")
	model, _ = pressKey(t, model, "esc")
	if model.exportPrompt {
		t.Fatal("esc did not close the prompt")
	}
	if len(calls) != 0 {
		t.Fatalf("cancelled prompt wrote %d files", len(calls))
	}
}

func TestExportPromptSwallowsDashboardKeys(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, nil)

	model, _ = pressKey(t, model, "E")
	model, cmd := pressKey(t, model, "q")
	if cmd != nil {
		t.Fatal("q inside the export prompt quit the dashboard")
	}
	if model.view != viewDashboard {
		t.Fatalf("prompt key changed the page to %v", model.view)
	}
	if !strings.HasSuffix(model.exportInput, "q") {
		t.Fatalf("q did not reach the prompt: %q", model.exportInput)
	}
}

func TestExportWithoutResults(t *testing.T) {
	var calls []exportCall
	model := New(nil, domain.Verdict{}, WithoutColor(),
		WithExporter(func(path string, envelope report.Envelope, replace bool) error {
			calls = append(calls, exportCall{path: path, envelope: envelope, replace: replace})
			return nil
		}),
	).Resize(100, 30)

	model, _ = pressKey(t, model, "e")
	if len(calls) != 0 {
		t.Fatal("empty run was exported")
	}
	if !strings.Contains(model.View(), "nothing to export") {
		t.Fatal("empty run does not explain why nothing was written")
	}
}

func TestExportWorksForCancelledRuns(t *testing.T) {
	var calls []exportCall
	model := New(nil, domain.Verdict{}, WithoutColor(),
		WithMode("elevated"),
		WithExporter(func(path string, envelope report.Envelope, replace bool) error {
			calls = append(calls, exportCall{path: path, envelope: envelope, replace: replace})
			return nil
		}),
	).Resize(100, 30).StartRunning()

	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{
		StartedAt:  runStart,
		FinishedAt: runStart.Add(2 * time.Second),
		Cancelled:  true,
		Results:    []domain.Result{testResult("local.interfaces", "en0", domain.StatusPass)},
	}})

	model, _ = pressKey(t, model, "e")
	if len(calls) != 1 {
		t.Fatalf("cancelled run export calls = %d", len(calls))
	}
	if calls[0].envelope.Verdict == nil || calls[0].envelope.Verdict.Level != domain.VerdictHealthy {
		t.Fatalf("cancelled export verdict = %+v", calls[0].envelope.Verdict)
	}
	if calls[0].envelope.Mode != "elevated" {
		t.Fatalf("cancelled export mode = %q", calls[0].envelope.Mode)
	}
	if !calls[0].envelope.Cancelled {
		t.Fatal("cancelled export is not marked as cancelled")
	}
}

func TestExportFailureIsReported(t *testing.T) {
	var calls []exportCall
	model := exportModel(t, &calls, errors.New("disk full"))

	model, _ = pressKey(t, model, "e")
	if !strings.Contains(model.View(), "export failed: disk full") {
		t.Fatalf("export failure is not reported:\n%s", model.View())
	}
}

func TestExportKeepsTerminalHeight(t *testing.T) {
	var calls []exportCall
	for _, size := range []struct{ width, height int }{{100, 30}, {80, 24}, {60, 20}} {
		model := fixtureModel(t, size.width, size.height, WithoutColor(),
			WithExporter(func(path string, envelope report.Envelope, replace bool) error {
				calls = append(calls, exportCall{path: path, envelope: envelope, replace: replace})
				return nil
			}),
		)
		model, _ = pressKey(t, model, "e")
		assertBlockShape(t, model.View(), size.width, size.height)
	}
}
