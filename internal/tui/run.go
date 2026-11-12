package tui

import "github.com/gothchibjo/egress/internal/app"

// RunProgressMsg reports a baseline check that exceeded the slow threshold.
type RunProgressMsg struct {
	Progress app.Progress
}

// RunCompleteMsg reports a finished baseline or the error that stopped it.
type RunCompleteMsg struct {
	Run app.Run
	Err error
}
