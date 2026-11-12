package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const dnsSummary = "Local link and gateway were usable but name resolution failed. Local inspection and gateway probing ran."

func bandModel(t *testing.T, width, height int, summary string, opts ...Option) Model {
	t.Helper()
	model := fixtureModel(t, width, height, opts...)
	model.scope = "dns"
	model.summary = summary
	return model
}

func isRuleLine(line string) bool {
	runes := []rune(line)
	if len(runes) == 0 {
		return false
	}
	if runes[0] != '-' && runes[0] != '─' {
		return false
	}
	for _, r := range runes {
		if r != runes[0] {
			return false
		}
	}
	return true
}

// bandLines returns the text lines of the scope band and the rule that closes
// it. The band starts after the header rule.
func bandLines(t *testing.T, view string) (text []string, closing string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	first := -1
	for i, line := range lines {
		if strings.Contains(line, "Local link and gateway") || strings.Contains(line, "upstream probes") {
			first = i
			break
		}
	}
	if first < 0 {
		t.Fatal("scope band is missing")
	}
	if first == 0 || !isRuleLine(lines[first-1]) {
		t.Fatalf("band is not opened by a rule: %q", lines[first-1])
	}
	last := first
	for last+1 < len(lines) && !isRuleLine(lines[last+1]) {
		last++
	}
	if last+1 >= len(lines) {
		t.Fatal("band is not closed by a rule")
	}
	return lines[first : last+1], lines[last+1]
}

func TestScopeBandIsClosedByRules(t *testing.T) {
	model := bandModel(t, 100, 30, dnsSummary, WithoutColor())
	text, closing := bandLines(t, model.View())

	if len(text) != 2 {
		t.Fatalf("band has %d text lines, want 2", len(text))
	}
	if closing != model.rule(100) {
		t.Fatalf("closing rule = %q", closing)
	}
}

func TestScopeBandWrapsLongSummary(t *testing.T) {
	model := bandModel(t, 80, 24, dnsSummary, WithoutColor())
	text, _ := bandLines(t, model.View())

	var joined []string
	for _, line := range text {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "…") {
			t.Fatalf("summary was truncated at 80 columns: %q", line)
		}
		joined = append(joined, trimmed)
	}
	if got := strings.Join(joined, " "); got != dnsSummary {
		t.Fatalf("band text = %q, want %q", got, dnsSummary)
	}
}

func TestScopeBandTruncatesWithEllipsis(t *testing.T) {
	long := strings.Repeat("upstream probes returned no response from any anchor ", 4)
	model := bandModel(t, 100, 30, long, WithoutColor())
	text, _ := bandLines(t, model.View())

	if len(text) != 2 {
		t.Fatalf("band has %d text lines, want 2", len(text))
	}
	if !strings.HasSuffix(strings.TrimRight(text[len(text)-1], " "), "…") {
		t.Fatalf("last band line is not truncated: %q", text[len(text)-1])
	}
}

func TestScopeBandCompactSingleLine(t *testing.T) {
	model := bandModel(t, 60, 20, dnsSummary, WithoutColor())
	if model.Layout() != LayoutCompact {
		t.Fatalf("layout = %v, want compact", model.Layout())
	}
	text, _ := bandLines(t, model.View())

	if len(text) != 1 {
		t.Fatalf("compact band has %d text lines, want 1", len(text))
	}
	if !strings.HasSuffix(strings.TrimRight(text[0], " "), "…") {
		t.Fatalf("compact band is not truncated: %q", text[0])
	}
}

func TestNoScopeBandWithoutClassification(t *testing.T) {
	model := fixtureModel(t, 100, 30, WithoutColor())
	lines := strings.Split(model.View(), "\n")

	if !isRuleLine(lines[1]) {
		t.Fatalf("header rule is missing: %q", lines[1])
	}
	if isRuleLine(lines[2]) {
		t.Fatal("healthy run renders an empty scope band")
	}
}

func TestScopeBandASCII(t *testing.T) {
	view := bandModel(t, 80, 24, dnsSummary, WithoutColor(), WithASCII()).View()
	if strings.Contains(view, "─") {
		t.Fatal("ASCII mode renders a Unicode rule")
	}
	lines := strings.Split(view, "\n")
	if !isRuleLine(lines[1]) || lines[1][0] != '-' {
		t.Fatalf("ASCII header rule = %q", lines[1])
	}
}

func TestScopeBandColor(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)
	lipgloss.SetColorProfile(termenv.ANSI256)

	model := bandModel(t, 100, 30, dnsSummary, WithColor())
	view := model.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("colored band rendered without ANSI styling")
	}
	if stripped := ansiPattern.ReplaceAllString(view, ""); !strings.Contains(strings.Join(strings.Fields(stripped), " "), dnsSummary) {
		t.Fatal("colored band lost the summary text")
	}
}

func TestViewKeepsTerminalHeightWithBand(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)

	for _, size := range []struct{ width, height int }{
		{width: 100, height: 30},
		{width: 80, height: 24},
		{width: 60, height: 20},
	} {
		lipgloss.SetColorProfile(termenv.Ascii)
		assertBlockShape(t, bandModel(t, size.width, size.height, dnsSummary, WithoutColor()).View(), size.width, size.height)

		lipgloss.SetColorProfile(termenv.ANSI256)
		assertBlockShape(t, bandModel(t, size.width, size.height, dnsSummary, WithColor()).View(), size.width, size.height)
	}
}
