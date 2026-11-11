package app

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
)

type fakeCheck struct {
	id  domain.CheckID
	run func(context.Context) domain.Result
}

func (f fakeCheck) ID() domain.CheckID {
	return f.id
}

func (f fakeCheck) Run(ctx context.Context) domain.Result {
	return f.run(ctx)
}

func resultFor(id domain.CheckID, value string) domain.Result {
	return domain.Result{
		CheckID:      id,
		Status:       domain.StatusPass,
		Target:       "test-target",
		Method:       "test-method",
		Measurements: []domain.Measurement{{Name: "value", Value: value}},
		Timestamp:    time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
}

func TestRunnerRunsChecksInOrder(t *testing.T) {
	checks := []Check{
		fakeCheck{id: "one", run: func(context.Context) domain.Result { return resultFor("one", "1") }},
		fakeCheck{id: "two", run: func(context.Context) domain.Result { return resultFor("two", "2") }},
		fakeCheck{id: "three", run: func(context.Context) domain.Result { return resultFor("three", "3") }},
	}

	run, err := New(checks).Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(run.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(run.Results))
	}
	for i, want := range []domain.CheckID{"one", "two", "three"} {
		if run.Results[i].CheckID != want {
			t.Fatalf("result %d = %q, want %q", i, run.Results[i].CheckID, want)
		}
	}
	if run.StartedAt.IsZero() || run.FinishedAt.IsZero() {
		t.Fatalf("run timestamps are not set: %+v", run)
	}
}

func TestRunnerRejectsConcurrentRun(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	check := fakeCheck{id: "blocking", run: func(context.Context) domain.Result {
		close(started)
		<-release
		return resultFor("blocking", "1")
	}}
	runner := New([]Check{check})

	go func() {
		defer close(finished)
		if _, err := runner.Run(context.Background(), nil); err != nil {
			t.Errorf("first Run() error = %v", err)
		}
	}()

	<-started
	if _, err := runner.Run(context.Background(), nil); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Run() error = %v, want %v", err, ErrAlreadyRunning)
	}
	close(release)
	<-finished
}

func TestRunnerReportsSlowProgress(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	progress := make(chan Progress, 1)
	check := fakeCheck{id: "slow", run: func(context.Context) domain.Result {
		close(started)
		<-release
		return resultFor("slow", "1")
	}}
	runner := New(
		[]Check{check},
		WithProgressInterval(time.Millisecond),
		WithSlowCheckThreshold(0),
	)

	go func() {
		defer close(finished)
		if _, err := runner.Run(context.Background(), func(p Progress) {
			select {
			case progress <- p:
			default:
			}
		}); err != nil {
			t.Errorf("Run() error = %v", err)
		}
	}()

	<-started
	select {
	case p := <-progress:
		if p.CheckID != "slow" {
			t.Fatalf("progress check id = %q, want slow", p.CheckID)
		}
		if p.Total != 1 || p.Index != 0 {
			t.Fatalf("progress position = %d/%d, want 0/1", p.Index, p.Total)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no progress reported for a slow check")
	}

	close(release)
	<-finished
}

func TestSessionReplacesPreviousRun(t *testing.T) {
	var counter atomic.Int64
	check := fakeCheck{id: "counter", run: func(context.Context) domain.Result {
		n := counter.Add(1)
		return resultFor("counter", fmt.Sprintf("run-%d", n))
	}}
	session := NewSession(New([]Check{check}))

	first, err := session.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := session.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	current, ok := session.Current()
	if !ok {
		t.Fatal("Current() reports no run")
	}
	if len(current.Results) != 1 {
		t.Fatalf("current results = %d, want 1", len(current.Results))
	}
	value := current.Results[0].Measurements[0].Value
	if value != "run-2" {
		t.Fatalf("current value = %q, want run-2", value)
	}
	if first.Results[0].Measurements[0].Value != "run-1" {
		t.Fatalf("first run mutated: %q", first.Results[0].Measurements[0].Value)
	}
	if second.Results[0].Measurements[0].Value != "run-2" {
		t.Fatalf("second run value = %q", second.Results[0].Measurements[0].Value)
	}
}
