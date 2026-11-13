package domain

import (
	"strings"
	"testing"
)

func respondingHop(ttl int, address string) Hop {
	return Hop{TTL: ttl, Address: address, Sent: 3, Received: 3, RTTMinMS: 10, RTTMedianMS: 12, RTTMaxMS: 15}
}

func silentHop(ttl int) Hop {
	return Hop{TTL: ttl, Sent: 3, Received: 0}
}

func TestLocalizePathComplete(t *testing.T) {
	hops := []Hop{respondingHop(1, "192.0.2.1"), respondingHop(2, "198.51.100.1"), respondingHop(3, "1.1.1.1")}
	report := BuildPathReport(hops, PathOptions{TargetIP: "1.1.1.1"})

	if report.Localization.Kind != PathComplete {
		t.Fatalf("kind = %q", report.Localization.Kind)
	}
	if !strings.Contains(report.Localization.Summary, "hop 3") {
		t.Fatalf("summary = %q", report.Localization.Summary)
	}
	if len(report.Hops) != len(hops) {
		t.Fatalf("report lost hops: %d", len(report.Hops))
	}
}

func TestLocalizePathDrop(t *testing.T) {
	hops := []Hop{respondingHop(1, "192.0.2.1"), respondingHop(2, "198.51.100.1"), silentHop(3), silentHop(4)}
	localization := LocalizePath(hops, PathOptions{TargetIP: "1.1.1.1"})

	if localization.Kind != PathDrop || localization.AfterTTL != 2 {
		t.Fatalf("localization = %+v", localization)
	}
	if !strings.Contains(localization.Summary, "drop after hop 2") {
		t.Fatalf("summary = %q", localization.Summary)
	}
	if len(localization.Alternatives) == 0 {
		t.Fatal("drop carries no alternatives")
	}
}

func TestLocalizePathTruncatedIsInconclusive(t *testing.T) {
	hops := []Hop{respondingHop(1, "192.0.2.1"), respondingHop(2, "198.51.100.1"), silentHop(3), silentHop(4)}
	localization := LocalizePath(hops, PathOptions{TargetIP: "1.1.1.1", Truncated: true})

	if localization.Kind != PathInconclusive {
		t.Fatalf("kind = %q", localization.Kind)
	}
	if localization.FromTTL != 3 || localization.ToTTL != 4 {
		t.Fatalf("range = %d..%d", localization.FromTTL, localization.ToTTL)
	}
	if !strings.Contains(localization.Summary, "truncated") {
		t.Fatalf("summary = %q", localization.Summary)
	}
}

func TestLocalizePathLadderReachableIsInconclusive(t *testing.T) {
	hops := []Hop{respondingHop(1, "192.0.2.1"), respondingHop(2, "198.51.100.1"), silentHop(3), silentHop(4)}
	localization := LocalizePath(hops, PathOptions{TargetIP: "1.1.1.1", LadderReachable: true})

	if localization.Kind != PathInconclusive {
		t.Fatalf("kind = %q", localization.Kind)
	}
	if !strings.Contains(localization.Summary, "answered the ladder") {
		t.Fatalf("summary = %q", localization.Summary)
	}
}

func TestLocalizePathNoReplies(t *testing.T) {
	localization := LocalizePath([]Hop{silentHop(1), silentHop(2)}, PathOptions{TargetIP: "1.1.1.1"})

	if localization.Kind != PathInconclusive {
		t.Fatalf("kind = %q", localization.Kind)
	}
	if !strings.Contains(localization.Summary, "no hop answered") {
		t.Fatalf("summary = %q", localization.Summary)
	}
}

func TestLocalizePathStarInTheMiddleDoesNotLookLikeADrop(t *testing.T) {
	hops := []Hop{
		respondingHop(1, "192.0.2.1"),
		silentHop(2),
		respondingHop(3, "198.51.100.1"),
		silentHop(4),
	}
	localization := LocalizePath(hops, PathOptions{TargetIP: "1.1.1.1"})

	if localization.Kind != PathDrop || localization.AfterTTL != 3 {
		t.Fatalf("localization = %+v", localization)
	}
}

func TestLocalizePathWithoutData(t *testing.T) {
	localization := LocalizePath(nil, PathOptions{})
	if localization.Kind != PathUnknown {
		t.Fatalf("kind = %q", localization.Kind)
	}
}

func TestHopLossPercent(t *testing.T) {
	hop := Hop{Sent: 3, Received: 1}
	if got := hop.LossPercent(); got != 66 {
		t.Fatalf("loss = %d, want 66", got)
	}
	if got := (Hop{}).LossPercent(); got != 0 {
		t.Fatalf("empty hop loss = %d", got)
	}
}
