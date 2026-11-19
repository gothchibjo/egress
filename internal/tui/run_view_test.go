package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

func forceColor(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

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
	trace := runStep("trace", domain.StepTrace, domain.StatusPass, 900, domain.Measurement{Name: "method", Value: "udp"})
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
	for _, want := range []string{"RUN", "1/2 targets", "TARGETS", "Alpha", "Beta", "✓", "x"} {
		if !strings.Contains(view, want) {
			t.Fatalf("run view is missing %q:\n%s", want, view)
		}
	}
}

func TestRunViewWideShowsLadderAndHops(t *testing.T) {
	model := completeRun(t, runModel(t, 120, 40))
	view := model.View()

	for _, want := range []string{"LADDER", "HOPS (udp)", "TTL", "203.0.113.10", "path complete at hop 2", "dns", "tcp.443"} {
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
	if view := model.View(); !strings.Contains(view, "no blockage detected") {
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
	if !strings.Contains(view, "…") {
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

	model, _ = pressKey(t, model, "enter")
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

func TestRunViewShowsPlannedAndRunningSteps(t *testing.T) {
	model := runModel(t, 120, 40)
	plan := []domain.StepPlan{
		{ID: "dns", Kind: domain.StepDNS},
		{ID: "tcp.443", Kind: domain.StepTCP},
		{ID: "trace", Kind: domain.StepTrace},
	}
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "alpha", Index: 0, Total: 2, Plan: plan}})
	model = applyMessage(t, model, ProbeStepMsg{TargetID: "alpha", Step: runStep("dns", domain.StepDNS, domain.StatusPass, 12)})
	model = applyMessage(t, model, ProbeStepStartMsg{TargetID: "alpha", Plan: plan[2]})

	view := model.View()
	if !strings.Contains(view, "TARGETS") {
		t.Fatalf("target column is missing:\n%s", view)
	}
	if !strings.Contains(view, "… trace") {
		t.Fatalf("running step is not next to the target:\n%s", view)
	}
	if !strings.Contains(view, "trace") || !strings.Contains(view, "running") {
		t.Fatalf("ladder does not show the running step:\n%s", view)
	}
	if !strings.Contains(view, "tcp.443") {
		t.Fatalf("ladder does not show the queued step:\n%s", view)
	}
}

func TestRunViewMarksTargetOutcomes(t *testing.T) {
	targets := runTargets()
	failed := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10),
		runStep("tcp.443", domain.StepTCP, domain.StatusFail, 20, domain.Measurement{Name: "outcome", Value: "timeout"}),
	}
	model := runModel(t, 120, 40)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: failed},
		{Target: targets[1], State: app.TargetDone, Steps: alphaSteps()},
	}}})

	view := model.View()
	if !strings.Contains(view, "✗ Alpha") {
		t.Fatalf("failed target lacks a failure marker:\n%s", view)
	}
	if !strings.Contains(view, "✓ Beta") && !strings.Contains(view, "✓ Alpha") {
		t.Fatalf("passing target lacks a pass marker:\n%s", view)
	}
}

func TestRunViewTargetColumnAtCompact(t *testing.T) {
	model := completeRun(t, runModel(t, 60, 20))
	view := model.View()
	assertBlockShape(t, view, 60, 20)
	if !strings.Contains(view, "TARGETS") || !strings.Contains(view, "Alpha") {
		t.Fatalf("compact view lost the target column:\n%s", view)
	}
}

func TestRunViewArrowsSwitchTargets(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))

	model, _ = pressKey(t, model, "j")
	if view := model.View(); !strings.Contains(view, "2/2 targets") {
		t.Fatalf("j did not move to the next target:\n%s", view)
	}
	model, _ = pressKey(t, model, "k")
	if view := model.View(); !strings.Contains(view, "1/2 targets") {
		t.Fatalf("k did not move back:\n%s", view)
	}
}

