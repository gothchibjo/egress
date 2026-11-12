package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/gothchibjo/egress/internal/domain"
)

const (
	detailLabelWidth = 12

	destinationKindWidth      = 10
	destinationTransportWidth = 10
	destinationEndpointWidth  = 24
)

type theme struct {
	pass    lipgloss.Style
	warn    lipgloss.Style
	fail    lipgloss.Style
	skip    lipgloss.Style
	muted   lipgloss.Style
	heading lipgloss.Style
}

func (m Model) theme() theme {
	if !m.opts.color {
		return theme{}
	}
	return theme{
		pass:    lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		fail:    lipgloss.NewStyle().Foreground(lipgloss.Color("1")),
		skip:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		heading: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")),
	}
}

func (t theme) status(status domain.Status) lipgloss.Style {
	switch status {
	case domain.StatusPass:
		return t.pass
	case domain.StatusWarn:
		return t.warn
	case domain.StatusFail:
		return t.fail
	default:
		return t.skip
	}
}

func (m Model) verdictStyle() lipgloss.Style {
	if !m.opts.color {
		return lipgloss.Style{}
	}
	style := lipgloss.NewStyle().Bold(true)
	switch m.verdict.Level {
	case domain.VerdictHealthy:
		return style.Foreground(lipgloss.Color("2"))
	case domain.VerdictDegraded:
		return style.Foreground(lipgloss.Color("3"))
	case domain.VerdictBlocked:
		return style.Foreground(lipgloss.Color("1"))
	default:
		return style
	}
}

func (m Model) verdictLabel() string {
	switch {
	case m.runErr != "":
		return "ERROR"
	case m.running:
		return "RUNNING"
	case m.cancelled:
		return "CANCELLED"
	case !m.hasVerdict:
		return "NO VERDICT"
	default:
		return strings.ToUpper(string(m.verdict.Level))
	}
}

