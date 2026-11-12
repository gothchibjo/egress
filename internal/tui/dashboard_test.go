package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/gothchibjo/egress/internal/domain"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

func testResult(id domain.CheckID, target string, status domain.Status) domain.Result {
	result := domain.Result{
		CheckID:   id,
		Status:    status,
		Target:    target,
		Method:    "test",
		Timestamp: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	switch status {
	case domain.StatusPass:
		result.Measurements = []domain.Measurement{{Name: "reachable", Value: "true"}}
	case domain.StatusWarn, domain.StatusFail:
		result.Measurements = []domain.Measurement{{Name: "reachable", Value: "false"}}
	case domain.StatusSkipped:
		result.Diagnostic = "capability unavailable in this mode"
	}
	return result
}

func fixtureResults() []domain.Result {
	return []domain.Result{
		testResult("local.interfaces", "en0", domain.StatusPass),
		testResult("gateway.reach", "192.168.1.1", domain.StatusWarn),
		testResult("dns.canary", "example.com:53", domain.StatusPass),
		testResult("icmp.anchor", "1.1.1.1", domain.StatusFail),
		testResult("http.canary", "captive.apple.com", domain.StatusSkipped),
		testResult("custom.check", "localhost", domain.StatusPass),
	}
}

func fixtureModel(t *testing.T, width, height int, opts ...Option) Model {
	t.Helper()
	results := fixtureResults()
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	return New(results, verdict, opts...).Resize(width, height)
}

func assertBlockShape(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != height {
		t.Fatalf("view has %d lines, want %d", len(lines), height)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d is %d wide, max %d: %q", i, got, width, line)
		}
	}
}

func TestLayoutThresholds(t *testing.T) {
	tests := []struct {
		width, height int
		want          Layout
	}{
		{width: 120, height: 40, want: LayoutNormal},
		{width: 80, height: 24, want: LayoutNormal},
		{width: 79, height: 24, want: LayoutCompact},
		{width: 80, height: 23, want: LayoutCompact},
		{width: 60, height: 20, want: LayoutCompact},
		{width: 59, height: 20, want: LayoutTooSmall},
		{width: 60, height: 19, want: LayoutTooSmall},
		{width: 40, height: 10, want: LayoutTooSmall},
		{width: 39, height: 24, want: LayoutStopped},
		{width: 80, height: 9, want: LayoutStopped},
		{width: 0, height: 0, want: LayoutStopped},
	}
	for _, test := range tests {
		model := fixtureModel(t, test.width, test.height, WithoutColor())
		if got := model.Layout(); got != test.want {
			t.Errorf("Layout() at %dx%d = %v, want %v", test.width, test.height, got, test.want)
		}
	}
}

func TestNormalLayoutShowsBothPanes(t *testing.T) {
	model := fixtureModel(t, 80, 24, WithoutColor())
	view := model.View()
	assertBlockShape(t, view, 80, 24)

	if !strings.Contains(view, "EVIDENCE") {
		t.Fatal("evidence pane is missing")
	}
	if !strings.Contains(view, "dns.canary") {
		t.Fatal("selected check id is missing")
	}
	for _, id := range []string{"local.interfaces", "gateway.reach", "icmp.anchor", "http.canary"} {
		if !strings.Contains(view, id) {
			t.Fatalf("check %q is missing from the list", id)
		}
	}
	found := false
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "EVIDENCE") && strings.Contains(line, "LOCAL LINK") {
			found = true
		}
	}
	if !found {
		t.Fatal("list and evidence are not side by side")
	}
	if !strings.Contains(view, "DEGRADED") {
		t.Fatal("verdict is missing")
	}
	if !strings.Contains(view, "pass 3") || !strings.Contains(view, "skip 1") {
		t.Fatal("status counts are missing")
	}
}

func TestCompactLayoutStacksPanes(t *testing.T) {
	model := fixtureModel(t, 70, 22, WithoutColor())
	view := model.View()
	assertBlockShape(t, view, 70, 22)

	lines := strings.Split(view, "\n")
	evidence := -1
	listRow := -1
	for i, line := range lines {
		if evidence < 0 && strings.Contains(line, "EVIDENCE") {
			evidence = i
		}
		if listRow < 0 && strings.Contains(line, "local.interfaces") {
			listRow = i
		}
	}
	if evidence < 0 || listRow < 0 {
		t.Fatal("stacked view is missing the list or the evidence pane")
	}
	if evidence <= listRow {
		t.Fatal("evidence pane is not stacked below the list")
	}
	if strings.Contains(view, "│") {
		t.Fatal("compact layout still uses a vertical pane separator")
	}
}

