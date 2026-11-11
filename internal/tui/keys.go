package tui

import tea "github.com/charmbracelet/bubbletea"

// viewState is the page the dashboard currently shows.
type viewState int

const (
	viewDashboard viewState = iota
	viewDetail
	viewHelp
)

// updateKey dispatches a key press to the active page. Every action has a key;
// no mouse handler exists, so keyboard control is complete.
func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmQuit {
		return m.updateConfirmKey(msg)
	}
	switch m.view {
	case viewHelp:
		return m.updateHelpKey(msg)
	case viewDetail:
		return m.updateDetailKey(msg)
	default:
		return m.updateDashboardKey(msg)
	}
}

func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		return m.move(-1), nil
	case "down", "j":
		return m.move(1), nil
	case "g":
		return m.Select(0), nil
	case "G":
		return m.Select(len(rows(m.results)) - 1), nil
	case "enter":
		if _, ok := m.SelectedResult(); ok {
			m.view = viewDetail
		}
	case "?":
		m.helpReturn = viewDashboard
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

func (m Model) updateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		return m.move(-1), nil
	case "down", "j":
		return m.move(1), nil
	case "g":
		return m.Select(0), nil
	case "G":
		return m.Select(len(rows(m.results)) - 1), nil
	case "esc":
		m.view = viewDashboard
	case "?":
		m.helpReturn = viewDetail
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m, nil
}

func (m Model) updateHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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
		return m, tea.Quit
	case "n", "N", "esc":
		m.confirmQuit = false
	}
	return m, nil
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
	switch m.view {
	case viewDetail:
		return "↑/k ↓/j move" + sep + "esc back" + sep + "? help" + sep + "q quit"
	case viewHelp:
		return "? close" + sep + "esc back"
	default:
		return "↑/k ↓/j move" + sep + "enter open" + sep + "? help" + sep + "q quit"
	}
}

// helpLines documents every key and the verdict semantics. The overlay and the
// review checklist use this list as the single source of truth.
func helpLines() []string {
	return []string{
		" HELP",
		" NAVIGATION",
		"   ↑ / k    move up",
		"   ↓ / j    move down",
		"   g / G    first / last check",
		"   enter    open the selected check",
		"   esc      go back",
		" ACTIONS",
		"   q        quit",
		"   ctrl+c   quit with confirmation",
		"   ?        close help",
		" VERDICT",
		"   healthy  every check passed or was skipped",
		"   degraded warn, isolated anchor fail or no pass",
		"   blocked  a core check failed or every anchor failed",
	}
}