func TestRunViewFocusMode(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))

	model, _ = pressKey(t, model, "enter")
	if !strings.Contains(model.View(), "[scroll]") {
		t.Fatalf("enter did not focus the pane:\n%s", model.View())
	}
	model, _ = pressKey(t, model, "j")
	if view := model.View(); !strings.Contains(view, "1/2 targets") {
		t.Fatalf("scrolling changed the target:\n%s", view)
	}

	model, _ = pressKey(t, model, "esc")
	if strings.Contains(model.View(), "[scroll]") {
		t.Fatal("esc did not leave the focus")
	}
	if strings.Contains(model.View(), "RUN") == false {
		t.Fatal("esc left the run view while focus was active")
	}
	model, _ = pressKey(t, model, "esc")
	if strings.Contains(model.View(), "RUN") {
		t.Fatal("second esc did not leave the run view")
	}
}

func TestRunViewTargetSwitchInsideFocus(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))
	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "l")
	if view := model.View(); !strings.Contains(view, "2/2 targets") {
		t.Fatalf("l did not switch target inside focus:\n%s", view)
	}
	if !strings.Contains(model.View(), "[scroll]") {
		t.Fatal("switching target dropped the focus")
	}
}

func TestRunViewAutoScrollsToTheRunningStep(t *testing.T) {
	plan := make([]domain.StepPlan, 0, 30)
	for i := 0; i < 30; i++ {
		plan = append(plan, domain.StepPlan{ID: fmt.Sprintf("step-%02d", i), Kind: domain.StepTCP})
	}
	model := runModel(t, 100, 30)
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "alpha", Index: 0, Total: 2, Plan: plan}})
	model = applyMessage(t, model, ProbeStepStartMsg{TargetID: "alpha", Plan: plan[25]})

	if view := model.View(); !strings.Contains(view, "step-25") {
		t.Fatalf("the running step is outside the window:\n%s", view)
	}
}

func TestRunViewMarksATargetDoneBeforeTheSessionEnds(t *testing.T) {
	model := runModel(t, 120, 40)
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "alpha", Index: 0, Total: 2}})
	model = applyMessage(t, model, ProbeProgressMsg{Progress: app.ProbeProgress{TargetID: "beta", Index: 1, Total: 2}})
	model = applyMessage(t, model, ProbeTargetDoneMsg{TargetID: "alpha", State: app.TargetDone, Steps: alphaSteps()})

	view := model.View()
	if !strings.Contains(view, "✓ Alpha") {
		t.Fatalf("finished target lacks its outcome marker:\n%s", view)
	}
	if !strings.Contains(view, "… Beta") {
		t.Fatalf("running target lost its running marker:\n%s", view)
	}
}

func TestRunViewGGJumpsTargets(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))

	model, _ = pressKey(t, model, "G")
	if view := model.View(); !strings.Contains(view, "2/2 targets") {
		t.Fatalf("G did not jump to the last target:\n%s", view)
	}
	model, _ = pressKey(t, model, "g")
	if view := model.View(); !strings.Contains(view, "1/2 targets") {
		t.Fatalf("g did not jump to the first target:\n%s", view)
	}
	if strings.Contains(model.View(), "[scroll]") {
		t.Fatal("g/G should not focus a pane")
	}
}

func TestRunViewProblemJumps(t *testing.T) {
	targets := runTargets()
	failed := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10),
		runStep("tcp.443", domain.StepTCP, domain.StatusFail, 20, domain.Measurement{Name: "outcome", Value: "timeout"}),
	}
	warned := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusWarn, 10, domain.Measurement{Name: "comparison", Value: "differ"}),
	}
	model := runModel(t, 100, 30)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: failed},
		{Target: targets[1], State: app.TargetDone, Steps: warned},
	}}})

	model, _ = pressKey(t, model, "n")
	if view := model.View(); !strings.Contains(view, "2/2 targets") {
		t.Fatalf("n did not move to the next problem target:\n%s", view)
	}
	model, _ = pressKey(t, model, "n")
	if view := model.View(); !strings.Contains(view, "1/2 targets") {
		t.Fatalf("n did not wrap to the first problem target:\n%s", view)
	}
	model, _ = pressKey(t, model, "p")
	if view := model.View(); !strings.Contains(view, "2/2 targets") {
		t.Fatalf("p did not move back to the last problem target:\n%s", view)
	}
}

