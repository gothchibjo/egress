//go:build darwin || linux

// Command egress runs the baseline and opens the interactive dashboard, or
// prints the JSON envelope for non-interactive use.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/app"
	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
	"github.com/gothchibjo/egress/internal/probe"
	"github.com/gothchibjo/egress/internal/report"
	"github.com/gothchibjo/egress/internal/tui"
)

// Documented exit codes shared by the dashboard and the JSON output.
const (
	exitHealthy  = 0
	exitDegraded = 1
	exitBlocked  = 2
	exitError    = 3
)

const usage = "usage: egress [--json] [--schema]"

// baselineRunner is the part of app.Session the command needs.
type baselineRunner interface {
	Run(ctx context.Context, progress func(app.Progress)) (app.Run, error)
	CancelCurrent() bool
	CancelAll() bool
}

var (
	detectPrivilege  = platform.DetectPrivilege
	readDestinations = probe.OutboundDestinations
	detectContext    = platform.DetectNetworkContext
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWith(args, stdout, stderr, func(elevated bool) baselineRunner {
		return app.NewSession(app.New(probe.BaselineChecks(probe.BaselineOptions{Elevated: elevated})))
	})
}

// cliOptions is the parsed command line.
type cliOptions struct {
	json   bool
	schema bool
}

func parseArgs(args []string) (cliOptions, error) {
	flags := flag.NewFlagSet("egress", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "emit the baseline as JSON on stdout")
	schema := flags.Bool("schema", false, "print the JSON schema and exit")
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if flags.NArg() > 0 {
		return cliOptions{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return cliOptions{json: *jsonOutput, schema: *schema}, nil
}

func runWith(args []string, stdout, stderr io.Writer, newSession func(elevated bool) baselineRunner) int {
	options, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(stdout, usage)
		return exitHealthy
	}
	if err != nil {
		fmt.Fprintln(stderr, "egress:", err)
		fmt.Fprintln(stderr, usage)
		return exitError
	}

	if options.schema {
		fmt.Fprintf(stdout, "%s\n", report.Schema())
		return exitHealthy
	}

	privilege := detectPrivilege()
	mode := "unprivileged"
	if privilege.Elevated {
		mode = "elevated"
	}
	destinations, destinationErr := readDestinations()
	if destinationErr != nil {
		fmt.Fprintln(stderr, "egress: destination list unavailable:", destinationErr)
	}
	networkContext, contextErr := detectContext()
	if contextErr != nil {
		fmt.Fprintln(stderr, "egress: interface list unavailable:", contextErr)
	}

	session := newSession(privilege.RawICMP)
	if options.json || !isTerminal(stdout) {
		return runJSON(stdout, stderr, session, mode, destinations, networkContext)
	}
	file, ok := stdout.(*os.File)
	if !ok {
		return runJSON(stdout, stderr, session, mode, destinations, networkContext)
	}
	return runTUI(file, stderr, session, mode, destinations, destinationErr, networkContext, contextErr)
}

// runJSON runs one baseline and prints the JSON envelope. It never touches the
// terminal, so the output stays free of control sequences.
func runJSON(stdout, stderr io.Writer, session baselineRunner, mode string, destinations []domain.Destination, networkContext platform.NetworkContext) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	baseline, err := session.Run(ctx, nil)
	if err != nil {
		fmt.Fprintln(stderr, "egress:", err)
		return exitError
	}
	envelope := report.BuildEnvelope(baseline.Results, mode, destinations)
	envelope.Cancelled = baseline.Cancelled
	envelope.Context = &networkContext
	if err := report.Encode(stdout, envelope); err != nil {
		fmt.Fprintln(stderr, "egress:", err)
		return exitError
	}
	return outcomeExit(baseline.Results)
}

// runTUI wires the baseline session to the dashboard and returns its outcome.
func runTUI(stdout *os.File, stderr io.Writer, session baselineRunner, mode string, destinations []domain.Destination, destinationErr error, networkContext platform.NetworkContext, contextErr error) int {
	model := tui.New(nil, domain.Verdict{},
		tui.WithMode(mode),
		tui.WithDestinations(destinations, destinationErr),
		tui.WithContext(networkContext, contextErr),
		tui.WithCancelHooks(
			func() { session.CancelCurrent() },
			func() { session.CancelAll() },
		),
	).StartRunning()
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(stdout), tea.WithInput(os.Stdin))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan app.Run, 1)
	go func() {
		baseline, runErr := session.Run(ctx, func(progress app.Progress) {
			program.Send(tui.RunProgressMsg{Progress: progress})
		})
		done <- baseline
		program.Send(tui.RunCompleteMsg{Run: baseline, Err: runErr})
	}()

	if _, err := program.Run(); err != nil {
		cancel()
		fmt.Fprintln(stderr, "egress:", err)
		return exitError
	}

	cancel()
	baseline := <-done
	if len(baseline.Results) == 0 {
		// Quitting before the first check finished is a normal user action, not
		// a failed run.
		return exitHealthy
	}
	return outcomeExit(baseline.Results)
}

// outcomeExit maps a completed run to its documented exit code.
func outcomeExit(results []domain.Result) int {
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		return exitError
	}
	switch verdict.Level {
	case domain.VerdictHealthy:
		return exitHealthy
	case domain.VerdictDegraded:
		return exitDegraded
	case domain.VerdictBlocked:
		return exitBlocked
	default:
		return exitError
	}
}

// isTerminal reports whether the writer is an interactive terminal.
func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
