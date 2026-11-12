// Package app orchestrates baseline checks and owns the current run.
package app

import (
	"context"
	"errors"
	"fmt"
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
	Cancelled  bool
}

// Runner executes baseline checks one at a time under a time budget.
type Runner struct {
	checks       []Check
	now          func() time.Time
	interval     time.Duration
	threshold    time.Duration
	budget       time.Duration
	checkTimeout time.Duration
	running      atomic.Bool

	mu               sync.Mutex
	runCancel        context.CancelFunc
	checkCancel      context.CancelFunc
	cancelRequested  atomic.Bool
	currentCancelled atomic.Bool
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

// WithBudget sets the overall baseline budget.
func WithBudget(d time.Duration) Option {
	return func(r *Runner) {
		r.budget = d
	}
}

// WithCheckTimeout sets the per-check timeout.
func WithCheckTimeout(d time.Duration) Option {
	return func(r *Runner) {
		r.checkTimeout = d
	}
}

// New returns a runner for the given checks.
func New(checks []Check, opts ...Option) *Runner {
	r := &Runner{
		checks:       append([]Check(nil), checks...),
		now:          time.Now,
		interval:     100 * time.Millisecond,
		threshold:    time.Second,
		budget:       30 * time.Second,
		checkTimeout: 8 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// CancelCurrent stops the check that is running and continues the baseline.
func (r *Runner) CancelCurrent() bool {
	r.mu.Lock()
	cancel := r.checkCancel
	r.mu.Unlock()
	if cancel == nil {
		return false
	}
	r.currentCancelled.Store(true)
	cancel()
	return true
}

// CancelAll stops the baseline and keeps the results collected so far.
func (r *Runner) CancelAll() bool {
	r.mu.Lock()
	cancel := r.runCancel
	r.mu.Unlock()
	if cancel == nil {
		return false
	}
	r.cancelRequested.Store(true)
	cancel()
	return true
}

// Run executes every check in order and returns one run. It never merges with a
// previous run, it rejects a second concurrent invocation and it bounds the run
// with the configured budget.
func (r *Runner) Run(ctx context.Context, progress func(Progress)) (Run, error) {
	if !r.running.CompareAndSwap(false, true) {
		return Run{}, ErrAlreadyRunning
	}
	defer r.running.Store(false)
	r.cancelRequested.Store(false)
	r.currentCancelled.Store(false)

	runCtx, cancelRun := context.WithCancel(ctx)
	if r.budget > 0 {
		runCtx, cancelRun = context.WithTimeout(ctx, r.budget)
	}
	defer cancelRun()

	r.mu.Lock()
	r.runCancel = cancelRun
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.runCancel = nil
		r.checkCancel = nil
		r.mu.Unlock()
	}()

	run := Run{
		StartedAt: r.now(),
		Results:   make([]domain.Result, 0, len(r.checks)),
	}
	for i, check := range r.checks {
		if runCtx.Err() != nil {
			run.Results = append(run.Results, r.skippedCheck(check, runCtx))
			continue
		}
		run.Results = append(run.Results, r.runCheck(runCtx, check, i, progress))
	}
	run.FinishedAt = r.now()
	run.Cancelled = r.cancelRequested.Load() || ctx.Err() != nil
	return run, nil
}

func (r *Runner) runCheck(runCtx context.Context, check Check, index int, progress func(Progress)) domain.Result {
	start := r.now()
	var checkCtx context.Context
	var cancelCheck context.CancelFunc
	if r.checkTimeout > 0 {
		checkCtx, cancelCheck = context.WithTimeout(runCtx, r.checkTimeout)
	} else {
		checkCtx, cancelCheck = context.WithCancel(runCtx)
	}
	defer cancelCheck()

	r.mu.Lock()
	r.checkCancel = cancelCheck
	r.currentCancelled.Store(false)
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.checkCancel = nil
		r.mu.Unlock()
	}()

	done := make(chan domain.Result, 1)
	go func() {
		done <- check.Run(checkCtx)
	}()

	if progress == nil || r.interval <= 0 {
		select {
		case result := <-done:
			return result
		case <-checkCtx.Done():
			return r.stoppedCheck(check, runCtx, done)
		}
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
		case <-checkCtx.Done():
			return r.stoppedCheck(check, runCtx, done)
		}
	}
}

// stoppedCheck classifies a check that did not return before its context ended.
func (r *Runner) stoppedCheck(check Check, runCtx context.Context, done <-chan domain.Result) domain.Result {
	select {
	case result := <-done:
		return result
	default:
	}

	switch {
	case r.currentCancelled.Load():
		return r.skippedResult(check.ID(), "check cancelled by user")
	case runCtx.Err() == nil:
		return r.timeoutResult(check, r.checkTimeout, "check exceeded its timeout")
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return r.timeoutResult(check, r.budget, "baseline budget exhausted while the check ran")
	default:
		return r.skippedResult(check.ID(), "baseline cancelled while the check ran")
	}
}

func (r *Runner) skippedCheck(check Check, runCtx context.Context) domain.Result {
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return r.skippedResult(check.ID(), "baseline budget exhausted before the check ran")
	}
	return r.skippedResult(check.ID(), "baseline cancelled before the check ran")
}

func (r *Runner) skippedResult(id domain.CheckID, reason string) domain.Result {
	return domain.Result{
		CheckID:    id,
		Status:     domain.StatusSkipped,
		Target:     string(id),
		Method:     "runner",
		Timestamp:  r.now(),
		Diagnostic: reason,
	}
}

func (r *Runner) timeoutResult(check Check, timeout time.Duration, reason string) domain.Result {
	value := timeout.Round(time.Millisecond).String()
	return domain.Result{
		CheckID:      check.ID(),
		Status:       domain.StatusFail,
		Target:       string(check.ID()),
		Method:       "runner timeout",
		Measurements: []domain.Measurement{{Name: "timeout", Value: value}},
		Timestamp:    r.now(),
		Diagnostic:   fmt.Sprintf("%s (%s)", reason, value),
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

// CancelCurrent stops the running check and continues the baseline.
func (s *Session) CancelCurrent() bool {
	return s.runner.CancelCurrent()
}

// CancelAll stops the baseline and keeps partial results.
func (s *Session) CancelAll() bool {
	return s.runner.CancelAll()
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
