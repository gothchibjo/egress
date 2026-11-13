package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

// runViewMode selects the visible pane in the compact layout.
type runViewMode int

const (
	runViewLadder runViewMode = iota
	runViewHops
	runViewVerdict
)

type runTarget struct {
	target     domain.ProbeTarget
	state      app.TargetState
	steps      []domain.StepResult
	path       domain.PathReport
	hypothesis domain.Hypothesis
	selected   int
	hopsOffset int
}

type runState struct {
	targets []runTarget
	active  int
	view    runViewMode
	running bool
	err     string
}

// ProbeProgressMsg marks a target as running.
type ProbeProgressMsg struct {
	Progress app.ProbeProgress
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
	m.run = runState{targets: make([]runTarget, len(targets)), running: true}
	for i, target := range targets {
		m.run.targets[i] = runTarget{target: target, state: app.TargetPending, steps: []domain.StepResult{}}
	}
	m.view = viewRun
	return m
}

// updateRunKey handles target tabs, pane switching, scrolling and cancellation.
func (m Model) updateRunKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m = m.clearNotice()
	switch msg.String() {
	case "up", "k":
		m.scrollRun(-1)
	case "down", "j":
		m.scrollRun(1)
	case "g":
		m.scrollRun(-1 << 20)
	case "G":
		m.scrollRun(1 << 20)
	case "left", "h", "[":
		m.selectRunTarget(-1)
	case "right", "l", "]":
		m.selectRunTarget(1)
	case "1":
		m.run.view = runViewLadder
	case "2":
		m.run.view = runViewHops
	case "3":
		m.run.view = runViewVerdict
	case "tab":
		m.run.view = (m.run.view + 1) % 3
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

func (m *Model) scrollRun(delta int) {
	target, ok := m.activeRunTarget()
	if !ok {
		return
	}
	index := m.run.active
	switch m.run.view {
	case runViewHops:
		next := m.run.targets[index].hopsOffset + delta
		if next < 0 {
			next = 0
		}
		if max := len(target.path.Hops) - 1; next > max {
			next = max
		}
		if next < 0 {
			next = 0
		}
		m.run.targets[index].hopsOffset = next
	default:
		next := m.run.targets[index].selected + delta
		if next < 0 {
			next = 0
		}
		if max := len(target.steps) - 1; next > max {
			next = max
		}
		if next < 0 {
			next = 0
		}
		m.run.targets[index].selected = next
	}
}

func (m *Model) selectRunTarget(delta int) {
	if len(m.run.targets) == 0 {
		return
	}
	next := m.run.active + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.run.targets) {
		next = len(m.run.targets) - 1
	}
	m.run.active = next
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
			m.run.targets[i].path, m.run.targets[i].hypothesis = domain.BuildTargetReport(result.Steps)
		}
	}
	return m
}

// appendProbeStep stores one live step.
func (m Model) appendProbeStep(targetID string, step domain.StepResult) Model {
	for i := range m.run.targets {
		if m.run.targets[i].target.ID == targetID {
			m.run.targets[i].steps = append(m.run.targets[i].steps, step)
			return m
		}
	}
	return m
}

