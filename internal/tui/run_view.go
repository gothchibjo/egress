package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// runViewMode selects the visible pane in the compact layout.
type runViewMode int

const (
	runViewLadder runViewMode = iota
	runViewHops
	runViewVerdict
)

type runTarget struct {
	target        domain.ProbeTarget
	state         app.TargetState
	steps         []domain.StepResult
	plan          []domain.StepPlan
	runningStep   domain.StepPlan
	outcome       domain.Status
	path          domain.PathReport
	hypothesis    domain.Hypothesis
	ladderOffset  int
	hopsOffset    int
	verdictOffset int
}

type runState struct {
	targets    []runTarget
	active     int
	view       runViewMode
	focus      bool
	running    bool
	cancelled  bool
	startedAt  time.Time
	finishedAt time.Time
	err        string
}

// ProbeProgressMsg marks a target as running.
type ProbeProgressMsg struct {
	Progress app.ProbeProgress
}

// ProbeStepStartMsg marks one step as running.
type ProbeStepStartMsg struct {
	TargetID string
	Plan     domain.StepPlan
}

// ProbeTargetDoneMsg reports one finished target before the session ends.
type ProbeTargetDoneMsg struct {
	TargetID string
	State    app.TargetState
	Steps    []domain.StepResult
}

// ProbeStepMsg appends one live step to a running target.
type ProbeStepMsg struct {
	TargetID string
	Step     domain.StepResult
}

// ProbeCompleteMsg delivers the finished probe session.
type ProbeCompleteMsg struct {
	Run app.ProbeRun
	Err error
}

// startRun switches to the run view with the selected targets pending.
func (m Model) startRun(targets []domain.ProbeTarget) Model {
	m.run = runState{targets: make([]runTarget, len(targets)), running: true, startedAt: m.opts.now()}
	for i, target := range targets {
		m.run.targets[i] = runTarget{target: target, state: app.TargetPending, steps: []domain.StepResult{}}
	}
	m.view = viewRun
	return m
}

