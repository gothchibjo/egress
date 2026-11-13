package domain

import (
	"errors"
	"fmt"
	"strings"
)

// BlockageType is one member of the blockage taxonomy.
type BlockageType string

const (
	BlockDNS        BlockageType = "dns-tampering"
	BlockTCPRST     BlockageType = "tcp-rst-injection"
	BlockIPDrop     BlockageType = "ip-drop"
	BlockPortBlock  BlockageType = "port-block"
	BlockSNIFilter  BlockageType = "sni-tls-filtering"
	BlockHTTP       BlockageType = "http-block"
	BlockThrottling BlockageType = "throttling"
	BlockUnknown    BlockageType = "unknown"
)

// Confidence is the strength of one hypothesis.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// ErrInvalidHypothesis marks a hypothesis that cannot explain itself.
var ErrInvalidHypothesis = errors.New("invalid hypothesis")

// RejectedAlternative is a taxonomy member that was considered and dismissed.
type RejectedAlternative struct {
	Type   BlockageType `json:"type"`
	Reason string       `json:"reason"`
}

// Hypothesis is the ranked blockage finding for one target.
type Hypothesis struct {
	Type       BlockageType          `json:"type"`
	Confidence Confidence            `json:"confidence"`
	Summary    string                `json:"summary"`
	Evidence   []string              `json:"evidence"`
	Rejected   []RejectedAlternative `json:"rejected"`
	Caveat     string                `json:"caveat"`
}

// SingleVantageCaveat accompanies every hypothesis.
const SingleVantageCaveat = "Single vantage point: one observation from one network; absence of evidence is not proof of blocking."

// Validate checks that the hypothesis can explain itself.
func (h Hypothesis) Validate() error {
	if !h.Type.valid() {
		return fmt.Errorf("%w: unknown type %q", ErrInvalidHypothesis, h.Type)
	}
	switch h.Confidence {
	case ConfidenceLow, ConfidenceMedium, ConfidenceHigh:
	default:
		return fmt.Errorf("%w: unknown confidence %q", ErrInvalidHypothesis, h.Confidence)
	}
	if strings.TrimSpace(h.Summary) == "" {
		return fmt.Errorf("%w: missing summary", ErrInvalidHypothesis)
	}
	if len(h.Evidence) == 0 {
		return fmt.Errorf("%w: %q has no evidence", ErrInvalidHypothesis, h.Type)
	}
	if strings.TrimSpace(h.Caveat) == "" {
		return fmt.Errorf("%w: missing caveat", ErrInvalidHypothesis)
	}
	for _, rejected := range h.Rejected {
		if rejected.Type == h.Type {
			return fmt.Errorf("%w: rejects its own type %q", ErrInvalidHypothesis, h.Type)
		}
	}
	return nil
}

func (t BlockageType) valid() bool {
	switch t {
	case BlockDNS, BlockTCPRST, BlockIPDrop, BlockPortBlock, BlockSNIFilter, BlockHTTP, BlockThrottling, BlockUnknown:
		return true
	default:
		return false
	}
}

// ClassifyTarget ranks one hypothesis from the ladder and path evidence.
func ClassifyTarget(steps []StepResult, path Localization) Hypothesis {
	summary := newEvidenceSummary(steps)

	switch {
	case summary.tlsDiverged():
		evidence := []string{
			"the real SNI failed while the decoy SNI completed the handshake",
			describeStep(summary.tlsReal),
		}
		if summary.tlsAbsent != nil && summary.tlsAbsent.Status == StatusPass {
			evidence = append(evidence, "the same host with no SNI completed the handshake")
		}
		confidence := ConfidenceMedium
		if summary.tlsReal.Status == StatusFail {
			confidence = ConfidenceHigh
		}
		return newHypothesis(BlockSNIFilter, confidence,
			"TLS fails with the real server name but succeeds with a decoy", evidence, summary)

	case summary.dnsDiffer || summary.dnsFailedWhileDoHAnswered():
		evidence := []string{describeStep(summary.dns)}
		if summary.dnsDiffer {
			evidence = append(evidence, "the system resolver and the DoH answers differ")
		} else {
			evidence = append(evidence, "the system resolver failed while DoH answered")
		}
		return newHypothesis(BlockDNS, ConfidenceMedium,
			"name resolution looks altered", evidence, summary)

	case summary.reset:
		return newHypothesis(BlockTCPRST, ConfidenceMedium,
			"a TCP reset was observed", []string{"a TCP connect was reset by the peer"}, summary)

	case summary.connected > 0 && summary.timedOut > 0:
		return newHypothesis(BlockPortBlock, ConfidenceMedium,
			"some ports connect while others time out",
			[]string{fmt.Sprintf("%d port(s) connected, %d timed out", summary.connected, summary.timedOut)}, summary)

	case summary.httpFailed():
		return newHypothesis(BlockHTTP, ConfidenceLow,
			"the HTTP step failed while transport worked", []string{describeStep(summary.http)}, summary)

	case summary.connected == 0 && summary.timedOut > 0:
		confidence := ConfidenceLow
		evidence := []string{"every TCP probe timed out"}
		if path.Kind == PathDrop {
			confidence = ConfidenceMedium
			evidence = append(evidence, path.Summary)
		}
		if summary.icmp == nil || summary.icmp.Status != StatusPass {
			evidence = append(evidence, "ICMP did not answer either")
		}
		return newHypothesis(BlockIPDrop, confidence,
			"the path does not answer above the last responding hop", evidence, summary)

	case summary.allPassed():
		return newHypothesis(BlockUnknown, ConfidenceLow,
			"no blockage detected: every ladder step passed", []string{"all ladder steps passed"}, summary)
	}

	return newHypothesis(BlockUnknown, ConfidenceLow,
		"insufficient evidence for a specific blockage type", summary.anomalies(), summary)
}

