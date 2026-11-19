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

// ObservationMode labels an evidence line as an active rejection or passive
// silence.
type ObservationMode string

const (
	ObservedActive  ObservationMode = "active"
	ObservedPassive ObservationMode = "passive"
)

// EvidenceLine is one labelled observation behind a hypothesis.
type EvidenceLine struct {
	Mode ObservationMode `json:"mode"`
	Text string          `json:"text"`
}

func active(text string) EvidenceLine  { return EvidenceLine{Mode: ObservedActive, Text: text} }
func passive(text string) EvidenceLine { return EvidenceLine{Mode: ObservedPassive, Text: text} }

// ErrInvalidHypothesis marks a hypothesis that cannot explain itself.
var ErrInvalidHypothesis = errors.New("invalid hypothesis")

// RejectedAlternative is a taxonomy member that was considered and dismissed.
type RejectedAlternative struct {
	Type   BlockageType `json:"type"`
	Reason string       `json:"reason"`
}

// Hypothesis is the ranked blockage finding for one target. NoBlockage marks a
// target whose ladder passed; the taxonomy type stays unknown because it only
// describes blockage types.
type Hypothesis struct {
	Type       BlockageType          `json:"type"`
	Confidence Confidence            `json:"confidence"`
	NoBlockage bool                  `json:"no_blockage,omitempty"`
	Summary    string                `json:"summary"`
	Evidence   []EvidenceLine        `json:"evidence"`
	Rejected   []RejectedAlternative `json:"rejected"`
	Caveat     string                `json:"caveat"`
}

// SingleVantageCaveat is the fixed opening of every hypothesis caveat.
const SingleVantageCaveat = "Single vantage point: one observation from one network; absence of evidence is not proof of blocking."