// updateRunKey switches targets; Enter focuses the selected pane for scrolling.
func (m Model) updateRunKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m = m.clearNotice()
	if m.run.focus {
		return m.updateRunPaneKey(msg)
	}

	switch msg.String() {
	case "up", "k", "left", "h", "[":
		m.selectRunTarget(-1)
	case "down", "j", "right", "l", "]":
		m.selectRunTarget(1)
	case "1":
		m.run.view = runViewLadder
	case "2":
		m.run.view = runViewHops
	case "3":
		m.run.view = runViewVerdict
	case "tab":
		m.run.view = (m.run.view + 1) % 3
	case "g":
		m.selectRunTargetIndex(0)
	case "G":
		m.selectRunTargetIndex(len(m.run.targets) - 1)
	case "n":
		return m.jumpProblemTarget(1)
	case "p":
		return m.jumpProblemTarget(-1)
	case "r":
		return m.rerunProbes()
	case "e":
		return m.exportRunCurrent()
	case "E":
		return m.promptRunExport()
	case "c":
		return m.openContext(viewRun), nil
	case "d":
		m.runDestinationOffset = 0
		m.view = viewRunDestinations
	case "enter":
		m.run.focus = true
	case "x":
		if target, ok := m.activeRunTarget(); ok && target.state == app.TargetRunning && m.opts.cancelTarget != nil {
			m.opts.cancelTarget(target.target.ID)
			return m.setNotice("cancelling " + target.target.Label)
		}
	case "X":
		if m.run.running && m.opts.cancelAllProbe != nil {
			m.opts.cancelAllProbe()
			return m.setNotice("cancelling every target")
		}
	case "esc":
		m.view = viewDashboard
	case "?":
		m.helpReturn = viewRun
		m.helpOffset = 0
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

// updateRunPaneKey scrolls the focused pane. Target switching stays on h/l so a
// scroll does not trap the user in one target.
func (m Model) updateRunPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.scrollRun(-1)
	case "down", "j":
		m.scrollRun(1)
	case "pgup":
		m.scrollRun(-10)
	case "pgdown":
		m.scrollRun(10)
	case "g":
		m.scrollRun(-1 << 20)
	case "G":
		m.scrollRun(1 << 20)
	case "left", "h", "[":
		m.selectRunTarget(-1)
	case "right", "l", "]":
		m.selectRunTarget(1)
	case "n":
		return m.jumpProblemTarget(1)
	case "p":
		return m.jumpProblemTarget(-1)
	case "r":
		return m.rerunProbes()
	case "e":
		return m.exportRunCurrent()
	case "E":
		return m.promptRunExport()
	case "c":
		return m.openContext(viewRun), nil
	case "d":
		m.runDestinationOffset = 0
		m.view = viewRunDestinations
	case "1":
		m.run.view = runViewLadder
	case "2":
		m.run.view = runViewHops
	case "3":
		m.run.view = runViewVerdict
	case "tab":
		m.run.view = (m.run.view + 1) % 3
	case "esc":
		m.run.focus = false
	case "x":
		if target, ok := m.activeRunTarget(); ok && target.state == app.TargetRunning && m.opts.cancelTarget != nil {
			m.opts.cancelTarget(target.target.ID)
			return m.setNotice("cancelling " + target.target.Label)
		}
	case "X":
		if m.run.running && m.opts.cancelAllProbe != nil {
			m.opts.cancelAllProbe()
			return m.setNotice("cancelling every target")
		}
	case "?":
		m.helpReturn = viewRun
		m.helpOffset = 0
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

// rerunProbes restarts the current target selection without returning to the
// picker. The previous results are replaced, matching the dashboard rerun.
func (m Model) rerunProbes() (tea.Model, tea.Cmd) {
	if m.run.running {
		return m.setNotice("probes are already running")
	}
	if m.opts.restartProbes == nil {
		return m.setNotice("re-run is unavailable in this session")
	}
	targets := make([]domain.ProbeTarget, len(m.run.targets))
	for i, target := range m.run.targets {
		targets[i] = target.target
	}
	if len(targets) == 0 {
		return m.setNotice("no targets to probe")
	}
	m.opts.restartProbes(targets)
	return m.startRun(targets), nil
}

func (m *Model) scrollRun(delta int) {
	target, ok := m.activeRunTarget()
	if !ok {
		return
	}
	index := m.run.active
	switch m.run.view {
	case runViewHops:
		m.run.targets[index].hopsOffset = clampOffset(target.hopsOffset+delta, 1<<20)
	case runViewVerdict:
		m.run.targets[index].verdictOffset = clampOffset(target.verdictOffset+delta, 1<<20)
	default:
		m.run.targets[index].ladderOffset = clampOffset(target.ladderOffset+delta, 1<<20)
	}
}

func (m *Model) selectRunTarget(delta int) {
	if len(m.run.targets) == 0 {
		return
	}
	m.selectRunTargetIndex(m.run.active + delta)
}

// selectRunTargetIndex moves the target selection and resets its scrolls.
func (m *Model) selectRunTargetIndex(index int) {
	if len(m.run.targets) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(m.run.targets) {
		index = len(m.run.targets) - 1
	}
	m.run.active = index
	m.run.targets[index].ladderOffset = 0
	m.run.targets[index].hopsOffset = 0
	m.run.targets[index].verdictOffset = 0
}

// jumpProblemTarget moves to the next or previous target whose outcome is fail
// or warn, wrapping at both ends.
func (m Model) jumpProblemTarget(delta int) (Model, tea.Cmd) {
	problems := make([]int, 0, len(m.run.targets))
	for i, target := range m.run.targets {
		if target.state != app.TargetDone {
			continue
		}
		if target.outcome == domain.StatusFail || target.outcome == domain.StatusWarn {
			problems = append(problems, i)
		}
	}
	if len(problems) == 0 {
		return m.setNotice("no failed or warning targets")
	}

	target := problems[0]
	if delta < 0 {
		target = problems[len(problems)-1]
		for i := len(problems) - 1; i >= 0; i-- {
			if problems[i] < m.run.active {
				target = problems[i]
				break
			}
		}
	} else {
		for _, index := range problems {
			if index > m.run.active {
				target = index
				break
			}
		}
	}
	m.selectRunTargetIndex(target)
	return m, nil
}

func (m Model) activeRunTarget() (runTarget, bool) {
	if m.run.active < 0 || m.run.active >= len(m.run.targets) {
		return runTarget{}, false
	}
	return m.run.targets[m.run.active], true
}

// finishProbe replaces the pending state with the session result.
func (m Model) finishProbe(run app.ProbeRun, err error) Model {
	m.run.running = false
	m.run.finishedAt = m.opts.now()
	m.run.cancelled = run.Cancelled
	if err != nil {
		m.run.err = err.Error()
		return m
	}
	for i := range m.run.targets {
		for _, result := range run.Targets {
			if result.Target.ID != m.run.targets[i].target.ID {
				continue
			}
			m.run.targets[i].state = result.State
			m.run.targets[i].steps = result.Steps
			m.run.targets[i].runningStep = domain.StepPlan{}
			m.run.targets[i].outcome = outcomeOf(result.Steps)
			m.run.targets[i].path, m.run.targets[i].hypothesis = domain.BuildTargetReport(result.Steps)
		}
	}
	return m
}

// finishTarget records one finished target without waiting for the session.
func (m Model) finishTarget(msg ProbeTargetDoneMsg) Model {
	for i := range m.run.targets {
		if m.run.targets[i].target.ID != msg.TargetID {
			continue
		}
		m.run.targets[i].state = msg.State
		m.run.targets[i].steps = msg.Steps
		m.run.targets[i].runningStep = domain.StepPlan{}
		m.run.targets[i].outcome = outcomeOf(msg.Steps)
		m.run.targets[i].path, m.run.targets[i].hypothesis = domain.BuildTargetReport(msg.Steps)
	}
	return m
}

// appendProbeStep stores one live step and clears the running step marker.
func (m Model) appendProbeStep(targetID string, step domain.StepResult) Model {
	for i := range m.run.targets {
		if m.run.targets[i].target.ID == targetID {
			m.run.targets[i].steps = append(m.run.targets[i].steps, step)
			if m.run.targets[i].runningStep.ID == step.ID {
				m.run.targets[i].runningStep = domain.StepPlan{}
			}
			return m
		}
	}
	return m
}

// markProbeTarget records that a target started and stores its step plan.
func (m Model) markProbeTarget(progress app.ProbeProgress) Model {
	for i := range m.run.targets {
		if m.run.targets[i].target.ID == progress.TargetID {
			m.run.targets[i].state = app.TargetRunning
			m.run.targets[i].plan = append([]domain.StepPlan(nil), progress.Plan...)
			m.run.targets[i].runningStep = domain.StepPlan{}
		}
	}
	return m
}

// markProbeStep records the step a target is currently running.
func (m Model) markProbeStep(targetID string, plan domain.StepPlan) Model {
	for i := range m.run.targets {
		if m.run.targets[i].target.ID == targetID {
			m.run.targets[i].runningStep = plan
		}
	}
	return m
}

// outcomeOf folds the steps into one target marker status.
func outcomeOf(steps []domain.StepResult) domain.Status {
	outcome := domain.StatusSkipped
	for _, step := range steps {
		switch step.Status {
		case domain.StatusFail:
			return domain.StatusFail
		case domain.StatusWarn:
			outcome = domain.StatusWarn
		case domain.StatusPass:
			if outcome == domain.StatusSkipped {
				outcome = domain.StatusPass
			}
		}
	}
	return outcome
}

// renderRun draws the run view with the adaptive layout.
func (m Model) renderRun() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}

	header := m.renderPageHeader("RUN", m.runPosition())
	contextLine := []string{m.runContextLine()}
	footer := make([]string, 0, 3)
	if m.notice != "" {
		footer = append(footer, padLine(" "+m.notice, "", m.width))
	}
	footer = append(footer, m.renderHintFooter(m.footerHints())...)
	bodyHeight := m.height - len(header) - len(contextLine) - len(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	targetWidth := 22
	if m.Layout() == LayoutCompact {
		targetWidth = 16
	}
	column := block(m.runTargetLines(targetWidth, bodyHeight), targetWidth, bodyHeight)
	separator := block(m.separatorLines(bodyHeight), 3, bodyHeight)
	rightWidth := m.width - targetWidth - 3
	if rightWidth < 20 {
		rightWidth = 20
	}

	var right []string
	if m.Layout() == LayoutNormal && bodyHeight >= 14 && m.run.view != runViewVerdict {
		bandLines := m.runVerdictBand(rightWidth)
		band := len(bandLines)
		panes := bodyHeight - band - 1
		if rightWidth >= 85 && panes >= 12 {
			// Wide terminals keep three columns: targets, ladder and hops.
			leftWidth := rightWidth * 2 / 5
			hopsWidth := rightWidth - leftWidth - 3
			ladder := block(m.runLadderLines(leftWidth, panes), leftWidth, panes)
			hops := block(m.runHopsLines(hopsWidth, panes), hopsWidth, panes)
			top := lipgloss.JoinHorizontal(lipgloss.Top, ladder, block(m.separatorLines(panes), 3, panes), hops)
			right = strings.Split(top, "\n")
			right = append(right, m.rule(rightWidth))
			right = append(right, strings.Split(block(bandLines, rightWidth, band), "\n")...)
		} else {
			// Narrow terminals use two columns: the ladder and the hops stack in
			// the right column, so neither pane wraps a wide table.
			ladderHeight := panes * 2 / 5
			if ladderHeight < 4 {
				ladderHeight = 4
			}
			hopsHeight := panes - ladderHeight - 1
			if hopsHeight < 4 {
				hopsHeight = 4
				ladderHeight = panes - hopsHeight - 1
			}
			ladder := block(m.runLadderLines(rightWidth, ladderHeight), rightWidth, ladderHeight)
			hops := block(m.runHopsLines(rightWidth, hopsHeight), rightWidth, hopsHeight)
			right = append(right, strings.Split(ladder, "\n")...)
			right = append(right, m.rule(rightWidth))
			right = append(right, strings.Split(hops, "\n")...)
			right = append(right, m.rule(rightWidth))
			right = append(right, strings.Split(block(bandLines, rightWidth, band), "\n")...)
		}
	} else {
		var lines []string
		switch m.run.view {
		case runViewHops:
			lines = m.runHopsLines(rightWidth, bodyHeight)
		case runViewVerdict:
			lines = m.runVerdictLines(rightWidth, bodyHeight)
		default:
			lines = m.runLadderLines(rightWidth, bodyHeight)
		}
		right = strings.Split(block(lines, rightWidth, bodyHeight), "\n")
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, column, separator, block(right, rightWidth, bodyHeight))
	lines := append(header, contextLine...)
	lines = append(lines, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

// runContextLine keeps the proxy and VPN context visible without opening the
// context page, and states its absence explicitly. Long lists collapse to a
// count so the "c details" hint always survives the width.
func (m Model) runContextLine() string {
	detail := "not collected"
	if context := m.opts.networkContext; context != nil {
		parts := make([]string, 0, 2)
		if proxies := uniqueNames(proxyNames(context.Proxies)); len(proxies) > 0 {
			parts = append(parts, "proxy "+summarizeNames(proxies, 2, "settings"))
		}
		if tunnels := uniqueNames(vpnNames(context.VPN)); len(tunnels) > 0 {
			parts = append(parts, "vpn "+summarizeNames(tunnels, 3, "interfaces"))
		}
		if len(parts) == 0 {
			detail = "no proxy or VPN detected"
		} else {
			detail = strings.Join(parts, " · ")
		}
	}
	if m.opts.contextErr != "" {
		detail += " · interface list unavailable"
	}
	suffix := " · c details"
	if limit := m.width - 2 - lipgloss.Width(suffix); limit > 4 && lipgloss.Width(detail) > limit {
		detail = clip(detail, limit)
	}
	return padLine(" "+m.theme().muted.Render("context: "+detail+suffix), "", m.width)
}

func proxyNames(proxies []platform.ProxySetting) []string {
	names := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		names = append(names, proxy.Name)
	}
	return names
}

func vpnNames(interfaces []platform.VPNInterface) []string {
	names := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		names = append(names, iface.Name)
	}
	return names
}

func uniqueNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func summarizeNames(names []string, limit int, unit string) string {
	if len(names) <= limit {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%d %s", len(names), unit)
}

// runTargetLines renders the target column with state and outcome markers.
func (m Model) runTargetLines(width, height int) []string {
	lines := []string{" " + m.theme().heading.Render("TARGETS")}
	rowLine := make([]int, len(m.run.targets))
	for i, target := range m.run.targets {
		prefix := "  "
		if i == m.run.active {
			prefix = "> "
		}
		rowLine[i] = len(lines)
		marker := m.targetMarker(target)
		label := m.clipLabel(targetLabel(target), width-2-lipgloss.Width(marker)-1)
		lines = append(lines, prefix+marker+" "+label)
	}
	if height <= 0 || len(lines) <= height {
		return lines
	}
	offset := 0
	if selected := rowLine[m.run.active]; selected >= height {
		offset = selected - height + 1
	}
	if maxOffset := len(lines) - height; offset > maxOffset {
		offset = maxOffset
	}
	return lines[offset : offset+height]
}

func (m Model) targetMarker(target runTarget) string {
	switch target.state {
	case app.TargetRunning:
		if m.opts.ascii {
			return m.theme().muted.Render("...")
		}
		return m.theme().muted.Render("…")
	case app.TargetPending:
		return m.marker(domain.StatusSkipped)
	case app.TargetCancelled:
		return "x"
	}
	return m.marker(target.outcome)
}

func targetLabel(target runTarget) string {
	if target.target.Label != "" {
		return target.target.Label
	}
	return target.target.Host
}

func (m Model) runPosition() string {
	if len(m.run.targets) == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d targets ", m.run.active+1, len(m.run.targets))
}

// runTabs renders the target tabs with their states.
func (m Model) runTabs() string {
	parts := make([]string, 0, len(m.run.targets))
	for i, target := range m.run.targets {
		label := target.target.Label
		if label == "" {
			label = target.target.Host
		}
		marker := runStateMarker(target.state)
		part := marker + " " + label
		if i == m.run.active {
			part = "> " + label + " " + marker
		}
		parts = append(parts, part)
	}
	return " " + strings.Join(parts, m.separator())
}

func runStateMarker(state app.TargetState) string {
	switch state {
	case app.TargetRunning:
		return "..."
	case app.TargetDone:
		return "ok"
	case app.TargetCancelled:
		return "x"
	default:
		return "-"
	}
}

// runLadderLines renders the planned steps of the active target: finished,
// running and queued. Without focus the window follows the running step.
func (m Model) runLadderLines(width, height int) []string {
	target, ok := m.activeRunTarget()
	if !ok {
		return []string{"", " no targets selected"}
	}
	title := m.paneTitle("LADDER", runViewLadder)
	rows, autoIndex := m.runLadderRows(target, width-2)
	if len(rows) == 0 {
		switch target.state {
		case app.TargetRunning:
			return []string{title, "", " probing " + target.target.Host + "..."}
		case app.TargetCancelled:
			return []string{title, "", " target cancelled before any step"}
		default:
			return []string{title, "", " waiting for the first step"}
		}
	}

	offset := 0
	if m.run.focus && m.run.view == runViewLadder {
		offset = target.ladderOffset
	} else if autoIndex >= 0 {
		offset = autoOffset(autoIndex, height-1)
	}

	visible := windowFrom(rows, offset, height-1)
	lines := make([]string, 0, len(visible)+1)
	lines = append(lines, title)
	for _, row := range visible {
		lines = append(lines, "  "+row)
	}
	return lines
}

// runLadderRows lists every planned step with its state and returns the row
// that should stay visible without focus.
func (m Model) runLadderRows(target runTarget, width int) ([]string, int) {
	finished := make(map[string]domain.StepResult, len(target.steps))
	for _, step := range target.steps {
		finished[step.ID] = step
	}

	runningMarker := "..."
	if !m.opts.ascii {
		runningMarker = "…"
	}

	if len(target.plan) == 0 {
		rows := make([]string, 0, len(target.steps))
		latest := -1
		for _, step := range target.steps {
			latest = len(rows)
			rows = append(rows, m.styleStepRow(step, width)...)
		}
		return rows, latest
	}

	rows := make([]string, 0, len(target.plan))
	latest := -1
	for _, planned := range target.plan {
		if step, ok := finished[planned.ID]; ok {
			latest = len(rows)
			rows = append(rows, m.styleStepRow(step, width)...)
			continue
		}
		if target.state == app.TargetRunning && planned.ID == target.runningStep.ID {
			latest = len(rows)
			rows = append(rows, wrapPaneLine(fmt.Sprintf("%s %-10s running",
				m.theme().muted.Render(runningMarker), planned.ID), width)...)
			continue
		}
		rows = append(rows, fmt.Sprintf("%s %-10s", m.marker(domain.StatusSkipped), planned.ID))
	}
	if latest < 0 {
		latest = 0
	}
	return rows, latest
}

func (m Model) runStepRow(step domain.StepResult) string {
	row := fmt.Sprintf("%s %-10s %5dms", m.marker(step.Status), step.ID, step.DurationMS)
	if step.Diagnostic != "" {
		row += "  " + step.Diagnostic
	}
	return row
}

// styleStepRow wraps a finished step and colours the whole row by its status,
// so a failed or warning step is visible without reading the marker.
func (m Model) styleStepRow(step domain.StepResult, width int) []string {
	wrapped := wrapPaneLine(m.runStepRow(step), width)
	style := m.theme().status(step.Status)
	for i, line := range wrapped {
		wrapped[i] = style.Render(line)
	}
	return wrapped
}

// runHopsLines renders the hop table of the active target. Narrow panes use a
// two-line row so a long address never collides with the statistics.
func (m Model) runHopsLines(width, height int) []string {
	target, ok := m.activeRunTarget()
	if !ok {
		return []string{"", " no targets selected"}
	}
	method := ""
	for _, step := range target.steps {
		if step.Kind == domain.StepTrace {
			for _, measurement := range step.Measurements {
				if measurement.Name == "method" {
					method = measurement.Value
				}
			}
		}
	}
	name := "HOPS"
	if method != "" {
		name = "HOPS (" + method + ")"
	}
	if address := tracedAddress(target.steps); address != "" && width >= 50 {
		name += " -> " + address
	}
	title := m.paneTitle(name, runViewHops)
	if len(target.path.Hops) == 0 {
		return []string{title, "", " no path data yet"}
	}

	allSilent := true
	for _, hop := range target.path.Hops {
		if hop.Received > 0 {
			allSilent = false
			break
		}
	}
	if allSilent {
		lines := []string{title, "", " no hop answered on " + method}
		if ladderReached(target.steps) {
			lines = append(lines, " the target answered a transport step; no router returned a probe reply")
		} else {
			lines = append(lines, " the target did not answer any ladder step")
		}
		lines = append(lines, fmt.Sprintf(" %d TTLs probed without a reply", len(target.path.Hops)))
		if summary := target.path.Localization.Summary; summary != "" {
			lines = append(lines, "", " "+summary)
		}
		return wrapPaneLines(lines, width)
	}

	narrow := width < 56
	addressWidth := width - 22
	if addressWidth < 18 {
		addressWidth = 18
	}
	// A target that answered the ladder is reachable even when the traceroute
	// received no ICMP reply for the last TTLs; do not draw a trailing star.
	hops := trimSilentTail(target.path.Hops, !ladderReached(target.steps))
	targetIP := tracedAddress(target.steps)
	table := make([]string, 0, len(hops)+2)
	if narrow {
		table = append(table, fmt.Sprintf("  %3s  %s", "TTL", "ADDRESS"), "        LOSS / RTT")
	} else {
		table = append(table, fmt.Sprintf("  %3s  %-*s %5s %7s", "TTL", addressWidth-2, "ADDRESS", "LOSS", "RTT"))
	}
	for index, hop := range hops {
		address := hopAddress(hop)
		if address == "" {
			address = "*"
		}
		marker := m.hopMarker(hop, targetIP, index == len(hops)-1)
		style := m.hopStyle(marker)
		if narrow || lipgloss.Width(address) > addressWidth-2 {
			table = append(table,
				style.Render(fmt.Sprintf("%s %3d  %s", marker, hop.TTL, address)),
				style.Render(fmt.Sprintf("        %d%% loss  %dms", hop.LossPercent(), hop.RTTMedianMS)),
			)
			continue
		}
		table = append(table, style.Render(fmt.Sprintf("%s %3d  %-*s %4d%% %6dms",
			marker, hop.TTL, addressWidth-2, address, hop.LossPercent(), hop.RTTMedianMS)))
	}
	if summary := target.path.Localization.Summary; summary != "" {
		table = append(table, "", " "+summary)
	}
	table = wrapPaneLines(table, width)

	offset := 0
	if m.run.focus && m.run.view == runViewHops {
		offset = target.hopsOffset
	}
	lines := append([]string{title}, windowFrom(table, offset, height-1)...)
	return lines
}

// runVerdictBand is the always-visible verdict strip: the caveat is wrapped
// instead of clipped, so it cannot be hidden by a narrow terminal.
func (m Model) runVerdictBand(width int) []string {
	target, ok := m.activeRunTarget()
	if !ok || target.hypothesis.Type == "" {
		return []string{" verdict appears when the target finishes"}
	}
	hypothesis := target.hypothesis
	headline := fmt.Sprintf(" %s  (%s)", hypothesis.Type, hypothesis.Confidence)
	if hypothesis.NoBlockage {
		headline = " no blockage detected"
	}
	lines := []string{
		headline,
		" " + hypothesis.Summary,
	}
	if len(hypothesis.Evidence) > 0 {
		lines = append(lines, fmt.Sprintf(" evidence: [%s] %s", hypothesis.Evidence[0].Mode, hypothesis.Evidence[0].Text))
	}
	lines = append(lines,
		wrapPaneLine(" caveat: "+hypothesis.Caveat, width)...,
	)
	lines = append(lines, " 3 opens the full verdict")
	return lines
}

// runVerdictLines renders the hypothesis with its evidence.
func (m Model) runVerdictLines(width, height int) []string {
	target, ok := m.activeRunTarget()
	if !ok {
		return []string{"", " no targets selected"}
	}
	title := m.paneTitle("VERDICT", runViewVerdict)
	if target.hypothesis.Type == "" {
		if target.state == app.TargetDone {
			return []string{title, "", " verdict is not available for this target"}
		}
		return []string{title, "", " verdict appears when the target finishes"}
	}

	hypothesis := target.hypothesis
	headline := fmt.Sprintf(" %s  (%s)", hypothesis.Type, hypothesis.Confidence)
	if hypothesis.NoBlockage {
		headline = " no blockage detected"
	}
	lines := []string{
		headline,
		" " + hypothesis.Summary,
		" CAVEAT",
		"   " + hypothesis.Caveat,
		" EVIDENCE",
	}
	for _, line := range hypothesis.Evidence {
		lines = append(lines, fmt.Sprintf("   [%s] %s", line.Mode, line.Text))
	}
	if len(hypothesis.Rejected) > 0 {
		lines = append(lines, " REJECTED")
		for _, rejected := range hypothesis.Rejected {
			lines = append(lines, fmt.Sprintf("   %s: %s", rejected.Type, rejected.Reason))
		}
	}
	lines = wrapPaneLines(lines, width)

	offset := 0
	if m.run.focus && m.run.view == runViewVerdict {
		offset = target.verdictOffset
	}
	return append([]string{title}, windowFrom(lines, offset, height-1)...)
}

// paneTitle marks the selected pane and the scroll focus.
func (m Model) paneTitle(name string, pane runViewMode) string {
	title := " " + m.theme().heading.Render(name)
	if m.run.view == pane {
		if m.run.focus {
			title += " " + m.theme().muted.Render("[scroll]")
		} else {
			title += " ·"
		}
	}
	return title
}

// autoOffset keeps an index visible in a window of the given height.
func autoOffset(index, height int) int {
	if height <= 0 || index < height {
		return 0
	}
	return index - height + 1
}

// windowFrom slices a window starting at offset.
func windowFrom(lines []string, offset, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	if offset < 0 {
		offset = 0
	}
	if max := len(lines) - height; offset > max {
		offset = max
	}
	return lines[offset : offset+height]
}

// wrapPaneLines wraps long pane lines so nothing is clipped horizontally.
func wrapPaneLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapPaneLine(line, width)...)
	}
	return out
}

