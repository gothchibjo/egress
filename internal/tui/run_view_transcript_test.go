package tui

import (
	"fmt"
	"os"
	"testing"
)

// TestRunViewTranscript prints the run view at the reviewed sizes. It is
// skipped by default and regenerated with EGRESS_TRANSCRIPT=1.
func TestRunViewTranscript(t *testing.T) {
	if os.Getenv("EGRESS_TRANSCRIPT") == "" {
		t.Skip("set EGRESS_TRANSCRIPT=1 to print the run view transcript")
	}

	wide := completeRun(t, runModel(t, 120, 40))
	fmt.Printf("\n=== run view, wide (120x40) ===\n%s\n", wide.View())

	compact := completeRun(t, runModel(t, 60, 20))
	fmt.Printf("\n=== run view, compact ladder (60x20) ===\n%s\n", compact.View())

	compact, _ = pressKey(t, compact, "2")
	fmt.Printf("\n=== run view, compact hops (60x20) ===\n%s\n", compact.View())

	verdict := completeRun(t, runModel(t, 100, 30))
	verdict, _ = pressKey(t, verdict, "3")
	fmt.Printf("\n=== run view, verdict (100x30) ===\n%s\n", verdict.View())

	small := completeRun(t, runModel(t, 50, 15))
	fmt.Printf("\n=== run view, below the minimum (50x15) ===\n%s\n", small.View())
}
