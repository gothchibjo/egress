package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// noticeTTL is how long a status message stays in the footer.
const noticeTTL = 4 * time.Second

// noticeExpiredMsg clears the status message with the matching identity.
type noticeExpiredMsg struct {
	ID int
}

// setNotice shows a transient status message in the footer and schedules its
// expiry. The identity guard keeps an older timer from clearing a newer message.
func (m Model) setNotice(text string) (Model, tea.Cmd) {
	m.notice = text
	m.noticeID++
	id := m.noticeID
	return m, tea.Tick(noticeTTL, func(time.Time) tea.Msg {
		return noticeExpiredMsg{ID: id}
	})
}

// clearNotice removes the current status message and invalidates its timer.
func (m Model) clearNotice() Model {
	m.notice = ""
	m.noticeID++
	return m
}
