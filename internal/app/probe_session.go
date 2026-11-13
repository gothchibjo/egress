package app

import (
	"context"
	"errors"
	"sync"

	"github.com/gothchibjo/egress/internal/domain"
)

// MaxConcurrentTargets is the upper bound on targets probed at once.
const MaxConcurrentTargets = 4

// ErrProbeSessionRunning reports a second concurrent probing session.
var ErrProbeSessionRunning = errors.New("a probing session is already running")

// ProbeLadder runs the sequential ladder for one target.
type ProbeLadder interface {
	Run(ctx context.Context, target domain.ProbeTarget) []domain.StepResult
}

// StepReporter is an optional ladder that reports each finished step.
type StepReporter interface {
	RunWithProgress(ctx context.Context, target domain.ProbeTarget, onStep func(domain.StepResult)) []domain.StepResult
}

// TargetState is the lifecycle of one target inside a session.
type TargetState string

const (
	TargetPending   TargetState = "pending"
	TargetRunning   TargetState = "running"
	TargetDone      TargetState = "done"
	TargetCancelled TargetState = "cancelled"
)

// TargetRun is the outcome of one target.
type TargetRun struct {
	Target domain.ProbeTarget
	State  TargetState
	Steps  []domain.StepResult
}

// ProbeRun is one probing session.
type ProbeRun struct {
	Targets   []TargetRun
	Cancelled bool
}

// ProbeProgress reports that a worker picked up a target.
type ProbeProgress struct {
	TargetID string
	Index    int
	Total    int
}

// ProbeSession schedules target ladders with a bounded worker pool.
type ProbeSession struct {
	ladder      ProbeLadder
	concurrency int

	// OnStep receives every finished step when the ladder reports them.
	OnStep func(targetID string, step domain.StepResult)

	mu            sync.Mutex
	running       bool
	runCancel     context.CancelFunc
	targetCancels map[string]context.CancelFunc
}

// ProbeOption configures a ProbeSession.
type ProbeOption func(*ProbeSession)

// WithConcurrency bounds the worker pool. Values outside 1..4 are clamped.
func WithConcurrency(workers int) ProbeOption {
	return func(s *ProbeSession) {
		if workers < 1 {
			workers = 1
		}
		if workers > MaxConcurrentTargets {
			workers = MaxConcurrentTargets
		}
		s.concurrency = workers
	}
}

// NewProbeSession builds a session around one ladder implementation.
func NewProbeSession(ladder ProbeLadder, opts ...ProbeOption) *ProbeSession {
	session := &ProbeSession{ladder: ladder, concurrency: MaxConcurrentTargets}
	for _, opt := range opts {
		opt(session)
	}
	return session
}

// Run probes the targets with at most MaxConcurrentTargets workers and returns
// the results in the input order. A cancelled target keeps the steps it
// collected before cancellation.
func (s *ProbeSession) Run(ctx context.Context, targets []domain.ProbeTarget, progress func(ProbeProgress)) (ProbeRun, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ProbeRun{}, ErrProbeSessionRunning
	}
	s.running = true
	runCtx, cancel := context.WithCancel(ctx)
	s.runCancel = cancel
	s.targetCancels = make(map[string]context.CancelFunc, len(targets))
	s.mu.Unlock()

	defer func() {
		cancel()
		s.mu.Lock()
		s.running = false
		s.runCancel = nil
		s.targetCancels = nil
		s.mu.Unlock()
	}()

	run := ProbeRun{Targets: make([]TargetRun, len(targets))}
	for i, target := range targets {
		run.Targets[i] = TargetRun{Target: target, State: TargetPending, Steps: []domain.StepResult{}}
	}
	if len(targets) == 0 {
		return run, nil
	}

	workers := s.concurrency
	if workers > len(targets) {
		workers = len(targets)
	}

	jobs := make(chan int)
	var workersDone sync.WaitGroup
	var resultMu sync.Mutex

	for worker := 0; worker < workers; worker++ {
		workersDone.Add(1)
		go func() {
			defer workersDone.Done()
			for index := range jobs {
				target := targets[index]
				targetCtx, targetCancel := context.WithCancel(runCtx)

				s.mu.Lock()
				s.targetCancels[target.ID] = targetCancel
				s.mu.Unlock()

				resultMu.Lock()
				run.Targets[index].State = TargetRunning
				resultMu.Unlock()

				if progress != nil {
					progress(ProbeProgress{TargetID: target.ID, Index: index, Total: len(targets)})
				}

				var steps []domain.StepResult
				if reporter, ok := s.ladder.(StepReporter); ok && s.OnStep != nil {
					steps = reporter.RunWithProgress(targetCtx, target, func(step domain.StepResult) {
						s.OnStep(target.ID, step)
					})
				} else {
					steps = s.ladder.Run(targetCtx, target)
				}

				state := TargetDone
				if targetCtx.Err() != nil {
					state = TargetCancelled
				}

				resultMu.Lock()
				run.Targets[index].Steps = steps
				run.Targets[index].State = state
				if runCtx.Err() != nil {
					run.Cancelled = true
				}
				resultMu.Unlock()

				s.mu.Lock()
				delete(s.targetCancels, target.ID)
				s.mu.Unlock()
				targetCancel()
			}
		}()
	}

	for index := range targets {
		jobs <- index
	}
	close(jobs)
	workersDone.Wait()

	return run, nil
}

// CancelTarget stops one running target and leaves the others alone.
func (s *ProbeSession) CancelTarget(id string) bool {
	s.mu.Lock()
	cancel, ok := s.targetCancels[id]
	s.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// CancelAll stops every target.
func (s *ProbeSession) CancelAll() bool {
	s.mu.Lock()
	cancel := s.runCancel
	s.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}
