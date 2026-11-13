package tui

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gothchibjo/egress/internal/domain"
)

// customResolveTimeout bounds one hostname resolution in the picker.
const customResolveTimeout = 3 * time.Second

// Column thresholds for the picker layout.
const (
	pickerTwoColumnMinWidth   = 100
	pickerThreeColumnMinWidth = 160
)

// pickerState is the target selection screen: presets grouped by category,
// custom targets, a filter and the selected set.
type pickerState struct {
	cursor       int
	filter       string
	filtering    bool
	selected     map[string]bool
	custom       []customTarget
	customInput  string
	customActive bool
}

// customTarget is a parsed custom destination with its resolution state.
type customTarget struct {
	Input     string
	Target    domain.Target
	Addresses []string
	Resolving bool
	Err       string
}

// pickerEntry is one navigable row: a preset or a custom target.
type pickerEntry struct {
	preset domain.Preset
	custom *customTarget
}

func (e pickerEntry) isCustom() bool {
	return e.custom != nil
}

// openPicker shows the picker without losing the current selection.
func (m Model) openPicker() Model {
	m.picker.filter = ""
	m.picker.filtering = false
	m.picker.customInput = ""
	m.picker.customActive = false
	m.picker.cursor = 0
	m.view = viewPicker
	return m
}

// updatePickerKey handles navigation, selection, filtering and the custom
// target prompt. Filtering and custom input are modal: their keys never trigger
// list actions.
func (m Model) updatePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m = m.clearNotice()

	if m.picker.filtering {
		switch msg.Type {
		case tea.KeyEsc:
			m.picker.filtering = false
			m.picker.filter = ""
			m.picker.cursor = 0
		case tea.KeyEnter:
			m.picker.filtering = false
		case tea.KeyBackspace:
			if runes := []rune(m.picker.filter); len(runes) > 0 {
				m.picker.filter = string(runes[:len(runes)-1])
				m.picker.cursor = 0
			}
		case tea.KeyRunes:
			m.picker.filter += string(msg.Runes)
			m.picker.cursor = 0
		}
		return m.normalizePicker(), nil
	}

	if m.picker.customActive {
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.picker.customActive = false
			m.picker.customInput = ""
		case tea.KeyBackspace:
			if runes := []rune(m.picker.customInput); len(runes) > 0 {
				m.picker.customInput = string(runes[:len(runes)-1])
			}
		case tea.KeyEnter:
			return m.addCustomTarget()
		case tea.KeyRunes:
			m.picker.customInput += string(msg.Runes)
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		m.picker.cursor--
	case "down", "j":
		m.picker.cursor++
	case "left":
		m = m.movePickerColumn(-1)
	case "right":
		m = m.movePickerColumn(1)
	case "g":
		m.picker.cursor = 0
	case "G":
		m.picker.cursor = len(m.pickerEntries()) - 1
	case " ":
		m = m.togglePickerRow()
	case "a":
		m = m.toggleAllVisible()
	case "/":
		m.picker.filtering = true
	case "c":
		m.picker.customActive = true
		m.picker.customInput = ""
	case "enter":
		return m.startSelection()
	case "esc":
		m.view = viewDashboard
	case "?":
		m.helpReturn = viewPicker
		m.helpOffset = 0
		m.view = viewHelp
	case "q":
		return m, tea.Quit
	case "ctrl+c":
		m.confirmQuit = true
	}
	return m.normalizePicker(), nil
}

// startSelection starts a run only when at least one target is selected.
func (m Model) startSelection() (tea.Model, tea.Cmd) {
	presets, custom := m.pickerSelection()
	if len(presets)+len(custom) == 0 {
		return m.setNotice("select at least one target")
	}
	if m.opts.startSelection == nil {
		return m.setNotice("the run view is not available yet")
	}
	targets := m.opts.startSelection(presets, custom)
	if len(targets) == 0 {
		return m.setNotice("no targets to probe")
	}
	return m.startRun(targets), nil
}

