// Package domain holds the network-diagnostic model shared by every adapter.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status is the outcome of a single baseline check.
type Status string

const (
	StatusPass    Status = "pass"
	StatusWarn    Status = "warn"
	StatusFail    Status = "fail"
	StatusSkipped Status = "skipped"
)

// Valid reports whether s is one of the four defined statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusPass, StatusWarn, StatusFail, StatusSkipped:
		return true
	default:
		return false
	}
}

// CheckID identifies a baseline check.
type CheckID string

// Measurement is one named value observed by a check.
type Measurement struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

// Result is the evidence emitted by one baseline check.
type Result struct {
	CheckID      CheckID       `json:"check_id"`
	Status       Status        `json:"status"`
	Target       string        `json:"target"`
	Method       string        `json:"method"`
	Measurements []Measurement `json:"measurements"`
	Timestamp    time.Time     `json:"timestamp"`
	Diagnostic   string        `json:"diagnostic,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// Validation errors returned by Validate and NewResult.
var (
	ErrEmptyCheckID       = errors.New("check id must not be empty")
	ErrInvalidStatus      = errors.New("status must be pass, warn, fail or skipped")
	ErrEmptyTarget        = errors.New("target must not be empty")
	ErrEmptyMethod        = errors.New("method must not be empty")
	ErrMissingTimestamp   = errors.New("timestamp must be set")
	ErrInvalidMeasurement = errors.New("measurement name and value must not be empty")
	ErrMissingEvidence    = errors.New("warn or fail requires a measured value or error")
	ErrMissingReason      = errors.New("skipped requires a diagnostic or error reason")
)

// HasEvidence reports whether the result carries a measured value or error.
func (r Result) HasEvidence() bool {
	return len(r.Measurements) > 0 || strings.TrimSpace(r.Error) != ""
}

// Validate rejects results that cannot explain their own status.
func (r Result) Validate() error {
	if strings.TrimSpace(string(r.CheckID)) == "" {
		return ErrEmptyCheckID
	}
	if !r.Status.Valid() {
		return ErrInvalidStatus
	}
	if strings.TrimSpace(r.Target) == "" {
		return ErrEmptyTarget
	}
	if strings.TrimSpace(r.Method) == "" {
		return ErrEmptyMethod
	}
	if r.Timestamp.IsZero() {
		return ErrMissingTimestamp
	}
	for i, m := range r.Measurements {
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Value) == "" {
			return fmt.Errorf("%w: measurement %d", ErrInvalidMeasurement, i)
		}
	}
	switch r.Status {
	case StatusWarn, StatusFail:
		if !r.HasEvidence() {
			return ErrMissingEvidence
		}
	case StatusSkipped:
		if strings.TrimSpace(r.Diagnostic) == "" && strings.TrimSpace(r.Error) == "" {
			return ErrMissingReason
		}
	}
	return nil
}

// NewResult returns a validated result or the first validation error.
func NewResult(r Result) (Result, error) {
	if err := r.Validate(); err != nil {
		return Result{}, err
	}
	return r, nil
}