// caveatFor builds the full disclosure: the vantage limitation, the
// false-positive risks of the probes that actually ran, the absence-of-evidence
// rule and the external-validation suggestion.
func caveatFor(steps []StepResult) string {
	risks := make([]string, 0, 5)
	seen := make(map[string]bool, 5)
	add := func(risk string) {
		if !seen[risk] {
			seen[risk] = true
			risks = append(risks, risk)
		}
	}
	for _, step := range steps {
		switch {
		case step.Kind == StepDNS:
			add("DNS answers can differ by CDN or geo-DNS")
		case step.Kind == StepTCP:
			add("TCP resets can come from middleboxes")
		case step.Kind == StepTLS:
			add("TLS alerts can reflect policy or certificates")
		case step.Kind == StepICMP || step.Kind == StepTrace:
			add("ICMP replies are often rate-limited or filtered")
		case step.Kind == StepHTTP:
			add("HTTP errors can be application policy")
		}
	}
	if len(risks) == 0 {
		risks = append(risks, "probe replies can be rate-limited or filtered")
	}
	return "Single vantage point: one observation from one network. False-positive risks: " +
		strings.Join(risks, "; ") +
		". Absence of evidence is not proof of blocking. Re-run from another network to validate."
}

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
	for _, line := range h.Evidence {
		if line.Mode != ObservedActive && line.Mode != ObservedPassive {
			return fmt.Errorf("%w: evidence line %q has mode %q", ErrInvalidHypothesis, line.Text, line.Mode)
		}
		if strings.TrimSpace(line.Text) == "" {
			return fmt.Errorf("%w: evidence line has no text", ErrInvalidHypothesis)
		}
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
		evidence := []EvidenceLine{
			stepObservation(summary.tlsReal),
			active("the decoy SNI completed the handshake"),
		}
		if summary.tlsAbsent != nil && summary.tlsAbsent.Status == StatusPass {
			evidence = append(evidence, active("the same host with no SNI completed the handshake"))
		}
		// The decoy handshake is an active observation, so the two signals
		// together justify high confidence.
		return newHypothesis(BlockSNIFilter, ConfidenceHigh,
			"TLS fails with the real server name but succeeds with a decoy", evidence, summary)

	case summary.dnsDiffer || summary.dnsFailedWhileDoHAnswered():
		evidence := []EvidenceLine{stepObservation(summary.dns)}
		if summary.dnsDiffer {
			if system := stepMeasurement(*summary.dns, "system.answers"); system != "" {
				evidence = append(evidence, active("system answers: "+system))
			}
			if doh := stepMeasurement(*summary.dns, "doh.answers"); doh != "" {
				evidence = append(evidence, active("DoH answers: "+doh))
			}
			evidence = append(evidence,
				active("the system resolver and the DoH answers differ"),
				active("CDN and geo-DNS answers can differ legitimately"),
			)
		} else {
			evidence = append(evidence,
				passive("the system resolver did not answer"),
				active("DoH answered the same query"),
			)
		}
		return newHypothesis(BlockDNS, ConfidenceMedium,
			"name resolution looks altered", evidence, summary)

	case summary.reset:
		return newHypothesis(BlockTCPRST, ConfidenceMedium,
			"a TCP reset was observed",
			[]EvidenceLine{active("a TCP connect was reset by the peer")}, summary)

	case summary.connected > 0 && summary.timedOut > 0:
		return newHypothesis(BlockPortBlock, ConfidenceMedium,
			"some ports connect while others time out",
			[]EvidenceLine{
				active(fmt.Sprintf("%d port(s) connected", summary.connected)),
				passive(fmt.Sprintf("%d port(s) timed out", summary.timedOut)),
			}, summary)

	case summary.httpFailed():
		return newHypothesis(BlockHTTP, ConfidenceLow,
			"the HTTP step failed while transport worked",
			[]EvidenceLine{stepObservation(summary.http)}, summary)

	case summary.connected == 0 && summary.timedOut > 0:
		confidence := ConfidenceLow
		evidence := []EvidenceLine{passive("every TCP probe timed out")}
		if path.Kind == PathDrop {
			confidence = ConfidenceMedium
			evidence = append(evidence, passive(path.Summary))
		}
		if summary.icmp == nil || summary.icmp.Status != StatusPass {
			evidence = append(evidence, passive("no ICMP reply; ICMP may be rate-limited or blocked"))
		}
		return newHypothesis(BlockIPDrop, confidence,
			"the path does not answer above the last responding hop", evidence, summary)

	case summary.allPassed():
		hypothesis := newHypothesis(BlockUnknown, ConfidenceLow,
			"every ladder step passed",
			[]EvidenceLine{active("all ladder steps passed")}, summary)
		hypothesis.NoBlockage = true
		return hypothesis
	}

	anomalies := make([]EvidenceLine, 0, len(steps))
	for i := range steps {
		if steps[i].Status == StatusFail || steps[i].Status == StatusWarn {
			anomalies = append(anomalies, stepObservation(&steps[i]))
		}
	}
	if len(anomalies) == 0 {
		anomalies = append(anomalies, passive("no ladder step produced a usable signal"))
	}
	return newHypothesis(BlockUnknown, ConfidenceLow,
		"insufficient evidence for a specific blockage type", anomalies, summary)
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

// observationMode labels a step as active rejection or passive silence.
func observationMode(step *StepResult) ObservationMode {
	if step == nil {
		return ObservedPassive
	}
	outcome := stepMeasurement(*step, "outcome")
	switch outcome {
	case "timeout", "no_reply":
		return ObservedPassive
	}
	if step.Kind == StepICMP && step.Status != StatusPass {
		return ObservedPassive
	}
	if step.Kind == StepHTTP && step.Status != StatusPass && stepMeasurement(*step, "status") == "" {
		return ObservedPassive
	}
	return ObservedActive
}

func stepObservation(step *StepResult) EvidenceLine {
	return EvidenceLine{Mode: observationMode(step), Text: describeStep(step)}
}

// newHypothesis builds a hypothesis with the standard caveat and rejections. A
// high confidence needs at least one active observation.
func newHypothesis(kind BlockageType, confidence Confidence, summary string, evidence []EvidenceLine, facts evidenceSummary) Hypothesis {
	if confidence == ConfidenceHigh {
		hasActive := false
		for _, line := range evidence {
			if line.Mode == ObservedActive {
				hasActive = true
				break
			}
		}
		if !hasActive {
			confidence = ConfidenceMedium
		}
	}
	return Hypothesis{
		Type:       kind,
		Confidence: confidence,
		Summary:    summary,
		Evidence:   evidence,
		Rejected:   rejections(kind, facts),
		Caveat:     caveatFor(facts.steps),
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