// addCustomTarget parses and stores the typed target, then resolves hostnames in
// the background so the addresses are visible before the run starts.
func (m Model) addCustomTarget() (tea.Model, tea.Cmd) {
	parsed, err := domain.ParseTarget(m.picker.customInput)
	if err != nil {
		return m.setNotice(err.Error())
	}
	for _, existing := range m.picker.custom {
		if existing.Target.Key() == parsed.Key() {
			return m.setNotice("target is already selected")
		}
	}

	m.picker.custom = append(m.picker.custom, customTarget{
		Input:     parsed.String(),
		Target:    parsed,
		Resolving: net.ParseIP(parsed.Host) == nil,
	})
	m.picker.customActive = false
	m.picker.customInput = ""
	m.picker.cursor = len(m.pickerEntries()) - 1

	if ip := net.ParseIP(parsed.Host); ip != nil {
		return m, nil
	}
	return m, m.resolveCustom(parsed)
}

// resolveCustom resolves one custom hostname off the event loop.
func (m Model) resolveCustom(target domain.Target) tea.Cmd {
	resolve := m.opts.resolve
	key := target.Key()
	host := target.Host
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), customResolveTimeout)
		defer cancel()
		addresses, err := resolve(ctx, host)
		if err != nil {
			return customResolvedMsg{Key: key, Err: err.Error()}
		}
		values := make([]string, 0, len(addresses))
		for _, address := range addresses {
			values = append(values, address.String())
		}
		return customResolvedMsg{Key: key, Addresses: values}
	}
}

// defaultResolver resolves a hostname with the system resolver.
func defaultResolver(ctx context.Context, host string) ([]net.IP, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ips = append(ips, address.IP)
	}
	return ips, nil
}

// customResolvedMsg stores the resolution outcome on the matching target.
type customResolvedMsg struct {
	Key       string
	Addresses []string
	Err       string
}

// toggleAllVisible selects every visible preset, or clears them when all are
// already selected. Hidden presets and custom rows are left alone.
func (m Model) toggleAllVisible() Model {
	visible := make([]domain.Preset, 0, len(domain.PresetCatalog()))
	for _, entry := range m.pickerEntries() {
		if !entry.isCustom() {
			visible = append(visible, entry.preset)
		}
	}
	if len(visible) == 0 {
		return m
	}

	allSelected := true
	for _, preset := range visible {
		if !m.picker.selected[preset.ID] {
			allSelected = false
			break
		}
	}
	for _, preset := range visible {
		if allSelected {
			delete(m.picker.selected, preset.ID)
			continue
		}
		m.picker.selected[preset.ID] = true
	}
	return m.normalizePicker()
}

// togglePickerRow selects a preset or removes a custom row.
func (m Model) togglePickerRow() Model {
	entries := m.pickerEntries()
	if m.picker.cursor < 0 || m.picker.cursor >= len(entries) {
		return m
	}
	entry := entries[m.picker.cursor]
	if entry.isCustom() {
		visiblePresets := 0
		for _, candidate := range entries {
			if !candidate.isCustom() {
				visiblePresets++
			}
		}
		index := m.picker.cursor - visiblePresets
		if index >= 0 && index < len(m.picker.custom) {
			m.picker.custom = append(m.picker.custom[:index], m.picker.custom[index+1:]...)
		}
		return m.normalizePicker()
	}

	id := entry.preset.ID
	if m.picker.selected[id] {
		delete(m.picker.selected, id)
	} else {
		m.picker.selected[id] = true
	}
	return m.normalizePicker()
}

// normalizePicker keeps the cursor inside the visible rows.
func (m Model) normalizePicker() Model {
	entries := m.pickerEntries()
	switch {
	case len(entries) == 0:
		m.picker.cursor = 0
	case m.picker.cursor < 0:
		m.picker.cursor = 0
	case m.picker.cursor >= len(entries):
		m.picker.cursor = len(entries) - 1
	}
	return m
}