// evidenceSummary extracts the facts the rules rely on.
type evidenceSummary struct {
	steps       []StepResult
	dns         *StepResult
	tlsReal     *StepResult
	tlsAbsent   *StepResult
	tlsDecoy    *StepResult
	http        *StepResult
	icmp        *StepResult
	dnsDiffer   bool
	dnsFailed   bool
	dohAnswered bool
	reset       bool
	connected   int
	timedOut    int
}

func newEvidenceSummary(steps []StepResult) evidenceSummary {
	summary := evidenceSummary{steps: steps}
	for i := range steps {
		step := &steps[i]
		switch {
		case step.ID == "dns":
			summary.dns = step
			if stepMeasurement(*step, "comparison") == "differ" {
				summary.dnsDiffer = true
			}
			if step.Status == StatusFail {
				summary.dnsFailed = true
			}
			if answers := stepMeasurement(*step, "doh.answers"); answers != "" && answers != "none" {
				summary.dohAnswered = true
			}
		case step.ID == "tls.real":
			summary.tlsReal = step
		case step.ID == "tls.decoy":
			summary.tlsDecoy = step
		case step.ID == "tls.absent":
			summary.tlsAbsent = step
		case step.ID == "http":
			summary.http = step
		case step.ID == "icmp":
			summary.icmp = step
		case step.Kind == StepTCP:
			switch stepMeasurement(*step, "outcome") {
			case "connected":
				summary.connected++
			case "timeout":
				summary.timedOut++
			case "reset":
				summary.reset = true
			}
		}
	}
	return summary
}

func (s evidenceSummary) tlsDiverged() bool {
	if s.tlsReal == nil || s.tlsDecoy == nil {
		return false
	}
	return s.tlsReal.Status != StatusPass && s.tlsDecoy.Status == StatusPass
}

func (s evidenceSummary) dnsFailedWhileDoHAnswered() bool {
	return s.dnsFailed && s.dohAnswered
}

func (s evidenceSummary) httpFailed() bool {
	if s.http == nil {
		return false
	}
	if s.http.Status == StatusFail {
		return true
	}
	status := stepMeasurement(*s.http, "status")
	return strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5")
}

func (s evidenceSummary) allPassed() bool {
	if len(s.steps) == 0 {
		return false
	}
	for _, step := range s.steps {
		if step.Status == StatusFail || step.Status == StatusWarn {
			return false
		}
	}
	return true
}

func (s evidenceSummary) anomalies() []string {
	values := make([]string, 0, 4)
	for _, step := range s.steps {
		if step.Status == StatusFail || step.Status == StatusWarn {
			values = append(values, describeStep(&step))
		}
	}
	if len(values) == 0 {
		values = append(values, "no ladder step produced a usable signal")
	}
	return values
}

func stepMeasurement(step StepResult, name string) string {
	for _, measurement := range step.Measurements {
		if measurement.Name == name {
			return measurement.Value
		}
	}
	return ""
}

func describeStep(step *StepResult) string {
	if step == nil {
		return "the step was not run"
	}
	text := fmt.Sprintf("%s %s", step.ID, step.Status)
	if step.Diagnostic != "" {
		text += ": " + step.Diagnostic
	}
	if step.Error != "" {
		text += " (" + step.Error + ")"
	}
	return text
}

// newHypothesis builds a hypothesis with the standard caveat and rejections.
func newHypothesis(kind BlockageType, confidence Confidence, summary string, evidence []string, facts evidenceSummary) Hypothesis {
	return Hypothesis{
		Type:       kind,
		Confidence: confidence,
		Summary:    summary,
		Evidence:   evidence,
		Rejected:   rejections(kind, facts),
		Caveat:     SingleVantageCaveat,
	}
}

// rejections lists every other taxonomy member with the reason it was dismissed.
func rejections(chosen BlockageType, facts evidenceSummary) []RejectedAlternative {
	reasons := map[BlockageType]string{
		BlockDNS:        "no divergence between the system resolver and DoH",
		BlockTCPRST:     "no TCP reset observed",
		BlockIPDrop:     "the target answered at least one probe",
		BlockPortBlock:  "the tested ports behaved the same",
		BlockSNIFilter:  "the real and decoy SNI behaved the same",
		BlockHTTP:       "the HTTP step completed",
		BlockThrottling: "no latency evidence above the threshold",
		BlockUnknown:    "a specific type matched the evidence",
	}
	if facts.dnsDiffer || facts.dnsFailedWhileDoHAnswered() {
		reasons[BlockDNS] = "the DNS divergence was considered, but another type explained the evidence better"
	}
	if facts.reset {
		reasons[BlockTCPRST] = "a reset was observed, but the remaining evidence weighted another type higher"
	}
	if facts.connected > 0 {
		reasons[BlockIPDrop] = "the target answered at least one probe"
	}
	if facts.connected > 0 && facts.timedOut > 0 {
		reasons[BlockPortBlock] = "the port divergence was considered, but another type matched better"
	}
	if facts.tlsDiverged() {
		reasons[BlockSNIFilter] = "the SNI divergence was considered, but another type matched better"
	}
	if facts.http != nil && facts.http.Status == StatusPass {
		reasons[BlockHTTP] = "the HTTP step completed"
	}

	order := []BlockageType{BlockDNS, BlockTCPRST, BlockIPDrop, BlockPortBlock, BlockSNIFilter, BlockHTTP, BlockThrottling, BlockUnknown}
	rejected := make([]RejectedAlternative, 0, len(order)-1)
	for _, kind := range order {
		if kind == chosen {
			continue
		}
		rejected = append(rejected, RejectedAlternative{Type: kind, Reason: reasons[kind]})
	}
	return rejected
}
