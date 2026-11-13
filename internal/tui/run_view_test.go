package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

func runTargets() []domain.ProbeTarget {
	return []domain.ProbeTarget{
		{ID: "alpha", Label: "Alpha", Host: "alpha.example", Ports: []int{443}},
		{ID: "beta", Label: "Beta", Host: "beta.example", Ports: []int{443}},
	}
}

func runStep(id string, kind domain.StepKind, status domain.Status, durationMS int64, measurements ...domain.Measurement) domain.StepResult {
	return domain.StepResult{
		ID:           id,
		Kind:         kind,
		Target:       "alpha.example",
		StartedAt:    time.Unix(0, 0),
		DurationMS:   durationMS,
		Status:       status,
		Measurements: measurements,
	}
}

func alphaSteps() []domain.StepResult {
	trace := runStep("trace", domain.StepTrace, domain.StatusPass, 900)
	trace.Hops = []domain.Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3, RTTMedianMS: 12},
		{TTL: 2, Address: "203.0.113.10", Sent: 3, Received: 3, RTTMedianMS: 35},
	}
	return []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 40, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
		runStep("tcp.443", domain.StepTCP, domain.StatusPass, 30, domain.Measurement{Name: "outcome", Value: "connected"}),
		runStep("icmp", domain.StepICMP, domain.StatusPass, 20, domain.Measurement{Name: "address", Value: "203.0.113.10"}),
		trace,
	}
}

func runModel(t *testing.T, width, height int, opts ...Option) Model {
	t.Helper()
	model := fixtureModel(t, width, height, append([]Option{WithoutColor()}, opts...)...)
	return model.startRun(runTargets())
}

func completeRun(t *testing.T, model Model) Model {
	t.Helper()
	targets := runTargets()
	return applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: alphaSteps()},
		{Target: targets[1], State: app.TargetCancelled, Steps: []domain.StepResult{runStep("dns", domain.StepDNS, domain.StatusSkipped, 0)}},
	}}})
}

func TestRunViewShowsTargetTabs(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))

	view := model.View()
	assertBlockShape(t, view, 100, 30)
	for _, want := range []string{"RUN", "1/2 targets", "Alpha", "Beta", "ok", "x"} {
		if !strings.Contains(view, want) {
			t.Fatalf("run view is missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewWideShowsLadderAndHops(t *testing.T) {
	model := completeRun(t, runModel(t, 120, 40))
	view := model.View()

	for _, want := range []string{"LADDER", "HOPS", "TTL", "203.0.113.10", "dns", "tcp.443"} {
		if !strings.Contains(view, want) {
			t.Fatalf("wide run view is missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewCompactSwitchesPanes(t *testing.T) {
	model := completeRun(t, runModel(t, 60, 20))
	view := model.View()
	assertBlockShape(t, view, 60, 20)
	if !strings.Contains(view, "LADDER") {
		t.Fatalf("compact view does not start on the ladder:\n%s", view)
	}
	if strings.Contains(view, "TTL") {
		t.Fatalf("compact view shows hops and the ladder together:\n%s", view)
	}

	model, _ = pressKey(t, model, "2")
	if view := model.View(); !strings.Contains(view, "HOPS") || strings.Contains(view, "LADDER") {
		t.Fatalf("2 did not switch to hops:\n%s", view)
	}
	model, _ = pressKey(t, model, "3")
	if view := model.View(); !strings.Contains(view, "unknown") {
		t.Fatalf("3 did not switch to the verdict:\n%s", view)
	}
}

func TestRunViewVerdictShowsLabelledEvidence(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))
	model, _ = pressKey(t, model, "3")

	view := model.View()
	for _, want := range []string{"EVIDENCE", "[active]", "REJECTED", "CAVEAT", "Single vantage point"} {
		if !strings.Contains(view, want) {
			t.Fatalf("verdict is missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewSwitchesTargets(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))

	model, _ = pressKey(t, model, "]")
	if view := model.View(); !strings.Contains(view, "2/2 targets") {
		t.Fatalf("] did not move to the second target:\n%s", view)
	}
	if !strings.Contains(model.View(), "beta") && !strings.Contains(model.View(), "Beta") {
		t.Fatalf("second target is not shown:\n%s", model.View())
	}

	model, _ = pressKey(t, model, "[")
	if view := model.View(); !strings.Contains(view, "1/2 targets") {
		t.Fatalf("[ did not return to the first target:\n%s", view)
	}
}

func TestRunViewCancelsTargets(t *testing.T) {
	var cancelledTarget, cancelledAll string
	model := runModel(t, 100, 30, WithProbeCancels(
		func(id string) bool { cancelledTarget = id; return true },
		func() bool { cancelledAll = "all"; return true },
	))
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "alpha", Index: 0, Total: 2}})

	model, _ = pressKey(t, model, "x")
	if cancelledTarget != "alpha" {
		t.Fatalf("x cancelled %q", cancelledTarget)
	}
	model, _ = pressKey(t, model, "X")
	if cancelledAll != "all" {
		t.Fatalf("X cancelled %q", cancelledAll)
	}
}

func TestRunViewEscNeverCancels(t *testing.T) {
	cancelled := 0
	model := runModel(t, 100, 30, WithProbeCancels(
		func(string) bool { cancelled++; return true },
		func() bool { cancelled++; return true },
	))
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "alpha", Index: 0, Total: 2}})

	model, _ = pressKey(t, model, "esc")
	if cancelled != 0 {
		t.Fatalf("esc cancelled %d targets", cancelled)
	}
	if strings.Contains(model.View(), "RUN") {
		t.Fatal("esc did not leave the run view")
	}
}