func TestRunViewProblemJumpWithoutProblems(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))
	model, _ = pressKey(t, model, "n")
	if !strings.Contains(model.View(), "no failed or warning targets") {
		t.Fatalf("empty problem jump is not explained:\n%s", model.View())
	}
}

func TestRunViewWrapsLongStepDiagnostics(t *testing.T) {
	long := strings.Repeat("path discovery truncated because the trace budget expired; ", 3) + "end-of-diagnostic"
	steps := []domain.StepResult{
		runStep("trace", domain.StepTrace, domain.StatusWarn, 20000,
			domain.Measurement{Name: "method", Value: "udp"}),
	}
	steps[0].Diagnostic = long
	targets := runTargets()
	model := runModel(t, 100, 30)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})

	if view := model.View(); !strings.Contains(view, "end-of-diagnostic") {
		t.Fatalf("long diagnostic is clipped:\n%s", view)
	}
}

func TestRunViewWrapsLongVerdictLines(t *testing.T) {
	long := strings.Repeat("the resolver answered with a differing record; ", 4) + "verdict-tail"
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusWarn, 10,
			domain.Measurement{Name: "comparison", Value: "differ"},
			domain.Measurement{Name: "system.answers", Value: "203.0.113.10"},
			domain.Measurement{Name: "doh.answers", Value: "203.0.113.10"}),
	}
	steps[0].Diagnostic = long
	targets := runTargets()
	model := runModel(t, 100, 30)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})

	model, _ = pressKey(t, model, "3")
	model, _ = pressKey(t, model, "enter")
	model, _ = pressKey(t, model, "G")
	if view := model.View(); !strings.Contains(view, "verdict-tail") {
		t.Fatalf("long verdict evidence is clipped:\n%s", view)
	}
}

func TestRunViewWrapsLongHopAddresses(t *testing.T) {
	address := "2a00:1450:400f:805::200e:1234:5678:9abc"
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusPass, 500, domain.Measurement{Name: "method", Value: "udp"})
	trace.Hops = []domain.Hop{{TTL: 1, Address: address, Sent: 3, Received: 3, RTTMedianMS: 20}}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 60, 20)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})
	model, _ = pressKey(t, model, "2")

	joined := strings.ReplaceAll(strings.Join(strings.Fields(model.View()), ""), "│", "")
	want := strings.ReplaceAll(address, " ", "")
	if !strings.Contains(joined, want) {
		t.Fatalf("long hop address is clipped:\n%s", model.View())
	}
}

func TestRunViewHidesUnknownForCleanTargets(t *testing.T) {
	model := completeRun(t, runModel(t, 100, 30))
	model, _ = pressKey(t, model, "3")
	view := model.View()

	if strings.Contains(view, "unknown  (low)") {
		t.Fatalf("clean target is labelled unknown:\n%s", view)
	}
	if !strings.Contains(view, "no blockage detected") {
		t.Fatalf("clean target has no explicit outcome:\n%s", view)
	}
}

func TestRunViewExplainsATunneledPath(t *testing.T) {
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 400, domain.Measurement{Name: "system.answers", Value: "87.120.36.68"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusWarn, 15000, domain.Measurement{Name: "method", Value: "udp"})
	trace.Diagnostic = "no hop answered; ICMP may be filtered"
	trace.Hops = []domain.Hop{
		{TTL: 1, Sent: 3},
		{TTL: 2, Sent: 3},
		{TTL: 3, Sent: 3},
	}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 120, 40)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})

	view := model.View()
	if !strings.Contains(view, "no hop answered") {
		t.Fatalf("tunneled path is not explained:\n%s", view)
	}
	if strings.Contains(view, "100%") {
		t.Fatalf("tunneled path is drawn as broken hops:\n%s", view)
	}
	if !strings.Contains(view, "87.120.36.68") {
		t.Fatalf("traced address is missing:\n%s", view)
	}
}