func TestCompactShowsSkippedReason(t *testing.T) {
	model := fixtureModel(t, 60, 20, WithoutColor()).Select(4)
	view := model.View()
	assertBlockShape(t, view, 60, 20)
	if !strings.Contains(view, "capability unavailable in this mode") {
		t.Fatal("skipped reason is missing from the compact evidence pane")
	}
}

func TestTooSmallMessage(t *testing.T) {
	model := fixtureModel(t, 50, 16, WithoutColor())
	view := model.View()
	assertBlockShape(t, view, 50, 16)
	if !strings.Contains(view, "too small") {
		t.Fatalf("resize message is missing: %q", view)
	}
	if !strings.Contains(view, "60x20") {
		t.Fatal("resize message does not name the minimum size")
	}
}

func TestRenderingStopsBelowMinimum(t *testing.T) {
	for _, size := range [][2]int{{39, 24}, {80, 9}, {20, 5}} {
		model := fixtureModel(t, size[0], size[1], WithoutColor())
		if view := model.View(); view != "" {
			t.Fatalf("View() at %dx%d = %q, want empty", size[0], size[1], view)
		}
	}
}

func TestResizePreservesSelection(t *testing.T) {
	model := fixtureModel(t, 80, 24, WithoutColor()).Select(3)
	selected, ok := model.SelectedResult()
	if !ok || selected.CheckID != "icmp.anchor" {
		t.Fatalf("selected result = %q, want icmp.anchor", selected.CheckID)
	}

	for _, size := range [][2]int{{60, 20}, {100, 40}, {70, 22}, {80, 24}} {
		model = model.Resize(size[0], size[1])
		if model.Selected() != 3 {
			t.Fatalf("selection = %d after resize to %dx%d, want 3", model.Selected(), size[0], size[1])
		}
		if selected, _ := model.SelectedResult(); selected.CheckID != "icmp.anchor" {
			t.Fatalf("selected result = %q after resize, want icmp.anchor", selected.CheckID)
		}
	}
}

func TestSelectionIsClamped(t *testing.T) {
	model := fixtureModel(t, 80, 24, WithoutColor())
	if model.Select(-5).Selected() != 0 {
		t.Fatal("negative selection was not clamped to zero")
	}
	if last := model.Select(100).Selected(); last != len(fixtureResults())-1 {
		t.Fatalf("selection = %d, want %d", last, len(fixtureResults())-1)
	}

	empty := New(nil, domain.Verdict{}, WithoutColor()).Select(10)
	if empty.Selected() != 0 {
		t.Fatalf("empty selection = %d, want 0", empty.Selected())
	}
	if _, ok := empty.SelectedResult(); ok {
		t.Fatal("empty dashboard reports a selected result")
	}
}

func TestStatusIsReadableWithoutColor(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	view := model.View()
	for _, marker := range []string{"✓", "!", "✗", "·"} {
		if !strings.Contains(view, marker) {
			t.Fatalf("status marker %q is missing", marker)
		}
	}
	if !strings.Contains(view, "✓ pass") {
		t.Fatal("detail pane does not pair a marker with a status word")
	}
	failing := model.Select(3).View()
	if !strings.Contains(failing, "✗ fail") {
		t.Fatal("failing detail does not pair a marker with a status word")
	}
	if strings.Contains(view, "\x1b") {
		t.Fatal("colorless view contains ANSI escapes")
	}
}

func TestNOColorIsHonored(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	results := fixtureResults()
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	model := New(results, verdict).Resize(100, 30)
	if view := model.View(); strings.Contains(view, "\x1b") {
		t.Fatal("NO_COLOR view contains ANSI escapes")
	}
}

func TestColorProfiles(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)

	plain := ""
	for _, profile := range []termenv.Profile{termenv.ANSI, termenv.ANSI256} {
		lipgloss.SetColorProfile(profile)
		view := fixtureModel(t, 100, 30, WithColor()).View()
		if !strings.Contains(view, "\x1b[") {
			t.Fatalf("profile %v rendered without ANSI styling", profile)
		}
		stripped := ansiPattern.ReplaceAllString(view, "")
		if !strings.Contains(stripped, "EVIDENCE") || !strings.Contains(stripped, "DEGRADED") {
			t.Fatalf("profile %v lost dashboard content", profile)
		}
		if plain == "" {
			plain = stripped
			continue
		}
		if stripped != plain {
			t.Fatalf("profile %v changed the visible layout", profile)
		}
	}
}