// markProbeTarget records that a target started.
func (m Model) markProbeTarget(progress app.ProbeProgress) Model {
	for i := range m.run.targets {
		if m.run.targets[i].target.ID == progress.TargetID {
			m.run.targets[i].state = app.TargetRunning
		}
	}
	return m
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
	tabs := []string{m.runTabs()}
	footer := m.renderHintFooter(m.footerHints())
	bodyHeight := m.height - len(header) - len(tabs) - len(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body []string
	if m.Layout() == LayoutNormal && bodyHeight >= 12 && m.run.view != runViewVerdict {
		band := 5
		panes := bodyHeight - band - 1
		leftWidth := m.width * 2 / 5
		rightWidth := m.width - leftWidth - 3
		left := block(m.runLadderLines(leftWidth, panes), leftWidth, panes)
		right := block(m.runHopsLines(rightWidth, panes), rightWidth, panes)
		joined := lipgloss.JoinHorizontal(lipgloss.Top,
			left, block(m.separatorLines(panes), 3, panes), right)
		body = append(body, strings.Split(joined, "\n")...)
		body = append(body, m.rule(m.width))
		body = append(body, strings.Split(block(m.runVerdictBand(), m.width, band), "\n")...)
	} else {
		var lines []string
		switch m.run.view {
		case runViewHops:
			lines = m.runHopsLines(m.width, bodyHeight)
		case runViewVerdict:
			lines = m.runVerdictLines(m.width, bodyHeight)
		default:
			lines = m.runLadderLines(m.width, bodyHeight)
		}
		body = strings.Split(block(lines, m.width, bodyHeight), "\n")
	}

	lines := append(header, tabs...)
	lines = append(lines, body...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
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

// runLadderLines renders the steps of the active target.
func (m Model) runLadderLines(width, height int) []string {
	target, ok := m.activeRunTarget()
	if !ok {
		return []string{"", " no targets selected"}
	}
	if len(target.steps) == 0 {
		title := " " + m.theme().heading.Render("LADDER")
		switch target.state {
		case app.TargetRunning:
			return []string{title, "", " probing " + target.target.Host + "..."}
		case app.TargetCancelled:
			return []string{title, "", " target cancelled before any step"}
		default:
			return []string{title, "", " waiting for the first step"}
		}
	}

	lines := make([]string, 0, len(target.steps)+1)
	lines = append(lines, " "+m.theme().heading.Render("LADDER"))
	for i, step := range target.steps {
		prefix := "  "
		if i == target.selected {
			prefix = "> "
		}
		row := fmt.Sprintf("%s%s %-10s %5dms", prefix, m.marker(step.Status), step.ID, step.DurationMS)
		if step.Diagnostic != "" {
			row += "  " + step.Diagnostic
		}
		lines = append(lines, row)
	}
	return windowLines(lines, target.selected+1, height)
}

// runHopsLines renders the hop table of the active target.
func (m Model) runHopsLines(width, height int) []string {
	target, ok := m.activeRunTarget()
	if !ok {
		return []string{"", " no targets selected"}
	}
	if len(target.path.Hops) == 0 {
		return []string{" " + m.theme().heading.Render("HOPS"), "", " no path data yet"}
	}

	lines := []string{
		" " + m.theme().heading.Render("HOPS"),
		fmt.Sprintf(" %3s  %-24s %5s %7s", "TTL", "ADDRESS", "LOSS", "RTT"),
	}
	for _, hop := range target.path.Hops {
		address := hop.Address
		if address == "" {
			address = "*"
		}
		lines = append(lines, fmt.Sprintf(" %3d  %-24s %4d%% %6dms",
			hop.TTL, address, hop.LossPercent(), hop.RTTMedianMS))
	}
	return windowLines(lines, target.hopsOffset+2, height)
}

// runVerdictBand is the compact verdict strip under the wide layout.
func (m Model) runVerdictBand() []string {
	target, ok := m.activeRunTarget()
	if !ok || target.hypothesis.Type == "" {
		return []string{" verdict appears when the target finishes"}
	}
	hypothesis := target.hypothesis
	lines := []string{
		fmt.Sprintf(" %s  (%s)", hypothesis.Type, hypothesis.Confidence),
		" " + hypothesis.Summary,
	}
	if len(hypothesis.Evidence) > 0 {
		lines = append(lines, fmt.Sprintf(" evidence: [%s] %s", hypothesis.Evidence[0].Mode, hypothesis.Evidence[0].Text))
	}
	lines = append(lines, " caveat: "+hypothesis.Caveat, " 3 opens the full verdict")
	return lines
}

// runVerdictLines renders the hypothesis with its evidence.
func (m Model) runVerdictLines(width, height int) []string {
	target, ok := m.activeRunTarget()
	if !ok {
		return []string{"", " no targets selected"}
	}
	if target.hypothesis.Type == "" {
		if target.state == app.TargetDone {
			return []string{"", " verdict is not available for this target"}
		}
		return []string{"", " verdict appears when the target finishes"}
	}

	hypothesis := target.hypothesis
	lines := []string{
		fmt.Sprintf(" %s  (%s)", hypothesis.Type, hypothesis.Confidence),
		" " + hypothesis.Summary,
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
	lines = append(lines, " CAVEAT", "   "+hypothesis.Caveat)
	return lines
}

// windowLines keeps the selected row visible inside the pane.
func windowLines(lines []string, selected, height int) []string {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	offset := 0
	if selected >= height {
		offset = selected - height + 1
	}
	if max := len(lines) - height; offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}
	return lines[offset : offset+height]
}
