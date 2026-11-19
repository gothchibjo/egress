package report

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// SessionSchemaVersion identifies the probing-session JSON envelope.
const SessionSchemaVersion = "session-v1"

// ErrInvalidSession reports a session envelope that cannot be exported.
var ErrInvalidSession = errors.New("invalid session export")

//go:embed schema/session-v1.schema.json
var sessionSchema []byte

// SessionTarget is one probed target with its full evidence.
type SessionTarget struct {
	ID         string                  `json:"id"`
	Label      string                  `json:"label,omitempty"`
	Host       string                  `json:"host"`
	Ports      []int                   `json:"ports,omitempty"`
	Protocols  []domain.PresetProtocol `json:"protocols,omitempty"`
	State      string                  `json:"state"`
	Steps      []domain.StepResult     `json:"steps"`
	Path       domain.PathReport       `json:"path"`
	Hypothesis domain.Hypothesis       `json:"hypothesis"`
}

// SessionEnvelope is the stable JSON document of one probing session.
type SessionEnvelope struct {
	SchemaVersion  string                      `json:"schema_version"`
	StartedAt      time.Time                   `json:"started_at"`
	FinishedAt     time.Time                   `json:"finished_at"`
	Mode           string                      `json:"mode"`
	CatalogVersion string                      `json:"catalog_version"`
	Cancelled      bool                        `json:"cancelled,omitempty"`
	Targets        []SessionTarget             `json:"targets"`
	Destinations   []domain.SessionDestination `json:"destinations,omitempty"`
	Context        *platform.NetworkContext    `json:"context,omitempty"`
}

// SessionSchema returns the JSON Schema that describes SessionEnvelope.
func SessionSchema() []byte {
	return append([]byte(nil), sessionSchema...)
}

// BuildSessionEnvelope assembles a session document from the finished targets.
func BuildSessionEnvelope(targets []SessionTarget, started, finished time.Time, mode string, cancelled bool, destinations []domain.SessionDestination) SessionEnvelope {
	return SessionEnvelope{
		StartedAt:      started,
		FinishedAt:     finished,
		Mode:           mode,
		CatalogVersion: domain.CatalogVersion,
		Cancelled:      cancelled,
		Targets:        targets,
		Destinations:   destinations,
	}
}

// WriteSessionFile writes the session envelope to path and refuses to replace
// an existing file.
func WriteSessionFile(path string, envelope SessionEnvelope) error {
	return writeSessionFile(path, envelope, false)
}

// WriteSessionFileReplace writes the session envelope to path, replacing an
// existing file.
func WriteSessionFileReplace(path string, envelope SessionEnvelope) error {
	return writeSessionFile(path, envelope, true)
}

func writeSessionFile(path string, envelope SessionEnvelope, replace bool) error {
	return writeEncoded(path, func(w io.Writer) error {
		return EncodeSession(w, envelope)
	}, replace)
}

// EncodeSession validates one session envelope and writes it to w.
func EncodeSession(w io.Writer, envelope SessionEnvelope) error {
	envelope.SchemaVersion = SessionSchemaVersion

	if len(envelope.Targets) == 0 {
		return fmt.Errorf("%w: no targets", ErrInvalidSession)
	}
	if envelope.StartedAt.IsZero() || envelope.FinishedAt.IsZero() {
		return fmt.Errorf("%w: missing session timestamp", ErrInvalidSession)
	}
	if envelope.FinishedAt.Before(envelope.StartedAt) {
		return fmt.Errorf("%w: session finishes before it starts", ErrInvalidSession)
	}
	switch envelope.Mode {
	case "unprivileged", "elevated":
	default:
		return fmt.Errorf("%w: unknown mode %q", ErrInvalidSession, envelope.Mode)
	}
	if envelope.CatalogVersion == "" {
		envelope.CatalogVersion = domain.CatalogVersion
	}

	targets := make([]SessionTarget, len(envelope.Targets))
	for i, target := range envelope.Targets {
		if target.ID == "" {
			return fmt.Errorf("%w: target %d has no id", ErrInvalidSession, i)
		}
		if target.Host == "" {
			return fmt.Errorf("%w: target %q has no host", ErrInvalidSession, target.ID)
		}
		switch target.State {
		case "pending", "running", "done", "cancelled":
		default:
			return fmt.Errorf("%w: target %q has state %q", ErrInvalidSession, target.ID, target.State)
		}
		steps := make([]domain.StepResult, len(target.Steps))
		for j, step := range target.Steps {
			if err := step.Validate(); err != nil {
				return fmt.Errorf("%w: target %q: %v", ErrInvalidSession, target.ID, err)
			}
			if step.Measurements == nil {
				step.Measurements = []domain.Measurement{}
			}
			if step.Hops == nil {
				step.Hops = []domain.Hop{}
			}
			steps[j] = step
		}
		switch target.Path.Localization.Kind {
		case domain.PathComplete, domain.PathDrop, domain.PathInconclusive, domain.PathUnknown:
		default:
			return fmt.Errorf("%w: target %q has localization %q", ErrInvalidSession, target.ID, target.Path.Localization.Kind)
		}
		if target.Path.Hops == nil {
			target.Path.Hops = []domain.Hop{}
		}
		if err := target.Hypothesis.Validate(); err != nil {
			return fmt.Errorf("%w: target %q: %v", ErrInvalidSession, target.ID, err)
		}
		if target.Hypothesis.Rejected == nil {
			target.Hypothesis.Rejected = []domain.RejectedAlternative{}
		}
		target.Steps = steps
		targets[i] = target
	}
	envelope.Targets = targets

	for _, destination := range envelope.Destinations {
		if err := destination.Validate(); err != nil {
			return fmt.Errorf("%w: destination %q: %v", ErrInvalidSession, destination.Host, err)
		}
	}
	if envelope.Context != nil {
		context := *envelope.Context
		if context.Proxies == nil {
			context.Proxies = []platform.ProxySetting{}
		}
		if context.VPN == nil {
			context.VPN = []platform.VPNInterface{}
		}
		envelope.Context = &context
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(envelope); err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	return nil
}

// encodeSessionToBuffer is used by tests and by the write path.
func encodeSessionToBuffer(envelope SessionEnvelope) ([]byte, error) {
	var buffer bytes.Buffer
	if err := EncodeSession(&buffer, envelope); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
