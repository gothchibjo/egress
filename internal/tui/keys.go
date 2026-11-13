package tui

import tea "github.com/charmbracelet/bubbletea"

// viewState is the page the dashboard currently shows.
type viewState int

const (
	viewDashboard viewState = iota
	viewDetail
	viewHelp
	viewDestinations
	viewContext
	viewPicker
	viewRun
)

// updateKey dispatches a key press to the active page. Every action has a key;
// no mouse handler exists, so keyboard control is complete.
func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmQuit {
		return m.updateConfirmKey(msg)
	}
	if m.exportPrompt || m.exportOverwrite != "" {
		return m.updateExportKey(msg)
	}
	switch m.view {
	case viewHelp:
		return m.updateHelpKey(msg)
	case viewDestinations:
		return m.updateDestinationsKey(msg)
	case viewContext:
		return m.updateContextKey(msg)
	case viewPicker:
		return m.updatePickerKey(msg)
	case viewRun:
		return m.updateRunKey(msg)
	case viewDetail:
		return m.updateDetailKey(msg)
	default:
		return m.updateDashboardKey(msg)
	}
}

func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m = m.clearNotice()
	switch msg.String() {
	case "up", "k":
		return m.move(-1), nil
	case "down", "j":
		return m.move(1), nil
	case "g":
		return m.Select(0), nil
	case "G":
		return m.Select(len(rows(m.results)) - 1), nil
	case "n":
		return m.jumpProblem(1)
	case "p":
		return m.jumpProblem(-1)
	case "enter":
		if _, ok := m.SelectedResult(); ok {
			m.view = viewDetail
			m.detailOffset = 0
		}
	case "?":
		m.helpReturn = viewDashboard
		m.helpOffset = 0
		m.view = viewHelp
	case "d":
		m.destinationsReturn = viewDashboard
		m.destinationOffset = 0
		m.view = viewDestinations
	case "c":
		return m.openContext(viewDashboard), nil
	case "t":
		return m.openPicker(), nil
	case "e":
		return m.exportCurrent()
	case "E":
		return m.promptExport()
	case "r":
		return m.rerun()
	case "x":
		if m.cancelCurrent() {
			m.progress = "cancelling current check..."
		}
	case "X":
		if m.cancelAll() {
			m.progress = "cancelling baseline..."
		}
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

func (m Model) updateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m = m.clearNotice()
	switch msg.String() {
	case "up", "k":
		m.detailOffset = clampOffset(m.detailOffset-1, m.detailMaxOffset())
	case "down", "j":
		m.detailOffset = clampOffset(m.detailOffset+1, m.detailMaxOffset())
	case "pgup":
		m.detailOffset = clampOffset(m.detailOffset-m.detailBodyHeight(), m.detailMaxOffset())
	case "pgdown":
		m.detailOffset = clampOffset(m.detailOffset+m.detailBodyHeight(), m.detailMaxOffset())
	case "g":
		m.detailOffset = 0
	case "G":
		m.detailOffset = m.detailMaxOffset()
	case "h", "[":
		return m.move(-1), nil
	case "l", "]":
		return m.move(1), nil
	case "n":
		return m.jumpProblem(1)
	case "p":
		return m.jumpProblem(-1)
	case "r":
		return m.rerun()
	case "esc":
		m.view = viewDashboard
	case "x":
		if m.cancelCurrent() {
			m.progress = "cancelling current check..."
		}
	case "X":
		if m.cancelAll() {
			m.progress = "cancelling baseline..."
		}
	case "?":
		m.helpReturn = viewDetail
		m.helpOffset = 0
		m.view = viewHelp
	case "d":
		m.destinationsReturn = viewDetail
		m.destinationOffset = 0
		m.view = viewDestinations
	case "c":
		return m.openContext(viewDetail), nil
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

// openContext shows the read-only proxy and tunnel context page.
func (m Model) openContext(returnTo viewState) Model {
	m.contextReturn = returnTo
	m.contextOffset = 0
	m.view = viewContext
	return m
}

func (m Model) updateContextKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.contextOffset = clampOffset(m.contextOffset-1, m.contextMaxOffset())
	case "down", "j":
		m.contextOffset = clampOffset(m.contextOffset+1, m.contextMaxOffset())
	case "g":
		m.contextOffset = 0
	case "G":
		m.contextOffset = m.contextMaxOffset()
	case "esc", "c":
		m.view = m.contextReturn
		m.contextOffset = 0
	case "?":
		m.helpReturn = viewContext
		m.helpOffset = 0
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

func (m Model) updateDestinationsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.destinationOffset = clampOffset(m.destinationOffset-1, m.destinationMaxOffset())
	case "down", "j":
		m.destinationOffset = clampOffset(m.destinationOffset+1, m.destinationMaxOffset())
	case "g":
		m.destinationOffset = 0
	case "G":
		m.destinationOffset = m.destinationMaxOffset()
	case "esc", "d":
		m.view = m.destinationsReturn
		m.destinationOffset = 0
	case "?":
		m.helpReturn = viewDestinations
		m.helpOffset = 0
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

func clampOffset(offset, max int) int {
	if offset < 0 {
		return 0
	}
	if offset > max {
		return max
	}
	return offset
}

func (m Model) updateHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m = m.clearNotice()
	switch msg.String() {
	case "up", "k":
		m.helpOffset = clampOffset(m.helpOffset-1, m.helpMaxOffset())
	case "down", "j":
		m.helpOffset = clampOffset(m.helpOffset+1, m.helpMaxOffset())
	case "pgup":
		m.helpOffset = clampOffset(m.helpOffset-m.helpBodyHeight(), m.helpMaxOffset())
	case "pgdown":
		m.helpOffset = clampOffset(m.helpOffset+m.helpBodyHeight(), m.helpMaxOffset())
	case "g":
		m.helpOffset = 0
	case "G":
		m.helpOffset = m.helpMaxOffset()
	case "?", "esc":
		m.view = m.helpReturn
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

func (m Model) updateConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "ctrl+c", "q":
		if m.running && m.opts.cancelAll != nil {
			m.opts.cancelAll()
		}
		return m, tea.Quit
	case "n", "N", "esc":
		m.confirmQuit = false
	}
	return m, nil
}

// cancelCurrent and cancelAll report whether a cancellation was requested.
func (m Model) cancelCurrent() bool {
	if !m.running || m.opts.cancelCurrent == nil {
		return false
	}
	m.opts.cancelCurrent()
	return true
}

func (m Model) cancelAll() bool {
	if !m.running || m.opts.cancelAll == nil {
		return false
	}
	m.opts.cancelAll()
	return true
}

// separator joins footer hints without introducing non-ASCII text.
func (m Model) separator() string {
	if m.opts.ascii {
		return " - "
	}
	return " · "
}

// footerHints returns the contextual key hints for the active page.
func (m Model) footerHints() string {
	sep := m.separator()
	if m.running {
		return "x cancel check" + sep + "X cancel run" + sep + "q quit"
	}
	switch m.view {
	case viewDetail:
		return "↑/k ↓/j scroll" + sep + "h/l check" + sep + "esc back" + sep + "? help"
	case viewDestinations, viewContext:
		return "j/k scroll" + sep + "esc back" + sep + "? help" + sep + "q quit"
	case viewPicker:
		if m.Layout() == LayoutCompact {
			return "space" + sep + "a all" + sep + "/ filter" + sep + "c custom" + sep + "enter" + sep + "esc"
		}
		return "space select" + sep + "a all" + sep + "/ filter" + sep + "c custom" + sep + "enter run" + sep + "esc back"
	case viewRun:
		if m.Layout() == LayoutCompact {
			return "h/l tgt" + sep + "1-3 view" + sep + "x/X cancel" + sep + "? help"
		}
		return "h/l target" + sep + "1/2/3 view" + sep + "j/k scroll" + sep + "x/X cancel" + sep + "? help"
	case viewHelp:
		return "j/k scroll" + sep + "? close" + sep + "esc back"
	default:
		if m.Layout() == LayoutCompact {
			return "↑↓ move" + sep + "enter open" + sep + "t targets" + sep + "q quit"
		}
		return "↑/k ↓/j move" + sep + "enter open" + sep + "t targets" + sep + "? help" + sep + "q quit"
	}
}

// helpLines documents every key and the verdict semantics. The overlay and the
// review checklist use this list as the single source of truth.
func helpLines() []string {
	return []string{
		" HELP",
		" NAVIGATION",
		"   ↑/k ↓/j    move or scroll",
		"   g / G      first / last",
		"   h/l [ ]    previous / next check",
		"   n / p      next / previous problem",
		"   PgUp/PgDn  scroll one page",
		"   enter      open · esc back",
		"   c / d      context / destinations",
		"   t          choose targets",
		" ACTIONS",
		"   r          run the baseline again",
		"   e / E      export default / path",
		"   x / X      cancel check / baseline",
		"   q/ctrl+c   quit, ctrl+c confirms",
		" VERDICT",
		"   healthy    all passed or skipped",
		"   degraded   warn, isolated anchor, no pass",
		"   blocked    core fail, all anchors fail",
	}
}