// pickerEntries returns the visible rows in display order: presets grouped by
// category in first-appearance order, then custom targets. The filter matches
// presets only.
func (m Model) pickerEntries() []pickerEntry {
	filter := strings.ToLower(strings.TrimSpace(m.picker.filter))

	order := make([]string, 0, 8)
	byCategory := make(map[string][]domain.Preset, 8)
	for _, preset := range domain.PresetCatalog() {
		if filter != "" && !presetMatches(preset, filter) {
			continue
		}
		if _, seen := byCategory[preset.Category]; !seen {
			order = append(order, preset.Category)
		}
		byCategory[preset.Category] = append(byCategory[preset.Category], preset)
	}

	entries := make([]pickerEntry, 0, len(domain.PresetCatalog())+len(m.picker.custom))
	for _, category := range order {
		for _, preset := range byCategory[category] {
			entries = append(entries, pickerEntry{preset: preset})
		}
	}
	for i := range m.picker.custom {
		entries = append(entries, pickerEntry{custom: &m.picker.custom[i]})
	}
	return entries
}

func presetMatches(preset domain.Preset, filter string) bool {
	haystack := strings.ToLower(preset.DisplayName + " " + preset.Host + " " + preset.Category)
	return strings.Contains(haystack, filter)
}

// pickerSelection returns the selected presets and custom targets.
func (m Model) pickerSelection() ([]domain.Preset, []domain.Target) {
	presets := make([]domain.Preset, 0, len(m.picker.selected))
	for _, preset := range domain.PresetCatalog() {
		if m.picker.selected[preset.ID] {
			presets = append(presets, preset)
		}
	}
	custom := make([]domain.Target, 0, len(m.picker.custom))
	for _, entry := range m.picker.custom {
		custom = append(custom, entry.Target)
	}
	return presets, custom
}

// selectedCount is the number shown in the picker header.
func (m Model) selectedCount() int {
	presets, custom := m.pickerSelection()
	return len(presets) + len(custom)
}