func TestRunViewKeepsTerminalHeight(t *testing.T) {
	for _, size := range []struct{ width, height int }{{100, 30}, {80, 24}, {60, 20}} {
		model := completeRun(t, runModel(t, size.width, size.height))
		assertBlockShape(t, model.View(), size.width, size.height)
	}
}

func TestRunViewHelpReturns(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))
	model, _ = pressKey(t, model, "?")
	if !strings.Contains(model.View(), "HELP") {
		t.Fatal("help did not open from the run view")
	}
	model, _ = pressKey(t, model, "esc")
	if !strings.Contains(model.View(), "RUN") {
		t.Fatal("help did not return to the run view")
	}
}

func TestRunViewShowsLiveProgress(t *testing.T) {
	model := runModel(t, 100, 30)
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "alpha", Index: 0, Total: 2}})
	model = applyMessage(t, model, ProbeStepMsg{TargetID: "alpha", Step: runStep("dns", domain.StepDNS, domain.StatusPass, 12)})

	view := model.View()
	if !strings.Contains(view, "dns") {
		t.Fatalf("live step is not shown:\n%s", view)
	}
	if !strings.Contains(view, "...") {
		t.Fatalf("running target marker is missing:\n%s", view)
	}
}

func TestRunViewScrolling(t *testing.T) {
	steps := make([]domain.StepResult, 0, 40)
	for i := 0; i < 40; i++ {
		steps = append(steps, runStep(fmt.Sprintf("step-%02d", i), domain.StepTCP, domain.StatusPass, 5))
	}
	targets := runTargets()
	model := runModel(t, 60, 20)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})

	model, _ = pressKey(t, model, "G")
	if !strings.Contains(model.View(), "step-39") {
		t.Fatalf("G did not scroll to the last step:\n%s", model.View())
	}
	model, _ = pressKey(t, model, "g")
	if !strings.Contains(model.View(), "step-00") {
		t.Fatalf("g did not scroll to the first step:\n%s", model.View())
	}
}

func TestRunViewResizeKeepsState(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))
	model, _ = pressKey(t, model, "]")
	model, _ = pressKey(t, model, "2")

	model = model.Resize(60, 20)
	if view := model.View(); !strings.Contains(view, "2/2 targets") || !strings.Contains(view, "HOPS") {
		t.Fatalf("resize lost the target or pane:\n%s", view)
	}
	assertBlockShape(t, model.View(), 60, 20)
}
