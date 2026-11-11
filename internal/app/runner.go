// Package app orchestrates baseline checks and owns the current run.
package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

// ErrAlreadyRunning is returned when a baseline is started twice at once.
var ErrAlreadyRunning = errors.New("baseline is already running")

// Check is one baseline check that produces evidence.
type Check interface {
	ID() domain.CheckID
	Run(context.Context) domain.Result
}

// Progress describes a check that has exceeded the slow-check threshold.
type Progress struct {
	CheckID domain.CheckID
	Index   int
	Total   int
	Elapsed time.Duration
}

// Run is one completed baseline.
type Run struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Results    []domain.Result
}

// Runner executes baseline checks one at a time.
type Runner struct {
	checks    []Check
	now       func() time.Time
	interval  time.Duration
	threshold time.Duration
	running   atomic.Bool
}

// Option configures a Runner.
type Option func(*Runner)

// WithClock replaces the time source, mainly for tests.
func WithClock(now func() time.Time) Option {
	return func(r *Runner) {
		if now != nil {
			r.now = now
		}
	}
}

// WithProgressInterval sets how often a slow check is reported.
func WithProgressInterval(d time.Duration) Option {
	return func(r *Runner) {
		if d > 0 {
			r.interval = d
		}
	}
}

// WithSlowCheckThreshold sets when progress reporting starts.
func WithSlowCheckThreshold(d time.Duration) Option {
	return func(r *Runner) {
		r.threshold = d
	}
}

// New returns a runner for the given checks.
func New(checks []Check, opts ...Option) *Runner {
	r := &Runner{
		checks:    append([]Check(nil), checks...),
		now:       time.Now,
		interval:  100 * time.Millisecond,
		threshold: time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run executes every check in order and returns one run. It never merges with a
// previous run, and it rejects a second concurrent invocation.
func (r *Runner) Run(ctx context.Context, progress func(Progress)) (Run, error) {
	if !r.running.CompareAndSwap(false, true) {
		return Run{}, ErrAlreadyRunning
	}
	defer r.running.Store(false)

	run := Run{
		StartedAt: r.now(),
		Results:   make([]domain.Result, 0, len(r.checks)),
	}
	for i, check := range r.checks {
		run.Results = append(run.Results, r.runCheck(ctx, check, i, progress))
	}
	run.FinishedAt = r.now()
	return run, nil
}

func (r *Runner) runCheck(ctx context.Context, check Check, index int, progress func(Progress)) domain.Result {
	start := r.now()
	done := make(chan domain.Result, 1)
	go func() {
		done <- check.Run(ctx)
	}()

	if progress == nil || r.interval <= 0 {
		return <-done
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case result := <-done:
			return result
		case <-ticker.C:
			elapsed := r.now().Sub(start)
			if elapsed >= r.threshold {
				progress(Progress{
					CheckID: check.ID(),
					Index:   index,
					Total:   len(r.checks),
					Elapsed: elapsed,
				})
			}
		}
	}
}

// Session keeps the latest run and replaces it on every re-run.
type Session struct {
	runner  *Runner
	mu      sync.RWMutex
	current *Run
}

// NewSession wraps a runner with the current-run store.
func NewSession(runner *Runner) *Session {
	return &Session{runner: runner}
}

// Run executes the baseline and makes its result the current run.
func (s *Session) Run(ctx context.Context, progress func(Progress)) (Run, error) {
	run, err := s.runner.Run(ctx, progress)
	if err != nil {
		return Run{}, err
	}
	s.mu.Lock()
	s.current = &run
	s.mu.Unlock()
	return run, nil
}

// Current returns the latest completed run.
func (s *Session) Current() (Run, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return Run{}, false
	}
	return *s.current, true
}
