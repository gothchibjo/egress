package domain

import (
	"errors"
	"fmt"
	"time"
)

// StepKind names one probe in the per-target ladder.
type StepKind string

const (
	StepDNS   StepKind = "dns"
	StepTCP   StepKind = "tcp"
	StepTLS   StepKind = "tls"
	StepHTTP  StepKind = "http"
	StepICMP  StepKind = "icmp"
	StepTrace StepKind = "trace"
)

// StepResult is one independent probe of the ladder.
type StepResult struct {
	ID           string        `json:"id"`
	Kind         StepKind      `json:"kind"`
	Order        int           `json:"order"`
	Target       string        `json:"target"`
	StartedAt    time.Time     `json:"started_at"`
	DurationMS   int64         `json:"duration_ms"`
	Status       Status        `json:"status"`
	Summary      string        `json:"summary,omitempty"`
	Measurements []Measurement `json:"measurements"`
	Diagnostic   string        `json:"diagnostic,omitempty"`
	Error        string        `json:"error,omitempty"`
	// Hops carries the path table for the trace step.
	Hops []Hop `json:"hops,omitempty"`
}

// Errors raised by step validation.
var (
	ErrInvalidStep = errors.New("invalid ladder step")
)

// Validate checks that a step can explain its own status.
func (s StepResult) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidStep)
	}
	switch s.Kind {
	case StepDNS, StepTCP, StepTLS, StepHTTP, StepICMP, StepTrace:
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidStep, s.Kind)
	}
	switch s.Status {
	case StatusPass, StatusWarn, StatusFail, StatusSkipped:
	default:
		return fmt.Errorf("%w: invalid status %q", ErrInvalidStep, s.Status)
	}
	if s.Target == "" {
		return fmt.Errorf("%w: empty target", ErrInvalidStep)
	}
	if s.StartedAt.IsZero() {
		return fmt.Errorf("%w: zero timestamp", ErrInvalidStep)
	}
	switch s.Status {
	case StatusWarn, StatusFail:
		if len(s.Measurements) == 0 && s.Error == "" {
			return fmt.Errorf("%w: %s step %q has no evidence", ErrInvalidStep, s.Status, s.ID)
		}
	case StatusSkipped:
		if s.Diagnostic == "" {
			return fmt.Errorf("%w: skipped step %q has no reason", ErrInvalidStep, s.ID)
		}
	}
	return nil
}
