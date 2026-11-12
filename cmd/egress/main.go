// Command egress runs the baseline and opens the interactive dashboard.
package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/probe"
	"github.com/gothchibjo/egress/internal/tui"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr))
}

// run wires the baseline session to the dashboard and returns the exit code.
func run(stdout, stderr *os.File) int {
	if !isTerminal(stdout) {
		fmt.Fprintln(stderr, "egress: the interactive dashboard requires a terminal")
		return 3
	}

	checks := probe.BaselineChecks(probe.BaselineOptions{Elevated: os.Geteuid() == 0})
	session := app.NewSession(app.New(checks))
	model := tui.New(nil, domain.Verdict{}, tui.WithCancelHooks(
		func() { session.CancelCurrent() },
		func() { session.CancelAll() },
	)).StartRunning()
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(stdout), tea.WithInput(os.Stdin))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		baseline, err := session.Run(ctx, func(progress app.Progress) {
			program.Send(tui.RunProgressMsg{Progress: progress})
		})
		program.Send(tui.RunCompleteMsg{Run: baseline, Err: err})
	}()

	if _, err := program.Run(); err != nil {
		cancel()
		fmt.Fprintln(stderr, "egress:", err)
		return 1
	}
	cancel()
	return 0
}

// isTerminal reports whether the file is a character device.
func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