// wrapPaneLine wraps one line, indenting the continuation under the text.
func wrapPaneLine(line string, width int) []string {
	if width < 8 || lipgloss.Width(line) <= width {
		return []string{line}
	}
	indent := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		indent++
	}
	continuation := indent + 2
	if continuation > 8 {
		continuation = 8
	}
	available := width - continuation
	if available < 8 {
		available = 8
	}
	parts := wrapText(strings.TrimSpace(line), available)
	out := make([]string, 0, len(parts))
	first := true
	for _, part := range parts {
		for _, chunk := range splitLongToken(part, available) {
			if first {
				out = append(out, strings.Repeat(" ", indent)+chunk)
				first = false
				continue
			}
			out = append(out, strings.Repeat(" ", continuation)+chunk)
		}
	}
	return out
}

// splitLongToken breaks a token that cannot fit on one line, such as a long
// IPv6 address, so its characters stay readable.
func splitLongToken(text string, width int) []string {
	runes := []rune(text)
	if width < 1 || len(runes) <= width {
		return []string{text}
	}
	out := make([]string, 0, len(runes)/width+1)
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

// clipLabel shortens a target label with an ellipsis.
func (m Model) clipLabel(text string, width int) string {
	if width < 2 {
		return clip(text, width)
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	ellipsis := "…"
	if m.opts.ascii {
		ellipsis = "..."
	}
	return clip(text, width-lipgloss.Width(ellipsis)) + ellipsis
}

// tracedAddress returns the address the ladder resolved for the target.
func tracedAddress(steps []domain.StepResult) string {
	for _, step := range steps {
		if step.Kind == domain.StepTrace {
			for _, measurement := range step.Measurements {
				if measurement.Name == "address" && measurement.Value != "" {
					return measurement.Value
				}
			}
		}
	}
	for _, step := range steps {
		switch step.Kind {
		case domain.StepDNS:
			for _, measurement := range step.Measurements {
				if measurement.Name == "system.answers" && measurement.Value != "" && measurement.Value != "none" {
					if index := strings.Index(measurement.Value, ","); index >= 0 {
						return strings.TrimSpace(measurement.Value[:index])
					}
					return strings.TrimSpace(measurement.Value)
				}
			}
		case domain.StepICMP:
			for _, measurement := range step.Measurements {
				if measurement.Name == "address" && measurement.Value != "" {
					return measurement.Value
				}
			}
		}
	}
	return ""
}

// hopMarker flags what deserves attention in a table that otherwise follows
// the familiar traceroute layout: the target hop, a hop with partial loss and
// the silent hop where the replies stop.
func (m Model) hopMarker(hop domain.Hop, targetIP string, last bool) string {
	target := "→"
	if m.opts.ascii {
		target = ">"
	}
	if targetIP != "" {
		for _, address := range hop.AllAddresses() {
			if address == targetIP {
				return target
			}
		}
	}
	if hop.Received == 0 {
		if last {
			return "x"
		}
		return " "
	}
	if hop.Received < hop.Sent {
		return "!"
	}
	return " "
}

// hopStyle colours a hop row: green for the target, yellow for partial loss,
// red for the silent boundary and no colour for a normal hop.
func (m Model) hopStyle(marker string) lipgloss.Style {
	switch marker {
	case "→", ">":
		return m.theme().pass
	case "!":
		return m.theme().warn
	case "x":
		return m.theme().fail
	default:
		return lipgloss.Style{}
	}
}

// hopAddress joins every router that answered at one TTL.
func hopAddress(hop domain.Hop) string {
	if len(hop.Addresses) > 0 {
		return strings.Join(hop.Addresses, ", ")
	}
	return hop.Address
}

// trimSilentTail drops trailing silent hops so a table does not end in a wall
// of stars. One silent hop is kept only when the target was not reachable on a
// ladder step, to show where the replies stop.
func trimSilentTail(hops []domain.Hop, keepSilent bool) []domain.Hop {
	last := -1
	for i, hop := range hops {
		if hop.Received > 0 {
			last = i
		}
	}
	if last < 0 || last == len(hops)-1 {
		return hops
	}
	end := last + 1
	if keepSilent {
		end = last + 2
	}
	if end > len(hops) {
		end = len(hops)
	}
	return hops[:end]
}

// ladderReached reports whether a transport step reached the target.
func ladderReached(steps []domain.StepResult) bool {
	for _, step := range steps {
		switch step.Kind {
		case domain.StepTCP, domain.StepTLS, domain.StepHTTP:
			if step.Status == domain.StatusPass {
				return true
			}
		}
	}
	return false
}