func TestASCIIFallback(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor(), WithASCII())
	view := model.View()
	assertBlockShape(t, view, 100, 30)

	for _, marker := range []string{"[ok]", "[!!]", "[xx]", "[--]"} {
		if !strings.Contains(view, marker) {
			t.Fatalf("ASCII marker %q is missing", marker)
		}
	}
	for _, unicode := range []string{"✓", "✗", "·", "│", "─", "—"} {
		if strings.Contains(view, unicode) {
			t.Fatalf("ASCII view contains %q", unicode)
		}
	}
	if !strings.Contains(view, "egress - DEGRADED") {
		t.Fatal("ASCII header is not rendered")
	}
}

func TestHeaderShowsElapsed(t *testing.T) {
	results := fixtureResults()
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	model := New(results, verdict, WithoutColor(), WithElapsed(1500*time.Millisecond)).Resize(100, 30)
	if view := model.View(); !strings.Contains(view, "1.5s") {
		t.Fatal("elapsed time is missing from the header")
	}
}

func TestEmptyDashboard(t *testing.T) {
	model := New(nil, domain.Verdict{}, WithoutColor()).Resize(80, 24)
	view := model.View()
	assertBlockShape(t, view, 80, 24)
	if !strings.Contains(view, "NO VERDICT") {
		t.Fatal("empty dashboard claims a verdict")
	}
	if !strings.Contains(view, "No baseline results yet.") {
		t.Fatal("empty dashboard has no placeholder")
	}
}

func TestLongEvidenceWrapsInsidePane(t *testing.T) {
	results := fixtureResults()
	results[3].Diagnostic = strings.Repeat("anchor timeout with no reply from the destination ", 6)
	verdict, err := domain.AggregateVerdict(results)
	if err != nil {
		t.Fatalf("AggregateVerdict() error = %v", err)
	}
	model := New(results, verdict, WithoutColor()).Resize(80, 24).Select(3)
	view := model.View()
	assertBlockShape(t, view, 80, 24)
	if !strings.Contains(view, "diagnostic") {
		t.Fatal("long diagnostic is missing")
	}
}

func TestUpdateHandlesWindowSize(t *testing.T) {
	model := fixtureModel(t, 80, 24, WithoutColor()).Select(2)
	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Fatal("resize returned a command")
	}
	resized, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update() returned %T", updated)
	}
	if resized.Layout() != LayoutNormal {
		t.Fatalf("layout = %v, want normal", resized.Layout())
	}
	if resized.Selected() != 2 {
		t.Fatalf("selection = %d, want 2", resized.Selected())
	}
}

func TestLayerFor(t *testing.T) {
	tests := map[domain.CheckID]Layer{
		"local.interfaces": LayerLocal,
		"link.state":       LayerLocal,
		"gateway.reach":    LayerGateway,
		"dns.canary":       LayerDNS,
		"icmp.anchor":      LayerTransport,
		"tcp.anchor":       LayerTransport,
		"mtu.pmtu":         LayerTransport,
		"http.canary":      LayerHTTP,
		"captive.portal":   LayerHTTP,
		"custom.check":     LayerOther,
		"noprefix":         LayerOther,
	}
	for id, want := range tests {
		if got := LayerFor(id); got != want {
			t.Errorf("LayerFor(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestRowsFollowPathOrder(t *testing.T) {
	results := []domain.Result{
		testResult("http.canary", "captive.apple.com", domain.StatusSkipped),
		testResult("dns.canary", "example.com", domain.StatusPass),
		testResult("custom.check", "localhost", domain.StatusPass),
		testResult("gateway.reach", "192.168.1.1", domain.StatusWarn),
		testResult("local.interfaces", "en0", domain.StatusPass),
		testResult("icmp.anchor", "1.1.1.1", domain.StatusFail),
	}
	grouped := rows(results)
	want := []Layer{LayerLocal, LayerGateway, LayerDNS, LayerTransport, LayerHTTP, LayerOther}
	for i, entry := range grouped {
		if entry.layer != want[i] {
			t.Fatalf("row %d layer = %q, want %q", i, entry.layer, want[i])
		}
	}
}

func TestHeaderShowsSessionMode(t *testing.T) {
	unprivileged := fixtureModel(t, 100, 30, WithoutColor(), WithMode("unprivileged"))
	if view := unprivileged.View(); !strings.Contains(view, "unprivileged") {
		t.Fatal("header does not label the unprivileged session")
	}

	elevated := fixtureModel(t, 100, 30, WithoutColor(), WithMode("elevated"))
	if view := elevated.View(); !strings.Contains(view, "elevated") {
		t.Fatal("header does not label the elevated session")
	}

	ascii := fixtureModel(t, 100, 30, WithoutColor(), WithASCII(), WithMode("elevated"))
	if view := ascii.View(); !strings.Contains(view, "egress - DEGRADED - elevated") {
		t.Fatal("ASCII header does not use an ASCII separator for the mode")
	}
}
