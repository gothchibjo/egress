package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// VerdictLevel is the aggregate health of one completed baseline run.
type VerdictLevel string

const (
	VerdictHealthy  VerdictLevel = "healthy"
	VerdictDegraded VerdictLevel = "degraded"
	VerdictBlocked  VerdictLevel = "blocked"
)

// Counts is the number of results per status in one run.
type Counts struct {
	Pass    int `json:"pass"`
	Warn    int `json:"warn"`
	Fail    int `json:"fail"`
	Skipped int `json:"skipped"`
}

// Verdict is the aggregate outcome of one baseline run. Finding points at the
// most actionable result; it is nil when no result needs attention.
type Verdict struct {
	Level   VerdictLevel `json:"verdict"`
	Counts  Counts       `json:"counts"`
	Finding *Result      `json:"finding,omitempty"`
}

// Aggregation errors.
var (
	ErrNoResults           = errors.New("no baseline results to aggregate")
	ErrInvalidVerdictInput = errors.New("baseline result is not valid evidence")
)

// AggregateVerdict derives one verdict from a completed run using the rule in
// docs/verdict-rule.md. It rejects an empty run and any result that cannot
// explain its own status, so a verdict always rests on evidence.
func AggregateVerdict(results []Result) (Verdict, error) {
	if len(results) == 0 {
		return Verdict{}, ErrNoResults
	}
	for i, result := range results {
		if err := result.Validate(); err != nil {
			return Verdict{}, fmt.Errorf("%w: result %d (%s): %w", ErrInvalidVerdictInput, i, result.CheckID, err)
		}
	}

	counts := Counts{}
	anchorIDs := make(map[string]bool, len(BaselineAnchors()))
	anchorResults, anchorFails := 0, 0
	for _, result := range results {
		switch result.Status {
		case StatusPass:
			counts.Pass++
		case StatusWarn:
			counts.Warn++
		case StatusFail:
			counts.Fail++
		case StatusSkipped:
			counts.Skipped++
		}
		if anchor, ok := AnchorForTarget(result.Target); ok {
			anchorResults++
			anchorIDs[anchor.ID] = true
			if result.Status == StatusFail {
				anchorFails++
			}
		}
	}

	// An isolated anchor failure is downgraded to a warning. Only a path-wide
	// destination failure, with at least two distinct anchors checked and all of
	// them failing, keeps anchor failures at full severity.
	anchorFailureBlocked := anchorFails > 0 && anchorFails == anchorResults && len(anchorIDs) >= 2

	blocked := false
	downgradedFails := 0
	for _, result := range results {
		switch result.Status {
		case StatusFail:
			if _, isAnchor := AnchorForTarget(result.Target); isAnchor && !anchorFailureBlocked {
				downgradedFails++
				continue
			}
			blocked = true
		}
	}

	verdict := Verdict{Counts: counts}
	switch {
	case blocked:
		verdict.Level = VerdictBlocked
	case downgradedFails > 0 || counts.Warn > 0:
		verdict.Level = VerdictDegraded
	case counts.Pass > 0:
		verdict.Level = VerdictHealthy
	default:
		// Every check was skipped: the run cannot establish health.
		verdict.Level = VerdictDegraded
	}

	verdict.Finding = mostActionable(results, anchorFailureBlocked, verdict.Level)
	return verdict, nil
}

// mostActionable returns the first result in baseline order at the highest
// effective severity. Ties keep the earlier result, so identical input always
// yields the same finding.
func mostActionable(results []Result, anchorFailureBlocked bool, level VerdictLevel) *Result {
	if finding := firstMatch(results, func(result Result) bool {
		if result.Status != StatusFail {
			return false
		}
		_, isAnchor := AnchorForTarget(result.Target)
		return !isAnchor || anchorFailureBlocked
	}); finding != nil {
		return finding
	}
	if finding := firstMatch(results, func(result Result) bool {
		if result.Status != StatusFail {
			return false
		}
		_, isAnchor := AnchorForTarget(result.Target)
		return isAnchor && !anchorFailureBlocked
	}); finding != nil {
		return finding
	}
	if finding := firstMatch(results, func(result Result) bool {
		return result.Status == StatusWarn
	}); finding != nil {
		return finding
	}
	if level != VerdictDegraded {
		return nil
	}
	return firstMatch(results, func(result Result) bool {
		return result.Status == StatusSkipped
	})
}

func firstMatch(results []Result, match func(Result) bool) *Result {
	for _, result := range results {
		if match(result) {
			finding := result
			return &finding
		}
	}
	return nil
}

// AnchorForTarget returns the curated anchor a result target refers to. Scheme,
// port, path, user info and a trailing dot are ignored; only the host is
// compared, case-insensitively.
func AnchorForTarget(target string) (Anchor, bool) {
	host := targetHost(target)
	if host == "" {
		return Anchor{}, false
	}
	for _, anchor := range BaselineAnchors() {
		if strings.EqualFold(anchor.Host, host) {
			return anchor, true
		}
	}
	return Anchor{}, false
}

func targetHost(target string) string {
	host := strings.TrimSpace(target)
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.LastIndex(host, ":"); i > 0 && strings.Count(host, ":") == 1 {
		if _, err := strconv.Atoi(host[i+1:]); err == nil {
			host = host[:i]
		}
	}
	return strings.ToLower(strings.TrimSuffix(host, "."))
}