// renderPicker draws the target selection page.
func (m Model) renderPicker() string {
	switch m.Layout() {
	case LayoutStopped:
		return ""
	case LayoutTooSmall:
		return m.renderTooSmall()
	}

	header := m.renderPageHeader("TARGETS", fmt.Sprintf("%d selected ", m.selectedCount()))

	footer := make([]string, 0, 3)
	if m.picker.filtering {
		footer = append(footer, padLine(" filter: "+m.picker.filter+"_", " enter apply"+m.separator()+"esc clear", m.width))
	} else if m.picker.customActive {
		footer = append(footer, padLine(" custom target: "+m.picker.customInput+"_", " enter add"+m.separator()+"esc cancel", m.width))
	}
	if m.notice != "" {
		footer = append(footer, padLine(" "+m.notice, "", m.width))
	}
	footer = append(footer, m.renderHintFooter(m.footerHints())...)

	bodyHeight := m.pickerBodyHeight()
	body := block(m.pickerLines(bodyHeight), m.width, bodyHeight)
	lines := append(header, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

// pickerLines renders the grouped rows with checkboxes. Narrow terminals keep
// one scrolling column; wide terminals flow the categories into two or three
// columns and paginate by column.
func (m Model) pickerLines(height int) []string {
	entries := m.pickerEntries()
	if len(entries) == 0 {
		return []string{"", " no targets match the filter"}
	}
	if columns := m.pickerColumns(); columns > 1 {
		return m.pickerGrid(entries, columns, height)
	}

	lines := make([]string, 0, len(entries)+8)
	rowLine := make([]int, len(entries))
	category := ""
	for i, entry := range entries {
		if entry.isCustom() {
			if category != "\x00custom" {
				category = "\x00custom"
				lines = append(lines, "", " "+m.theme().heading.Render("CUSTOM"))
			}
		} else if entry.preset.Category != category {
			category = entry.preset.Category
			lines = append(lines, "", " "+m.theme().heading.Render(strings.ToUpper(category)))
		}

		rowLine[i] = len(lines)
		prefix := "  "
		if i == m.picker.cursor {
			prefix = "> "
		}
		if entry.isCustom() {
			lines = append(lines, prefix+"[x] "+m.customTargetLabel(entry.custom))
			continue
		}

		box := "[ ]"
		if m.picker.selected[entry.preset.ID] {
			box = "[x]"
		}
		lines = append(lines, m.pickerPresetLine(entry.preset, prefix, box))
	}

	if height > 0 && len(lines) > height {
		offset := 0
		if selected := rowLine[m.picker.cursor]; selected >= height {
			offset = selected - height + 1
		}
		if maxOffset := len(lines) - height; offset > maxOffset {
			offset = maxOffset
		}
		lines = lines[offset : offset+height]
	}
	return lines
}

// pickerColumns returns the number of columns for the current width.
func (m Model) pickerColumns() int {
	switch {
	case m.width >= pickerThreeColumnMinWidth:
		return 3
	case m.width >= pickerTwoColumnMinWidth:
		return 2
	default:
		return 1
	}
}

// pickerItem is a heading or an entry in the column flow.
type pickerItem struct {
	heading string
	entry   *pickerEntry
	index   int
}

// pickerLayout is the rendered grid: item indices per column plus the position
// of every item. Rendering and arrow-key navigation share it so they cannot
// disagree.
type pickerLayout struct {
	items     []pickerItem
	columns   [][]int
	columnOf  []int
	rowOf     []int
	entryItem []int
}

func (m Model) pickerLayout(entries []pickerEntry, columns, height int) pickerLayout {
	items, entryItem := m.pickerItems(entries)
	layout := pickerLayout{items: items, entryItem: entryItem}
	if len(items) == 0 || height < 1 {
		return layout
	}

	blocks := pickerBlocks(items)
	target := (len(items) + columns - 1) / columns
	if target < 1 {
		target = 1
	}
	if target > height {
		target = height
	}

	current := make([]int, 0, target)
	flush := func() {
		if len(current) == 0 {
			return
		}
		layout.columns = append(layout.columns, current)
		current = make([]int, 0, target)
	}

	for _, block := range blocks {
		for len(block) > 0 {
			if len(current) > 0 && len(current)+len(block) > target && len(current) >= target/2 && len(layout.columns) < columns-1 {
				flush()
				continue
			}
			if len(current) >= height {
				flush()
				continue
			}
			limit := height - len(current)
			take := len(block)
			if take > limit {
				take = limit
			}
			current = append(current, block[:take]...)
			block = block[take:]
			if len(current) >= height {
				flush()
			}
		}
	}
	flush()

	layout.columnOf = make([]int, len(items))
	layout.rowOf = make([]int, len(items))
	for c, column := range layout.columns {
		for row, index := range column {
			layout.columnOf[index] = c
			layout.rowOf[index] = row
		}
	}
	return layout
}

// pickerGrid renders the balanced columns for the current page.
func (m Model) pickerGrid(entries []pickerEntry, columns, height int) []string {
	layout := m.pickerLayout(entries, columns, height)
	if len(layout.items) == 0 || len(layout.columns) == 0 {
		return nil
	}

	cursor := m.picker.cursor
	if cursor < 0 || cursor >= len(layout.entryItem) {
		cursor = 0
	}
	cursorColumn := layout.columnOf[layout.entryItem[cursor]]
	start := (cursorColumn / columns) * columns
	if start >= len(layout.columns) {
		start = 0
	}

	columnWidth := (m.width - 2*(columns-1)) / columns
	if columnWidth < 20 {
		columnWidth = 20
	}

	rendered := make([]string, 0, columns)
	for c := start; c < start+columns && c < len(layout.columns); c++ {
		lines := make([]string, 0, len(layout.columns[c]))
		for _, index := range layout.columns[c] {
			lines = append(lines, m.pickerItemLine(layout.items[index]))
		}
		rendered = append(rendered, block(lines, columnWidth, height))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, rendered...), "\n")
}

// movePickerColumn moves the cursor to the nearest entry in the adjacent
// column. Edges and the single-column layout do not move.
func (m Model) movePickerColumn(delta int) Model {
	columns := m.pickerColumns()
	if columns <= 1 {
		return m
	}
	layout := m.pickerLayout(m.pickerEntries(), columns, m.pickerBodyHeight())
	if len(layout.columns) <= 1 {
		return m
	}
	cursor := m.picker.cursor
	if cursor < 0 || cursor >= len(layout.entryItem) {
		return m
	}
	item := layout.entryItem[cursor]
	column := layout.columnOf[item]
	target := column + delta
	if target < 0 || target >= len(layout.columns) {
		return m
	}

	row := layout.rowOf[item]
	chosen, last := -1, -1
	for _, index := range layout.columns[target] {
		if layout.items[index].heading != "" {
			continue
		}
		last = index
		if chosen == -1 && layout.rowOf[index] >= row {
			chosen = index
		}
	}
	if chosen == -1 {
		chosen = last
	}
	if chosen == -1 {
		return m
	}
	m.picker.cursor = layout.items[chosen].index
	return m.normalizePicker()
}

// pickerBodyHeight is the number of rows available to the picker body.
func (m Model) pickerBodyHeight() int {
	header := m.renderPageHeader("TARGETS", "")
	footer := 2 // rule and hint line
	if m.picker.filtering || m.picker.customActive {
		footer++
	}
	if m.notice != "" {
		footer++
	}
	height := m.height - len(header) - footer
	if height < 1 {
		height = 1
	}
	return height
}

// pickerBlocks groups the flat item list into heading-plus-entries blocks.
func pickerBlocks(items []pickerItem) [][]int {
	blocks := make([][]int, 0, 8)
	for i, item := range items {
		if item.heading != "" || len(blocks) == 0 {
			blocks = append(blocks, []int{i})
			continue
		}
		last := len(blocks) - 1
		blocks[last] = append(blocks[last], i)
	}
	return blocks
}

// pickerItems flattens the grouped rows into headings and entries.
func (m Model) pickerItems(entries []pickerEntry) ([]pickerItem, []int) {
	items := make([]pickerItem, 0, len(entries)+8)
	entryItem := make([]int, len(entries))
	category := ""
	for i, entry := range entries {
		if entry.isCustom() {
			if category != "\x00custom" {
				category = "\x00custom"
				items = append(items, pickerItem{heading: "CUSTOM"})
			}
		} else if entry.preset.Category != category {
			category = entry.preset.Category
			items = append(items, pickerItem{heading: strings.ToUpper(category)})
		}
		entryItem[i] = len(items)
		items = append(items, pickerItem{entry: &entries[i], index: i})
	}
	return items, entryItem
}

// pickerItemLine renders one heading or row inside a column.
func (m Model) pickerItemLine(item pickerItem) string {
	if item.heading != "" {
		return " " + m.theme().heading.Render(item.heading)
	}
	prefix := "  "
	if item.index == m.picker.cursor {
		prefix = "> "
	}
	if item.entry.isCustom() {
		return prefix + "[x] " + m.customTargetLabel(item.entry.custom)
	}
	box := "[ ]"
	if m.picker.selected[item.entry.preset.ID] {
		box = "[x]"
	}
	return m.pickerPresetLine(item.entry.preset, prefix, box)
}

// customTargetLabel renders a custom target with its resolution state.
func (m Model) customTargetLabel(entry *customTarget) string {
	label := entry.Input
	switch {
	case entry.Resolving:
		label += " (resolving...)"
	case entry.Err != "":
		label += " (unresolved: " + entry.Err + ")"
	case len(entry.Addresses) > 0:
		label += " (" + strings.Join(entry.Addresses, ", ") + ")"
	}
	return label
}

// pickerPresetLine renders one preset as "Name (host)". The host is muted when
// colour is enabled so the name leads, and no fixed padding is used, which keeps
// multi-column layouts compact.
func (m Model) pickerPresetLine(preset domain.Preset, prefix, box string) string {
	host := " (" + preset.Host + ")"
	if m.opts.color {
		host = m.theme().muted.Render(host)
	}
	return prefix + box + " " + preset.DisplayName + host
}
