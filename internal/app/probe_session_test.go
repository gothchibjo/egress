package app

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

type fakeLadder struct {
	run func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult
}

func (f fakeLadder) Run(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
	return f.run(ctx, target)
}

func probeTargets(count int) []domain.ProbeTarget {
	targets := make([]domain.ProbeTarget, 0, count)
	for i := 0; i < count; i++ {
		targets = append(targets, domain.ProbeTarget{
			ID:    fmt.Sprintf("target-%d", i),
			Label: fmt.Sprintf("Target %d", i),
			Host:  "example.com",
			Ports: []int{443},
		})
	}
	return targets
}

func probeStep(target domain.ProbeTarget) domain.StepResult {
	return domain.StepResult{
		ID:        target.ID + ".step",
		Kind:      domain.StepTCP,
		Target:    target.Host,
		StartedAt: time.Unix(0, 0),
		Status:    domain.StatusPass,
		Measurements: []domain.Measurement{
			{Name: "outcome", Value: "connected"},
		},
	}
}

func TestProbeSessionCapsConcurrency(t *testing.T) {
	var mu sync.Mutex
	active, maxActive := 0, 0

	ladder := fakeLadder{run: func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()

		time.Sleep(5 * time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()
		return []domain.StepResult{probeStep(target)}
	}}

	session := NewProbeSession(ladder)
	run, err := session.Run(context.Background(), probeTargets(10), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	mu.Lock()
	peak := maxActive
	mu.Unlock()
	if peak > MaxConcurrentTargets {
		t.Fatalf("peak concurrency = %d, want at most %d", peak, MaxConcurrentTargets)
	}
	if peak < 2 {
		t.Fatalf("peak concurrency = %d, want parallel workers", peak)
	}
	if len(run.Targets) != 10 {
		t.Fatalf("targets = %d, want 10", len(run.Targets))
	}
	for i, target := range run.Targets {
		if target.Target.ID != fmt.Sprintf("target-%d", i) {
			t.Fatalf("target %d = %q: order changed", i, target.Target.ID)
		}
		if target.State != TargetDone {
			t.Fatalf("target %d state = %q", i, target.State)
		}
		if len(target.Steps) != 1 {
			t.Fatalf("target %d steps = %d", i, len(target.Steps))
		}
	}
}

func TestProbeSessionCancelsOneTarget(t *testing.T) {
	started := make(chan struct{}, 2)
	ladder := fakeLadder{run: func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
		steps := []domain.StepResult{probeStep(target)}
		if target.ID == "target-1" {
			started <- struct{}{}
			<-ctx.Done()
		}
		return steps
	}}

	session := NewProbeSession(ladder)
	done := make(chan ProbeRun, 1)
	go func() {
		run, _ := session.Run(context.Background(), probeTargets(2), nil)
		done <- run
	}()

	<-started
	if !session.CancelTarget("target-1") {
		t.Fatal("CancelTarget() reported no running target")
	}
	if session.CancelTarget("missing") {
		t.Fatal("CancelTarget() accepted an unknown target")
	}

	run := <-done
	if run.Targets[0].State != TargetDone {
		t.Fatalf("target 0 state = %q, want done", run.Targets[0].State)
	}
	if run.Targets[1].State != TargetCancelled {
		t.Fatalf("target 1 state = %q, want cancelled", run.Targets[1].State)
	}
	if len(run.Targets[1].Steps) != 1 {
		t.Fatalf("cancelled target kept %d steps, want 1", len(run.Targets[1].Steps))
	}
	if run.Cancelled {
		t.Fatal("one cancelled target marked the whole session cancelled")
	}
}

func TestProbeSessionCancelsAll(t *testing.T) {
	started := make(chan struct{}, MaxConcurrentTargets)
	ladder := fakeLadder{run: func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
		started <- struct{}{}
		<-ctx.Done()
		return []domain.StepResult{probeStep(target)}
	}}

	session := NewProbeSession(ladder)
	done := make(chan ProbeRun, 1)
	go func() {
		run, _ := session.Run(context.Background(), probeTargets(4), nil)
		done <- run
	}()

	for i := 0; i < MaxConcurrentTargets; i++ {
		<-started
	}
	if !session.CancelAll() {
		t.Fatal("CancelAll() reported no running session")
	}

	run := <-done
	if !run.Cancelled {
		t.Fatal("session is not marked cancelled")
	}
	for i, target := range run.Targets {
		if target.State != TargetCancelled {
			t.Fatalf("target %d state = %q", i, target.State)
		}
	}
}

func TestProbeSessionBoundedByTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ladder := fakeLadder{run: func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
		select {
		case <-ctx.Done():
		case <-time.After(time.Hour):
		}
		return nil
	}}

	session := NewProbeSession(ladder)
	start := time.Now()
	run, err := session.Run(ctx, probeTargets(2), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("hung probes stalled the session for %s", elapsed)
	}
	if !run.Cancelled {
		t.Fatal("a deadline-exceeded session is not marked cancelled")
	}
	for _, target := range run.Targets {
		if target.State != TargetCancelled {
			t.Fatalf("target %q state = %q", target.Target.ID, target.State)
		}
	}
}

func TestProbeSessionRejectsConcurrentRun(t *testing.T) {
	blocked := make(chan struct{})
	release := make(chan struct{})
	ladder := fakeLadder{run: func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
		close(blocked)
		<-release
		return nil
	}}

	session := NewProbeSession(ladder)
	done := make(chan struct{})
	go func() {
		_, _ = session.Run(context.Background(), probeTargets(1), nil)
		close(done)
	}()
	<-blocked

	if _, err := session.Run(context.Background(), probeTargets(1), nil); err != ErrProbeSessionRunning {
		t.Fatalf("second Run() error = %v, want %v", err, ErrProbeSessionRunning)
	}

	close(release)
	<-done
}

func TestProbeSessionProgress(t *testing.T) {
	var mu sync.Mutex
	seen := make(map[string]bool, 3)

	ladder := fakeLadder{run: func(ctx context.Context, target domain.ProbeTarget) []domain.StepResult {
		return nil
	}}
	session := NewProbeSession(ladder)
	_, err := session.Run(context.Background(), probeTargets(3), func(progress ProbeProgress) {
		mu.Lock()
		seen[progress.TargetID] = true
		mu.Unlock()
		if progress.Total != 3 {
			t.Errorf("progress total = %d", progress.Total)
		}
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, target := range probeTargets(3) {
		if !seen[target.ID] {
			t.Fatalf("no progress for %q", target.ID)
		}
	}
}
