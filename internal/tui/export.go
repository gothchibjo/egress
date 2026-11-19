package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/report"
)

// rerun starts the baseline again, replacing the previous results.
func (m Model) rerun() (Model, tea.Cmd) {
	if m.running {
		return m.setNotice("baseline is already running")
	}
	if m.opts.rerun == nil {
		return m.setNotice("re-run is unavailable in this session")
	}
	m.opts.rerun()
	return m.restart(), nil
}

// jumpProblem selects the next or previous check that needs attention. The
// walk follows the rendered order and wraps at both ends.
func (m Model) jumpProblem(delta int) (Model, tea.Cmd) {
	grouped := rows(m.results)
	problems := make([]int, 0, len(grouped))
	for i, entry := range grouped {
		if entry.result.Status == domain.StatusFail || entry.result.Status == domain.StatusWarn {
			problems = append(problems, i)
		}
	}
	if len(problems) == 0 {
		return m.setNotice("no failing or warning checks")
	}

	target := problems[0]
	if delta < 0 {
		target = problems[len(problems)-1]
		for i := len(problems) - 1; i >= 0; i-- {
			if problems[i] < m.selected {
				target = problems[i]
				break
			}
		}
	} else {
		for _, index := range problems {
			if index > m.selected {
				target = index
				break
			}
		}
	}
	return m.Select(target), nil
}

// exportCurrent writes the default `egress-<timestamp>.json` file.
func (m Model) exportCurrent() (Model, tea.Cmd) {
	if len(m.results) == 0 {
		return m.setNotice("nothing to export: the baseline produced no results")
	}
	m.exportSession = false
	return m.writeExport(report.DefaultExportName(m.opts.now()), false)
}

// promptExport opens the path prompt with the default name filled in.
func (m Model) promptExport() (Model, tea.Cmd) {
	if len(m.results) == 0 {
		return m.setNotice("nothing to export: the baseline produced no results")
	}
	m.exportSession = false
	m.exportPrompt = true
	m.exportInput = report.DefaultExportName(m.opts.now())
	return m, nil
}

// writeExport stores the finished baseline or probing session and reports the
// outcome in the footer.
func (m Model) writeExport(path string, replace bool) (Model, tea.Cmd) {
	if m.exportSession {
		return m.writeSessionExport(path, replace)
	}
	envelope := report.BuildEnvelope(m.results, m.opts.mode, m.opts.destinations)
	envelope.Cancelled = m.cancelled
	if m.opts.networkContext != nil {
		envelope.Context = m.opts.networkContext
	}

	err := m.opts.writeExport(path, envelope, replace)
	switch {
	case errors.Is(err, report.ErrExportExists):
		m.exportOverwrite = path
		return m.setNotice(path + " exists; overwrite? y/n")
	case err != nil:
		return m.setNotice("export failed: " + err.Error())
	default:
		m.exportOverwrite = ""
		return m.setNotice("wrote " + path)
	}
}

// exportRunCurrent writes the default session export from the run view.
func (m Model) exportRunCurrent() (Model, tea.Cmd) {
	if m.run.running {
		return m.setNotice("wait for the probe session to finish")
	}
	if m.run.err != "" {
		return m.setNotice("nothing to export: the probe session failed")
	}
	if len(m.run.targets) == 0 {
		return m.setNotice("nothing to export: no targets")
	}
	m.exportSession = true
	return m.writeExport(report.DefaultExportName(m.opts.now()), false)
}

// promptRunExport opens the path prompt for the probing session.
func (m Model) promptRunExport() (Model, tea.Cmd) {
	if m.run.running {
		return m.setNotice("wait for the probe session to finish")
	}
	if m.run.err != "" {
		return m.setNotice("nothing to export: the probe session failed")
	}
	if len(m.run.targets) == 0 {
		return m.setNotice("nothing to export: no targets")
	}
	m.exportSession = true
	m.exportPrompt = true
	m.exportInput = report.DefaultExportName(m.opts.now())
	return m, nil
}

// writeSessionExport stores the current probe session.
func (m Model) writeSessionExport(path string, replace bool) (Model, tea.Cmd) {
	err := m.opts.sessionExport(path, m.sessionEnvelope(), replace)
	switch {
	case errors.Is(err, report.ErrExportExists):
		m.exportOverwrite = path
		return m.setNotice(path + " exists; overwrite? y/n")
	case err != nil:
		return m.setNotice("export failed: " + err.Error())
	default:
		m.exportOverwrite = ""
		m.exportSession = false
		return m.setNotice("wrote " + path)
	}
}

// sessionEnvelope assembles the full session document from the run state.
func (m Model) sessionEnvelope() report.SessionEnvelope {
	targets := make([]report.SessionTarget, 0, len(m.run.targets))
	for _, target := range m.run.targets {
		path, hypothesis := target.path, target.hypothesis
		if path.Localization.Kind == "" || hypothesis.Type == "" {
			path, hypothesis = domain.BuildTargetReport(target.steps)
		}
		state := string(target.state)
		if state == "" {
			state = "done"
		}
		targets = append(targets, report.SessionTarget{
			ID:         target.target.ID,
			Label:      target.target.Label,
			Host:       target.target.Host,
			Ports:      target.target.Ports,
			Protocols:  target.target.Protocols,
			State:      state,
			Steps:      target.steps,
			Path:       path,
			Hypothesis: hypothesis,
		})
	}
	mode := m.opts.mode
	if mode == "" {
		mode = "unprivileged"
	}
	envelope := report.BuildSessionEnvelope(targets, m.run.startedAt, m.run.finishedAt, mode, m.run.cancelled, m.sessionDestinations())
	if m.opts.networkContext != nil {
		envelope.Context = m.opts.networkContext
	}
	return envelope
}

// sessionDestinations lists every endpoint the current run may contact.
func (m Model) sessionDestinations() []domain.SessionDestination {
	targets := make([]domain.ProbeTarget, 0, len(m.run.targets))
	steps := make(map[string][]domain.StepResult, len(m.run.targets))
	for _, target := range m.run.targets {
		targets = append(targets, target.target)
		steps[target.target.ID] = target.steps
	}
	return domain.BuildSessionDestinations(targets, steps, m.opts.destinations)
}

// updateExportKey handles the path prompt and the overwrite confirmation. Every
// key belongs to the modal, so typing a path cannot trigger dashboard actions.
func (m Model) updateExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.exportOverwrite != "" {
		switch msg.String() {
		case "y", "Y":
			path := m.exportOverwrite
			m.exportOverwrite = ""
			return m.writeExport(path, true)
		case "n", "N", "esc", "ctrl+c":
			m.exportOverwrite = ""
			m.exportSession = false
			return m.setNotice("export cancelled")
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.exportPrompt = false
		m.exportInput = ""
		m.exportSession = false
	case tea.KeyEnter:
		path := strings.TrimSpace(m.exportInput)
		if path == "" {
			return m.setNotice("export path is empty")
		}
		m.exportPrompt = false
		m.exportInput = ""
		return m.writeExport(path, false)
	case tea.KeyBackspace:
		if runes := []rune(m.exportInput); len(runes) > 0 {
			m.exportInput = string(runes[:len(runes)-1])
		}
	case tea.KeyRunes:
		m.exportInput += string(msg.Runes)
	}
	return m, nil
}

// exportFooter renders the modal footer, or nil when no export modal is open.
func (m Model) exportFooter() []string {
	if m.exportPrompt {
		return []string{
			m.rule(m.width),
			padLine(" export to: "+m.exportInput+"_",
				" enter write"+m.separator()+"esc cancel", m.width),
		}
	}
	return nil
}
