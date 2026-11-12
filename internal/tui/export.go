package tui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/report"
)

// exportCurrent writes the default `egress-<timestamp>.json` file.
func (m Model) exportCurrent() Model {
	if len(m.results) == 0 {
		m.notice = "nothing to export: the baseline produced no results"
		return m
	}
	return m.writeExport(report.DefaultExportName(m.opts.now()), false)
}

// promptExport opens the path prompt with the default name filled in.
func (m Model) promptExport() Model {
	if len(m.results) == 0 {
		m.notice = "nothing to export: the baseline produced no results"
		return m
	}
	m.exportPrompt = true
	m.exportInput = report.DefaultExportName(m.opts.now())
	m.notice = ""
	return m
}

// writeExport stores the finished run and reports the outcome in the footer.
func (m Model) writeExport(path string, replace bool) Model {
	envelope := report.BuildEnvelope(m.results, m.opts.mode, m.opts.destinations)
	envelope.Cancelled = m.cancelled
	if m.opts.networkContext != nil {
		envelope.Context = m.opts.networkContext
	}
	err := m.opts.writeExport(path, envelope, replace)
	switch {
	case errors.Is(err, report.ErrExportExists):
		m.exportOverwrite = path
		m.notice = path + " exists; overwrite? y/n"
	case err != nil:
		m.notice = "export failed: " + err.Error()
	default:
		m.exportOverwrite = ""
		m.notice = "wrote " + path
	}
	return m
}

// updateExportKey handles the path prompt and the overwrite confirmation. Every
// key belongs to the modal, so typing a path cannot trigger dashboard actions.
func (m Model) updateExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.exportOverwrite != "" {
		switch msg.String() {
		case "y", "Y":
			path := m.exportOverwrite
			m.exportOverwrite = ""
			return m.writeExport(path, true), nil
		case "n", "N", "esc", "ctrl+c":
			m.exportOverwrite = ""
			m.notice = "export cancelled"
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.exportPrompt = false
		m.exportInput = ""
	case tea.KeyEnter:
		path := strings.TrimSpace(m.exportInput)
		if path == "" {
			m.notice = "export path is empty"
			return m, nil
		}
		m.exportPrompt = false
		m.exportInput = ""
		return m.writeExport(path, false), nil
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