func (m Model) renderNormal() string {
	header := append(m.renderHeader(), m.renderScopeBand(m.width)...)
	footer := m.renderFooter()
	bodyHeight := m.height - len(header) - len(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	listWidth := m.width / 3
	if listWidth < 24 {
		listWidth = 24
	}
	if listWidth > 38 {
		listWidth = 38
	}
	const separatorWidth = 3
	detailWidth := m.width - listWidth - separatorWidth
	if detailWidth < 20 {
		detailWidth = 20
		listWidth = m.width - detailWidth - separatorWidth
	}

	list := block(m.listLines(listWidth, bodyHeight), listWidth, bodyHeight)
	separator := block(m.separatorLines(bodyHeight), separatorWidth, bodyHeight)
	detail := block(m.detailPreview(detailWidth, bodyHeight), detailWidth, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, list, separator, detail)

	lines := append(header, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func (m Model) renderCompact() string {
	header := append(m.renderHeader(), m.renderScopeBand(m.width)...)
	footer := m.renderFooter()
	bodyHeight := m.height - len(header) - len(footer)
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	listHeight := bodyHeight * 2 / 5
	if listHeight < 4 {
		listHeight = 4
	}
	detailHeight := bodyHeight - listHeight - 1
	if detailHeight < 3 {
		detailHeight = 3
		listHeight = bodyHeight - detailHeight - 1
		if listHeight < 1 {
			listHeight = 1
		}
	}

	list := block(m.listLines(m.width, listHeight), m.width, listHeight)
	detail := block(m.detailPreview(m.width, detailHeight), m.width, detailHeight)

	lines := append([]string{}, header...)
	lines = append(lines, strings.Split(list, "\n")...)
	lines = append(lines, m.rule(m.width))
	lines = append(lines, strings.Split(detail, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(fitBlock(lines, m.width, m.height), "\n")
}

func (m Model) renderTooSmall() string {
	lines := []string{
		fmt.Sprintf(" Terminal too small: %dx%d", m.width, m.height),
		"",
		fmt.Sprintf(" Need at least %dx%d.", CompactMinWidth, CompactMinHeight),
		" Resize the terminal to continue.",
	}
	return strings.Join(fitBlock(lines, m.width, m.height), "\n")
}

func (m Model) renderHeader() []string {
	left := " egress " + m.dash() + " " + m.verdictLabel()
	if m.scope != "" {
		left += m.separator() + m.scope
	}
	if m.opts.mode != "" {
		left += m.separator() + m.opts.mode
	}
	if m.opts.color {
		left = m.verdictStyle().Render(left)
	}
	right := ""
	if m.hasVerdict && !m.running && m.runErr == "" {
		counts := m.verdict.Counts
		right = fmt.Sprintf("pass %d  warn %d  fail %d  skip %d ", counts.Pass, counts.Warn, counts.Fail, counts.Skipped)
	}
	if m.opts.hasElapsed {
		right += fmt.Sprintf(" %s", m.opts.elapsed.Round(10*time.Millisecond))
	}
	return []string{padLine(left, right, m.width), m.rule(m.width)}
}

// renderScopeBand renders the classification sentence as a band closed by a
// rule. The header rule above it and the returned rule below it keep the
// sentence inside the frame instead of letting it float between the header and
// the result list.
func (m Model) renderScopeBand(width int) []string {
	if m.summary == "" {
		return nil
	}
	limit := 2
	if m.Layout() == LayoutCompact {
		limit = 1
	}
	if width < 4 {
		width = 4
	}

	parts := wrapText(m.summary, width-2)
	if len(parts) > limit {
		parts = parts[:limit]
		parts[limit-1] = clip(parts[limit-1], width-4) + "..."
	}

	lines := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		line := " " + part
		if m.opts.color {
			line = m.theme().muted.Render(line)
		}
		lines = append(lines, padLine(line, "", width))
	}
	return append(lines, m.rule(width))
}

func (m Model) renderFooter() []string {
	if footer := m.exportFooter(); footer != nil {
		return footer
	}
	position := " no results"
	switch {
	case m.notice != "":
		position = " " + m.notice
	case m.running && m.progress != "":
		position = " running " + m.progress
	case len(m.results) > 0:
		position = fmt.Sprintf(" %d/%d selected", m.selected+1, len(m.results))
	}
	return []string{m.rule(m.width), padLine(position, " "+m.footerHints(), m.width)}
}

func (m Model) listLines(width, height int) []string {
	grouped := rows(m.results)
	if len(grouped) == 0 {
		return []string{"", " " + m.emptyMessage()}
	}

	lines := make([]string, 0, len(grouped)+len(layerOrder))
	rowLine := make([]int, len(grouped))
	current := Layer("")
	for i, entry := range grouped {
		if entry.layer != current {
			current = entry.layer
			lines = append(lines, m.theme().heading.Render(" "+strings.ToUpper(string(entry.layer))))
		}
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		id := string(entry.result.CheckID)
		if i == m.selected && m.opts.color {
			id = lipgloss.NewStyle().Bold(true).Render(id)
		}
		rowLine[i] = len(lines)
		lines = append(lines, prefix+m.marker(entry.result.Status)+" "+id)
	}

	if height > 0 && len(lines) > height {
		offset := 0
		if selected := rowLine[m.selected]; selected >= height {
			offset = selected - height + 1
		}
		if maxOffset := len(lines) - height; offset > maxOffset {
			offset = maxOffset
		}
		lines = lines[offset : offset+height]
	}
	return lines
}

// detailLines returns the full evidence for the selected check. Callers decide
// how much of it fits on screen.
func (m Model) detailLines(width int) []string {
	grouped := rows(m.results)
	if len(grouped) == 0 {
		note := " The dashboard appears when the baseline reports results."
		if m.running {
			note = " Slow checks report progress in the footer."
		}
		if m.runErr != "" {
			note = " " + m.runErr
		}
		return []string{
			" " + m.theme().heading.Render("EVIDENCE"),
			"",
			" " + m.emptyMessage(),
			"",
			note,
		}
	}

	entry := grouped[m.selected]
	result := entry.result
	lines := []string{
		" " + m.theme().heading.Render("EVIDENCE"),
	}
	if m.opts.color {
		lines = append(lines, " "+lipgloss.NewStyle().Bold(true).Render(string(result.CheckID)))
	} else {
		lines = append(lines, " "+string(result.CheckID))
	}
	lines = append(lines, m.field("status", m.marker(result.Status)+" "+string(result.Status), width)...)
	lines = append(lines, m.field("layer", string(entry.layer), width)...)
	lines = append(lines, m.field("target", result.Target, width)...)
	lines = append(lines, m.field("method", result.Method, width)...)

	timestamp := m.none()
	if !result.Timestamp.IsZero() {
		timestamp = result.Timestamp.UTC().Format("2006-01-02 15:04:05 MST")
	}
	lines = append(lines, m.field("time", timestamp, width)...)

	if len(result.Measurements) > 0 {
		for i, measurement := range result.Measurements {
			label := "measured"
			if i > 0 {
				label = ""
			}
			value := measurement.Name + " = " + measurement.Value
			if measurement.Unit != "" {
				value += " " + measurement.Unit
			}
			lines = append(lines, m.field(label, value, width)...)
		}
	}
	if result.Diagnostic != "" {
		lines = append(lines, m.evidenceField("diagnostic", result.Diagnostic, width, result)...)
	}
	if result.Error != "" {
		lines = append(lines, m.evidenceField("error", result.Error, width, result)...)
	}

	return lines
}

// detailPreview truncates the evidence pane and marks the cut, so a long check
// never looks complete when it is not.
func (m Model) detailPreview(width, height int) []string {
	lines := m.detailLines(width)
	if height <= 0 || len(lines) <= height {
		return lines
	}
	visible := height - 1
	if visible < 0 {
		visible = 0
	}
	preview := make([]string, 0, height)
	preview = append(preview, lines[:visible]...)
	preview = append(preview, m.theme().muted.Render(" ... enter for the full evidence"))
	return preview
}

func (m Model) field(label, value string, width int) []string {
	return m.fieldWithMarker(label, value, width, "")
}

// evidenceField marks the lines that explain a failure. The marker prefixes the
// label, matching the `✗ fail` and `! warn` markers used elsewhere.
func (m Model) evidenceField(label, value string, width int, result domain.Result) []string {
	status := domain.Status("")
	if result.Status == domain.StatusFail || result.Status == domain.StatusWarn {
		status = result.Status
		label = "! " + label
	}
	return m.fieldWithMarker(label, value, width, status)
}

func (m Model) fieldWithMarker(label, value string, width int, status domain.Status) []string {
	if value == "" {
		value = m.none()
	}
	wrapWidth := width - detailLabelWidth - 2
	if wrapWidth < 8 {
		wrapWidth = 8
	}
	parts := wrapText(value, wrapWidth)
	style := m.theme().status(status)
	lines := make([]string, 0, len(parts))
	for i, part := range parts {
		var line string
		if i == 0 {
			line = " " + padLabel(label) + " " + part
		} else {
			line = " " + strings.Repeat(" ", detailLabelWidth) + " " + part
		}
		if m.opts.color && status != "" {
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) separatorLines(height int) []string {
	line := " " + m.vertical() + " "
	if m.opts.color {
		line = m.theme().muted.Render(line)
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = line
	}
	return lines
}

func (m Model) marker(status domain.Status) string {
	t := m.theme()
	if m.opts.ascii {
		switch status {
		case domain.StatusPass:
			return t.pass.Render("[ok]")
		case domain.StatusWarn:
			return t.warn.Render("[!!]")
		case domain.StatusFail:
			return t.fail.Render("[xx]")
		default:
			return t.skip.Render("[--]")
		}
	}
	switch status {
	case domain.StatusPass:
		return t.pass.Render("✓")
	case domain.StatusWarn:
		return t.warn.Render("!")
	case domain.StatusFail:
		return t.fail.Render("✗")
	default:
		return t.skip.Render("·")
	}
}

func (m Model) rule(width int) string {
	return strings.Repeat(m.horizontal(), width)
}

func (m Model) horizontal() string {
	if m.opts.ascii {
		return "-"
	}
	return "─"
}

func (m Model) vertical() string {
	if m.opts.ascii {
		return "|"
	}
	return "│"
}

func (m Model) dash() string {
	if m.opts.ascii {
		return "-"
	}
	return "—"
}

func (m Model) none() string {
	if m.opts.ascii {
		return "-"
	}
	return "—"
}

func padLabel(label string) string {
	if len(label) >= detailLabelWidth {
		return label[:detailLabelWidth]
	}
	return label + strings.Repeat(" ", detailLabelWidth-len(label))
}

func padLine(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return clip(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

func clip(line string, width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func block(lines []string, width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 0 {
		height = 0
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		if len(out) >= height {
			break
		}
		line = clip(line, width)
		if pad := width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		out = append(out, line)
	}
	blank := strings.Repeat(" ", width)
	for len(out) < height {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

func fitBlock(lines []string, width, height int) []string {
	return strings.Split(block(lines, width, height), "\n")
}

func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, len(words)/4+1)
	current := words[0]
	for _, word := range words[1:] {
		if len([]rune(current))+1+len([]rune(word)) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	return append(lines, current)
}

func (m Model) renderPageHeader(title, right string) []string {
	left := " egress " + m.dash() + " " + title
	if m.opts.color {
		left = m.theme().heading.Render(left)
	}
	return []string{padLine(left, right, m.width), m.rule(m.width)}
}

func (m Model) renderHintFooter(hints string) []string {
	return []string{m.rule(m.width), padLine("", " "+hints, m.width)}
}

func (m Model) renderHelp() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}
	header := m.renderPageHeader("HELP", "")
	footer := m.renderHintFooter(m.footerHints())
	bodyHeight := m.helpBodyHeight()

	content := helpLines()
	offset := clampOffset(m.helpOffset, m.helpMaxOffset())
	body := block(content[offset:], m.width, bodyHeight)
	lines := append(header, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

// helpBodyHeight is the number of help lines the overlay can show.
func (m Model) helpBodyHeight() int {
	header := m.renderPageHeader("HELP", "")
	footer := m.renderHintFooter("")
	height := m.height - len(header) - len(footer)
	if height < 1 {
		height = 1
	}
	return height
}

// helpMaxOffset is the furthest the help overlay can scroll.
func (m Model) helpMaxOffset() int {
	total := len(helpLines())
	height := m.helpBodyHeight()
	if total <= height {
		return 0
	}
	return total - height
}

func (m Model) renderDetailPage() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}
	bodyHeight := m.detailBodyHeight()
	offset := clampOffset(m.detailOffset, m.detailMaxOffset())
	evidence := m.detailLines(m.width)

	position := ""
	if len(m.results) > 0 {
		position = fmt.Sprintf("%d/%d", m.selected+1, len(m.results))
	}
	if len(evidence) > bodyHeight {
		end := offset + bodyHeight
		if end > len(evidence) {
			end = len(evidence)
		}
		position += fmt.Sprintf("  %d-%d/%d", offset+1, end, len(evidence))
	}
	if position != "" {
		position += " "
	}

	header := m.renderPageHeader("DETAIL", position)
	footer := m.renderHintFooter(m.footerHints())
	body := block(evidence[offset:], m.width, bodyHeight)
	lines := append(header, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

// detailBodyHeight is the number of evidence lines the detail page can show.
func (m Model) detailBodyHeight() int {
	header := m.renderPageHeader("DETAIL", "")
	footer := m.renderHintFooter("")
	height := m.height - len(header) - len(footer)
	if height < 1 {
		height = 1
	}
	return height
}

// detailMaxOffset is the furthest the detail page can scroll.
func (m Model) detailMaxOffset() int {
	total := len(m.detailLines(m.width))
	height := m.detailBodyHeight()
	if total <= height {
		return 0
	}
	return total - height
}

func (m Model) renderContext() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}
	header := m.renderPageHeader("CONTEXT", "")
	footer := m.renderHintFooter(m.footerHints())
	bodyHeight := m.contextBodyHeight()

	lines := m.contextLines()
	offset := clampOffset(m.contextOffset, m.contextMaxOffset())
	body := block(lines[offset:], m.width, bodyHeight)
	out := append(header, strings.Split(body, "\n")...)
	out = append(out, footer...)
	return strings.Join(out, "\n")
}

func (m Model) contextBodyHeight() int {
	header := m.renderPageHeader("CONTEXT", "")
	footer := m.renderHintFooter("")
	height := m.height - len(header) - len(footer)
	if height < 1 {
		height = 1
	}
	return height
}

func (m Model) contextMaxOffset() int {
	total := len(m.contextLines())
	height := m.contextBodyHeight()
	if total <= height {
		return 0
	}
	return total - height
}

// contextLines renders the read-only proxy and tunnel context.
func (m Model) contextLines() []string {
	lines := []string{"", " Read-only context. It never changes which probes run."}
	for _, part := range wrapText("Sources: proxy variables from the environment; tunnel interface names, with the default route marked when it applies.", m.width-2) {
		lines = append(lines, " "+part)
	}
	lines = append(lines, "")
	if m.opts.contextErr != "" {
		lines = append(lines, " Interface list unavailable: "+m.opts.contextErr, "")
	}
	if m.opts.networkContext == nil {
		return append(lines, " No context was collected in this session.")
	}

	context := m.opts.networkContext
	lines = append(lines, " "+m.theme().heading.Render("PROXY"))
	if len(context.Proxies) == 0 {
		lines = append(lines, "   none detected")
	}
	for _, proxy := range context.Proxies {
		lines = append(lines,
			"   "+proxy.Name+"  "+proxy.Value,
			"     source  environment ("+proxy.Source+")",
		)
	}

	lines = append(lines, "", " "+m.theme().heading.Render("VPN"))
	if len(context.VPN) == 0 {
		lines = append(lines, "   none detected")
	}
	for _, iface := range context.VPN {
		summary := iface.Kind + "  " + iface.State
		if iface.DefaultRoute {
			summary += "  [default route]"
		}
		lines = append(lines, "   "+iface.Name+"  "+summary)
		if len(iface.Addresses) > 0 {
			lines = append(lines, "     addresses  "+strings.Join(iface.Addresses, ", "))
		}
	}
	return lines
}

func (m Model) renderDestinations() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}
	position := ""
	if count := len(m.opts.destinations); count > 0 {
		position = fmt.Sprintf("%d endpoints ", count)
	}
	header := m.renderPageHeader("DESTINATIONS", position)
	footer := m.renderHintFooter(m.footerHints())
	bodyHeight := m.destinationBodyHeight()

	lines := m.destinationLines()
	offset := m.destinationOffset
	if maxOffset := len(lines) - bodyHeight; offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	body := block(lines[offset:], m.width, bodyHeight)
	out := append(header, strings.Split(body, "\n")...)
	out = append(out, footer...)
	return strings.Join(out, "\n")
}

func (m Model) destinationBodyHeight() int {
	header := m.renderPageHeader("DESTINATIONS", "")
	footer := m.renderHintFooter("")
	height := m.height - len(header) - len(footer)
	if height < 1 {
		height = 1
	}
	return height
}

func (m Model) destinationMaxOffset() int {
	lines := len(m.destinationLines())
	height := m.destinationBodyHeight()
	if lines <= height {
		return 0
	}
	return lines - height
}

// destinationLines builds the documented outbound table. The purpose column is
// dropped on narrow terminals so the endpoint always stays readable.
func (m Model) destinationLines() []string {
	intro := wrapText("Only the endpoints listed here are contacted. No telemetry, analytics, crash reports, uploads or enrichment.", m.width-2)
	lines := []string{""}
	for _, part := range intro {
		lines = append(lines, " "+part)
	}
	lines = append(lines, "")

	if m.opts.destinationErr != "" {
		lines = append(lines, " Resolver list unavailable: "+m.opts.destinationErr, "")
	}
	if len(m.opts.destinations) == 0 {
		return append(lines, " No documented destinations.")
	}

	// The purpose column appears only when a full sentence fits; otherwise the
	// endpoint stays readable and the purpose lives in the documentation.
	withPurpose := m.width >= 96
	if withPurpose {
		lines = append(lines, fmt.Sprintf(" %-*s %-*s %-*s %s",
			destinationKindWidth, "KIND",
			destinationTransportWidth, "TRANSPORT",
			destinationEndpointWidth, "ENDPOINT",
			"PURPOSE",
		))
	} else {
		lines = append(lines, fmt.Sprintf(" %-*s %-*s %s",
			destinationKindWidth, "KIND",
			destinationTransportWidth, "TRANSPORT",
			"ENDPOINT",
		))
	}
	for _, destination := range m.opts.destinations {
		row := fmt.Sprintf(" %-*s %-*s %-*s",
			destinationKindWidth, destinationKindLabel(destination.Kind),
			destinationTransportWidth, destination.Transport,
			destinationEndpointWidth, destination.Endpoint(),
		)
		if withPurpose {
			row += " " + destination.Purpose
		}
		lines = append(lines, row)
	}
	return lines
}

func destinationKindLabel(kind domain.DestinationKind) string {
	switch kind {
	case domain.DestinationResolver:
		return "resolver"
	case domain.DestinationDoH:
		return "doh"
	case domain.DestinationAnchor:
		return "anchor"
	case domain.DestinationPathMTU:
		return "path-mtu"
	case domain.DestinationCaptive:
		return "captive"
	default:
		return string(kind)
	}
}

func (m Model) renderConfirm() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}
	header := m.renderPageHeader("QUIT?", "")
	footer := m.renderHintFooter("y quit" + m.separator() + "n cancel")
	bodyHeight := m.height - len(header) - len(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := block([]string{
		"",
		" Quit egress?",
		"",
		" y  quit",
		" n  cancel",
		"",
		" Ctrl-C again also quits.",
	}, m.width, bodyHeight)
	lines := append(header, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

func (m Model) emptyMessage() string {
	switch {
	case m.runErr != "":
		return "Baseline failed."
	case m.running:
		return "Baseline running..."
	default:
		return "No baseline results yet."
	}
}
