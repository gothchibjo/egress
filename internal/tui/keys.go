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
	viewRunDestinations
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
	case viewRunDestinations:
		return m.updateRunDestinationsKey(msg)
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

func (m Model) updateRunDestinationsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.runDestinationOffset = clampOffset(m.runDestinationOffset-1, m.runDestinationMaxOffset())
	case "down", "j":
		m.runDestinationOffset = clampOffset(m.runDestinationOffset+1, m.runDestinationMaxOffset())
	case "pgup":
		m.runDestinationOffset = clampOffset(m.runDestinationOffset-m.runDestinationBodyHeight(), m.runDestinationMaxOffset())
	case "pgdown":
		m.runDestinationOffset = clampOffset(m.runDestinationOffset+m.runDestinationBodyHeight(), m.runDestinationMaxOffset())
	case "g":
		m.runDestinationOffset = 0
	case "G":
		m.runDestinationOffset = m.runDestinationMaxOffset()
	case "esc", "d":
		m.view = viewRun
	case "?":
		m.helpReturn = viewRunDestinations
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
	case viewRunDestinations:
		return "j/k scroll" + sep + "esc back" + sep + "? help" + sep + "q quit"
	case viewPicker:
		if m.Layout() == LayoutCompact {
			return "space" + sep + "a all" + sep + "/ filter" + sep + "c custom" + sep + "enter" + sep + "esc"
		}
		return "space select" + sep + "a all" + sep + "/ filter" + sep + "c custom" + sep + "enter run" + sep + "esc back"
	case viewRun:
		if m.run.focus {
			return "↑/k ↓/j scroll" + sep + "g/G top/end" + sep + "h/l target" + sep + "e export" + sep + "r rerun" + sep + "esc back"
		}
		if m.Layout() == LayoutCompact || m.width < 100 {
			return "↑↓ target" + sep + "1-3 pane" + sep + "enter" + sep + "e export" + sep + "r rerun" + sep + "? help"
		}
		return "↑/k ↓/j target" + sep + "1/2/3 pane" + sep + "enter scroll" + sep + "c ctx" + sep + "d dest" + sep + "e export" + sep + "r rerun" + sep + "x/X cancel"
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
		"   h/l [ ]    previous / next check or target",
		"   n / p      next / previous problem",
		"   PgUp/PgDn  scroll one page",
		"   enter      open · esc back",
		"   c          proxy and tunnel context",
		"   d          run destinations",
		"   t          choose targets",
		" ACTIONS",
		"   r          run the baseline or probes again",
		"   e / E      export JSON (default / path)",
		"   x / X      cancel check / baseline",
		"   q/ctrl+c   quit, ctrl+c confirms",
		" PICKER",
		"   space      select or clear a preset",
		"   a          select or clear every visible preset",
		"   /          filter presets",
		"   c          add a custom target",
		"   enter      start the selected targets",
		" RUN",
		"   1/2/3      ladder · hops · verdict panes",
		"   tab        cycle the panes",
		"   enter      focus the pane for scrolling",
		"   c / d      proxy context / run destinations",
		"   e / E      export the session",
		"   r          re-run the selected targets",
		" LADDER",
		"   dns        system resolver and DoH comparison",
		"   tcp.<port> TCP connect result",
		"   tls.real   ClientHello with the target name",
		"   tls.decoy  ClientHello with a decoy name",
		"   tls.absent ClientHello without SNI",
		"   http       HTTP GET on the primary port",
		"   icmp       ICMP echo to the resolved address",
		"   trace      TTL-limited path discovery",
		"   ✓ ! ✗      pass · warn · fail;  · queued, … running",
		" HOPS",
		"   TTL        hop number",
		"   ADDRESS    every router that answered at the TTL",
		"   LOSS       share of probes without a reply",
		"   RTT        median round trip",
		"   → ! x      target · partial loss · replies stop",
		"   complete · drop · inconclusive localization",
		" TAXONOMY",
		"   dns-tampering      resolver and DoH answers differ",
		"   tcp-rst-injection  a TCP connect was reset",
		"   ip-drop            no reply above the last hop",
		"   port-block         some ports connect, others time out",
		"   sni-tls-filtering  real SNI fails, decoy succeeds",
		"   http-block         HTTP fails while TCP/TLS work",
		"   throttling         latency or loss above a baseline",
		"   unknown            evidence does not support a type",
		" CONFIDENCE",
		"   low                one observation or thin evidence",
		"   medium             a consistent active signal",
		"   high               several independent active signals",
		"   unknown            valid when evidence is insufficient",
		" CAVEAT",
		"   Every verdict keeps the vantage caveat: absence of",
		"   evidence is not proof; re-run from another network.",
		" VERDICT",
		"   healthy    all passed or skipped",
		"   degraded   warn, isolated anchor, no pass",
		"   blocked    core fail, all anchors fail",
	}
}