func TestRunViewShowsEveryRouterAtAHop(t *testing.T) {
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusPass, 500, domain.Measurement{Name: "method", Value: "udp"})
	trace.Hops = []domain.Hop{{
		TTL: 5, Address: "109.206.242.94",
		Addresses: []string{"109.206.242.94", "109.206.242.60"},
		Sent:      3, Received: 3, RTTMedianMS: 20,
	}}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 160, 40)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})
	model, _ = pressKey(t, model, "2")

	view := model.View()
	if !strings.Contains(view, "109.206.242.94") || !strings.Contains(view, "109.206.242.60") {
		t.Fatalf("a hop with two routers shows only one:\n%s", view)
	}
}

func TestRunViewTrimsTrailingSilentHops(t *testing.T) {
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusWarn, 500, domain.Measurement{Name: "method", Value: "udp"})
	trace.Hops = []domain.Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3},
		{TTL: 2, Address: "198.51.100.1", Sent: 3, Received: 3},
		{TTL: 3, Sent: 3},
		{TTL: 4, Sent: 3},
		{TTL: 5, Sent: 3},
	}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 160, 40)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})
	model, _ = pressKey(t, model, "2")

	view := model.View()
	if strings.Contains(view, "5  *") || strings.Contains(view, "4  *") {
		t.Fatalf("trailing silent hops are still listed:\n%s", view)
	}
	if !strings.Contains(view, "3  *") {
		t.Fatalf("the first silent hop is not shown:\n%s", view)
	}

	// When a transport step reaches the target, the trailing star is dropped.
	reached := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
		runStep("tcp.443", domain.StepTCP, domain.StatusPass, 20, domain.Measurement{Name: "outcome", Value: "connected"}),
		trace,
	}
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: reached},
		{Target: targets[1], State: app.TargetDone},
	}}})
	view = model.View()
	if strings.Contains(view, "3  *") {
		t.Fatalf("a reachable target still shows a trailing star:\n%s", view)
	}
}

func TestRunViewMarksTheTargetAndLossyHops(t *testing.T) {
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
		runStep("tcp.443", domain.StepTCP, domain.StatusPass, 20, domain.Measurement{Name: "outcome", Value: "connected"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusPass, 500,
		domain.Measurement{Name: "method", Value: "icmp"},
		domain.Measurement{Name: "address", Value: "203.0.113.10"})
	trace.Hops = []domain.Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3, RTTMedianMS: 10},
		{TTL: 2, Address: "198.51.100.1", Sent: 3, Received: 2, RTTMedianMS: 20},
		{TTL: 3, Address: "203.0.113.10", Sent: 3, Received: 3, RTTMedianMS: 30},
	}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 160, 40)
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})
	model, _ = pressKey(t, model, "2")

	view := model.View()
	if !strings.Contains(view, "→   3") {
		t.Fatalf("the target hop is not marked:\n%s", view)
	}
	if !strings.Contains(view, "!   2") {
		t.Fatalf("a hop with partial loss is not marked:\n%s", view)
	}
	if !strings.Contains(view, "path complete at hop 3") {
		t.Fatalf("the localization is missing:\n%s", view)
	}
}

