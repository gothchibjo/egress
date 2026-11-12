package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
)

// Layout thresholds for the dashboard.
const (
	NormalMinWidth   = 80
	NormalMinHeight  = 24
	CompactMinWidth  = 60
	CompactMinHeight = 20
	StopMinWidth     = 40
	StopMinHeight    = 10
)

// Layout is the dashboard form selected for the current terminal size.
type Layout int

const (
	// LayoutStopped means the terminal is below the minimum supported size and
	// nothing is rendered.
	LayoutStopped Layout = iota
	// LayoutTooSmall means the terminal is below the compact layout minimum but
	// still above the absolute minimum.
	LayoutTooSmall
	// LayoutCompact stacks the list above the evidence pane.
	LayoutCompact
	// LayoutNormal shows the list beside the evidence pane.
	LayoutNormal
)

type options struct {
	color         bool
	ascii         bool
	elapsed       time.Duration
	hasElapsed    bool
	mode          string
	cancelCurrent func()
	cancelAll     func()
}

// Option configures the dashboard rendering.
type Option func(*options)

// WithoutColor disables all ANSI styling.
func WithoutColor() Option {
	return func(o *options) { o.color = false }
}

// WithColor forces ANSI styling on, mainly for tests and non-TTY rendering.
func WithColor() Option {
	return func(o *options) { o.color = true }
}

// WithASCII replaces Unicode markers and separators with ASCII equivalents.
func WithASCII() Option {
	return func(o *options) { o.ascii = true }
}

// WithElapsed shows the baseline duration in the header.
func WithElapsed(d time.Duration) Option {
	return func(o *options) { o.elapsed = d; o.hasElapsed = true }
}

// WithMode labels the session as elevated or unprivileged.
func WithMode(mode string) Option {
	return func(o *options) { o.mode = mode }
}

// WithCancelHooks wires the runner cancellation controls into the keyboard.
func WithCancelHooks(cancelCurrent, cancelAll func()) Option {
	return func(o *options) {
		o.cancelCurrent = cancelCurrent
		o.cancelAll = cancelAll
	}
}

// Model is the dashboard. It renders a completed run, keeps the selected check
// across resizes and handles keyboard navigation.
type Model struct {
	results     []domain.Result
	verdict     domain.Verdict
	hasVerdict  bool
	selected    int
	width       int
	height      int
	view        viewState
	helpReturn  viewState
	confirmQuit bool
	running     bool
	cancelled   bool
	progress    string
	runErr      string
	opts        options
}

// New builds a dashboard for one run. The verdict is produced by
// domain.AggregateVerdict; pass a zero verdict when the run has none.
func New(results []domain.Result, verdict domain.Verdict, opts ...Option) Model {
	model := Model{
		results:    append([]domain.Result(nil), results...),
		verdict:    verdict,
		hasVerdict: verdict.Level != "",
		width:      80,
		height:     24,
		opts:       options{color: colorEnabled()},
	}
	for _, opt := range opts {
		opt(&model.opts)
	}
	model.clampSelection()
	return model
}

// colorEnabled honors NO_COLOR and a dumb terminal.
func colorEnabled() bool {
	if value := os.Getenv("NO_COLOR"); value != "" {
		return false
	}
	return !strings.EqualFold(os.Getenv("TERM"), "dumb")
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.Resize(msg.Width, msg.Height), nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	case RunProgressMsg:
		m.running = true
		m.progress = fmt.Sprintf(
			"%s (%d/%d) %s",
			msg.Progress.CheckID,
			msg.Progress.Index+1,
			msg.Progress.Total,
			msg.Progress.Elapsed.Round(100*time.Millisecond),
		)
		return m, nil
	case RunCompleteMsg:
		if msg.Err != nil {
			return m.failRun(msg.Err), nil
		}
		return m.finishRun(msg.Run), nil
	default:
		return m, nil
	}
}

// StartRunning marks the dashboard as waiting for the baseline.
func (m Model) StartRunning() Model {
	m.running = true
	m.cancelled = false
	m.progress = ""
	m.runErr = ""
	return m
}

// finishRun replaces the dashboard content with a completed run.
func (m Model) finishRun(run app.Run) Model {
	m.running = false
	m.cancelled = run.Cancelled
	m.progress = ""
	m.runErr = ""
	m.results = append([]domain.Result(nil), run.Results...)
	m.selected = 0
	m.verdict = domain.Verdict{}
	m.hasVerdict = false
	if verdict, err := domain.AggregateVerdict(run.Results); err == nil {
		m.verdict = verdict
		m.hasVerdict = true
	}
	if elapsed := run.FinishedAt.Sub(run.StartedAt); elapsed > 0 {
		m.opts.elapsed = elapsed
		m.opts.hasElapsed = true
	}
	return m
}

// failRun keeps the dashboard visible and records the baseline failure.
func (m Model) failRun(err error) Model {
	m.running = false
	m.progress = ""
	m.runErr = err.Error()
	return m
}

// Resize sets the terminal size and keeps the selected check.
func (m Model) Resize(width, height int) Model {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	m.width = width
	m.height = height
	m.clampSelection()
	return m
}

// Select moves the selection to index, clamped to the result range.
func (m Model) Select(index int) Model {
	m.selected = index
	m.clampSelection()
	return m
}

// Selected returns the selected index in display order.
func (m Model) Selected() int {
	return m.selected
}

// SelectedResult returns the selected result.
func (m Model) SelectedResult() (domain.Result, bool) {
	grouped := rows(m.results)
	if len(grouped) == 0 {
		return domain.Result{}, false
	}
	return grouped[m.selected].result, true
}

// Layout reports the form that View renders for the current size.
func (m Model) Layout() Layout {
	switch {
	case m.width < StopMinWidth || m.height < StopMinHeight:
		return LayoutStopped
	case m.width < CompactMinWidth || m.height < CompactMinHeight:
		return LayoutTooSmall
	case m.width < NormalMinWidth || m.height < NormalMinHeight:
		return LayoutCompact
	default:
		return LayoutNormal
	}
}

func (m *Model) clampSelection() {
	count := len(rows(m.results))
	switch {
	case count == 0:
		m.selected = 0
	case m.selected < 0:
		m.selected = 0
	case m.selected >= count:
		m.selected = count - 1
	}
}

// View implements tea.Model.
func (m Model) View() string {
	if m.confirmQuit {
		return m.renderConfirm()
	}
	switch m.view {
	case viewHelp:
		return m.renderHelp()
	case viewDetail:
		return m.renderDetailPage()
	}
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	case LayoutCompact:
		return m.renderCompact()
	default:
		return m.renderNormal()
	}
}

// move shifts the selection by delta, clamped to the result range.
func (m Model) move(delta int) Model {
	return m.Select(m.selected + delta)
}
