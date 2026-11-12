package tui

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/report"
)

// TestWalkthroughTranscript prints a deterministic walkthrough of every page.
// It is skipped by default and regenerated with EGRESS_TRANSCRIPT=1; the output
// is stored as verification evidence.
func TestWalkthroughTranscript(t *testing.T) {
	if os.Getenv("EGRESS_TRANSCRIPT") == "" {
		t.Skip("set EGRESS_TRANSCRIPT=1 to print the walkthrough transcript")
	}

	section := func(title string, model Model) {
		fmt.Printf("\n=== %s ===\n%s\n", title, model.View())
	}

	start := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

	// 1. Startup before the first result.
	running := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	running = applyMessage(t, running, RunProgressMsg{Progress: app.Progress{
		CheckID: "tcp.anchor.example-com.53",
		Index:   14,
		Total:   17,
		Elapsed: 1100 * time.Millisecond,
	}})
	section("startup, running the baseline (100x30)", running)

	// 2. Completed run with a failure and the classification band.
	results := []domain.Result{
		testResult("local.interfaces", "local host", domain.StatusPass),
		testResult("gateway.reach", "192.168.1.1", domain.StatusPass),
		failureResult(domain.StatusFail, "no resolver answered: resolver.192.0.2.53: timeout", ""),
		testResult("tcp.anchor.cloudflare-dns.443", "1.1.1.1:443", domain.StatusPass),
		testResult("mtu.pmtu", "example.com", domain.StatusSkipped),
	}
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	model := New(results, verdict, WithoutColor()).Resize(100, 30)
	model = applyMessage(t, model, RunCompleteMsg{Run: app.Run{
		StartedAt:  start,
		FinishedAt: start.Add(2500 * time.Millisecond),
		Results:    results,
	}})
	section("completed baseline (100x30)", model)

	// 3. Detail page of the failing check, then scrolled to the end.
	model, _ = pressKey(t, model, "n")
	model, _ = pressKey(t, model, "enter")
	section("detail of the failing check (100x30)", model)
	model, _ = pressKey(t, model, "G")
	section("detail scrolled to the end (100x30)", model)

	// 4. Help overlay and its scroll.
	model, _ = pressKey(t, model, "?")
	section("help overlay (100x30)", model)
	model, _ = pressKey(t, model, "G")
	section("help scrolled to the end (100x30)", model)
	model, _ = pressKey(t, model, "esc")

	// 5. Destinations and context pages.
	model, _ = pressKey(t, model, "d")
	section("outbound destinations (100x30)", model)
	model, _ = pressKey(t, model, "esc")

	contextModel := fixtureModel(t, 100, 30, WithoutColor(),
		WithContext(contextFixture(), nil))
	contextModel, _ = pressKey(t, contextModel, "c")
	section("proxy and tunnel context (100x30)", contextModel)

	// 6. Export prompt and the status message.
	exportModel := fixtureModel(t, 100, 30, WithoutColor(),
		WithExporter(func(string, report.Envelope, bool) error { return nil }))
	exportModel, _ = pressKey(t, exportModel, "E")
	section("export path prompt (100x30)", exportModel)
	exportModel, _ = pressKey(t, exportModel, "enter")
	section("export status message (100x30)", exportModel)

	// 7. Offline classification.
	offline := New(nil, domain.Verdict{}, WithoutColor()).Resize(100, 30).StartRunning()
	offline = applyMessage(t, offline, RunCompleteMsg{Run: app.Run{
		StartedAt:  start,
		FinishedAt: start.Add(time.Second),
		Results: []domain.Result{
			testResult("local.interfaces", "local host", domain.StatusFail),
			testResult("gateway.reach", "default gateway", domain.StatusSkipped),
		},
	}})
	section("offline: no route (100x30)", offline)

	// 8. Compact layout.
	compact := fixtureModel(t, 60, 20, WithoutColor(), WithMode("unprivileged"))
	section("compact layout (60x20)", compact)

	// 9. Below the minimum size.
	tooSmall := fixtureModel(t, 50, 15, WithoutColor())
	section("below the minimum size (50x15)", tooSmall)
}
