package domain

import "strings"

// BuildTargetReport derives the path report and the hypothesis for one target
// from its ladder steps. The TUI and the export share it.
func BuildTargetReport(steps []StepResult) (PathReport, Hypothesis) {
	hops := make([]Hop, 0, 30)
	targetIP := ""
	reachable := false
	truncated := false

	for _, step := range steps {
		switch step.Kind {
		case StepTrace:
			hops = append(hops, step.Hops...)
			if strings.Contains(step.Diagnostic, "truncated") {
				truncated = true
			}
		case StepDNS:
			if targetIP == "" {
				targetIP = firstAnswer(stepMeasurement(step, "system.answers"))
			}
		case StepICMP:
			if targetIP == "" {
				targetIP = stepMeasurement(step, "address")
			}
			reachable = reachable || step.Status == StatusPass
		case StepTCP, StepTLS, StepHTTP:
			reachable = reachable || step.Status == StatusPass
		}
	}

	path := BuildPathReport(hops, PathOptions{
		TargetIP:        targetIP,
		Truncated:       truncated,
		LadderReachable: reachable,
	})
	return path, ClassifyTarget(steps, path.Localization)
}

// firstAnswer returns the first address of a joined DNS answer list.
func firstAnswer(value string) string {
	if index := strings.Index(value, ","); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}