func TestRunViewNarrowStacksLadderAboveHops(t *testing.T) {
	model := completeRun(t, runModel(t, 82, 28))
	view := model.View()
	assertBlockShape(t, view, 82, 28)

	ladderLine, hopsLine := -1, -1
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "LADDER") {
			ladderLine = i
		}
		if strings.Contains(line, "HOPS (") {
			hopsLine = i
		}
		if strings.Contains(line, "LADDER") && strings.Contains(line, "HOPS (") {
			t.Fatalf("narrow view still draws three columns:\n%s", view)
		}
	}
	if ladderLine < 0 || hopsLine < 0 {
		t.Fatalf("narrow view lost a pane:\n%s", view)
	}
	if ladderLine > hopsLine {
		t.Fatalf("hops are not under the ladder:\n%s", view)
	}
}

func TestRunViewWideKeepsThreeColumns(t *testing.T) {
	model := completeRun(t, runModel(t, 130, 40))
	view := model.View()

	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "LADDER") && strings.Contains(line, "HOPS (") {
			return
		}
	}
	t.Fatalf("wide view does not draw three columns:\n%s", view)
}

func TestRunViewRerunsProbes(t *testing.T) {
	var restarted []domain.ProbeTarget
	model := completeRun(t, runModel(t, 120, 40, WithProbeRestart(func(targets []domain.ProbeTarget) {
		restarted = targets
	})))

	model, _ = pressKey(t, model, "r")
	if len(restarted) != len(model.run.targets) || len(restarted) == 0 {
		t.Fatalf("restart received %d targets, want %d", len(restarted), len(model.run.targets))
	}
	if !model.run.running {
		t.Fatal("the rerun did not reset the run state")
	}
	if model.run.targets[0].state != app.TargetPending || len(model.run.targets[0].steps) != 0 {
		t.Fatalf("the rerun kept the previous result: %+v", model.run.targets[0])
	}
}

func TestRunViewColoursHopRows(t *testing.T) {
	forceColor(t)
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
		runStep("tcp.443", domain.StepTCP, domain.StatusPass, 20, domain.Measurement{Name: "outcome", Value: "connected"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusPass, 500,
		domain.Measurement{Name: "method", Value: "udp"},
		domain.Measurement{Name: "address", Value: "203.0.113.10"})
	trace.Hops = []domain.Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3, RTTMedianMS: 10},
		{TTL: 2, Address: "198.51.100.1", Sent: 3, Received: 2, RTTMedianMS: 20},
		{TTL: 3, Address: "203.0.113.10", Sent: 3, Received: 3, RTTMedianMS: 30},
	}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 160, 40, WithColor())
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})
	model, _ = pressKey(t, model, "2")

	view := model.View()
	if !strings.Contains(view, "\x1b[32m→") {
		t.Fatalf("the target hop is not green:\n%q", view)
	}
	if !strings.Contains(view, "\x1b[33m!") {
		t.Fatalf("a lossy hop is not yellow:\n%q", view)
	}
}

func TestRunViewColoursTheDropBoundary(t *testing.T) {
	forceColor(t)
	steps := []domain.StepResult{
		runStep("dns", domain.StepDNS, domain.StatusPass, 10, domain.Measurement{Name: "system.answers", Value: "203.0.113.10"}),
	}
	trace := runStep("trace", domain.StepTrace, domain.StatusWarn, 500, domain.Measurement{Name: "method", Value: "udp"})
	trace.Hops = []domain.Hop{
		{TTL: 1, Address: "192.0.2.1", Sent: 3, Received: 3},
		{TTL: 2, Sent: 3},
	}
	steps = append(steps, trace)
	targets := runTargets()
	model := runModel(t, 160, 40, WithColor())
	model = applyMessage(t, model, ProbeCompleteMsg{Run: app.ProbeRun{Targets: []app.TargetRun{
		{Target: targets[0], State: app.TargetDone, Steps: steps},
		{Target: targets[1], State: app.TargetDone},
	}}})
	model, _ = pressKey(t, model, "2")

	if view := model.View(); !strings.Contains(view, "\x1b[31mx") {
		t.Fatalf("the drop boundary is not red:\n%q", view)
	}
}
