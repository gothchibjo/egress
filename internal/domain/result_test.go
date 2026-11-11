package domain

import (
	"errors"
	"testing"
	"time"
)

func baseResult() Result {
	return Result{
		CheckID:   "local.route",
		Status:    StatusPass,
		Target:    "default-route",
		Method:    "inspection",
		Timestamp: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Result)
		want   error
	}{
		{
			name:   "pass without measurements",
			mutate: func(*Result) {},
		},
		{
			name: "warn with measurement",
			mutate: func(r *Result) {
				r.Status = StatusWarn
				r.Measurements = []Measurement{{Name: "rtt_ms", Value: "12.4", Unit: "ms"}}
			},
		},
		{
			name: "warn with error",
			mutate: func(r *Result) {
				r.Status = StatusWarn
				r.Error = "portal redirect observed"
			},
		},
		{
			name: "fail with error",
			mutate: func(r *Result) {
				r.Status = StatusFail
				r.Error = "connection refused"
			},
		},
		{
			name: "skipped with diagnostic",
			mutate: func(r *Result) {
				r.Status = StatusSkipped
				r.Diagnostic = "raw sockets unavailable without elevation"
			},
		},
		{
			name: "skipped with error",
			mutate: func(r *Result) {
				r.Status = StatusSkipped
				r.Error = "no default gateway"
			},
		},
		{
			name:   "empty check id",
			mutate: func(r *Result) { r.CheckID = "  " },
			want:   ErrEmptyCheckID,
		},
		{
			name:   "invalid status",
			mutate: func(r *Result) { r.Status = "unknown" },
			want:   ErrInvalidStatus,
		},
		{
			name:   "empty target",
			mutate: func(r *Result) { r.Target = "" },
			want:   ErrEmptyTarget,
		},
		{
			name:   "empty method",
			mutate: func(r *Result) { r.Method = " " },
			want:   ErrEmptyMethod,
		},
		{
			name:   "zero timestamp",
			mutate: func(r *Result) { r.Timestamp = time.Time{} },
			want:   ErrMissingTimestamp,
		},
		{
			name: "measurement without name",
			mutate: func(r *Result) {
				r.Measurements = []Measurement{{Value: "12.4"}}
			},
			want: ErrInvalidMeasurement,
		},
		{
			name: "measurement without value",
			mutate: func(r *Result) {
				r.Measurements = []Measurement{{Name: "rtt_ms"}}
			},
			want: ErrInvalidMeasurement,
		},
		{
			name: "warn without evidence",
			mutate: func(r *Result) {
				r.Status = StatusWarn
				r.Diagnostic = "looks odd"
			},
			want: ErrMissingEvidence,
		},
		{
			name: "fail without evidence",
			mutate: func(r *Result) {
				r.Status = StatusFail
			},
			want: ErrMissingEvidence,
		},
		{
			name: "skipped without reason",
			mutate: func(r *Result) {
				r.Status = StatusSkipped
			},
			want: ErrMissingReason,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := baseResult()
			tt.mutate(&r)

			got := r.Validate()
			if !errors.Is(got, tt.want) {
				t.Fatalf("Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewResult(t *testing.T) {
	valid, err := NewResult(baseResult())
	if err != nil {
		t.Fatalf("NewResult() error = %v", err)
	}
	if valid.CheckID != "local.route" {
		t.Fatalf("NewResult() check id = %q", valid.CheckID)
	}

	invalid := baseResult()
	invalid.Status = StatusFail
	if _, err := NewResult(invalid); !errors.Is(err, ErrMissingEvidence) {
		t.Fatalf("NewResult() error = %v, want %v", err, ErrMissingEvidence)
	}
}
