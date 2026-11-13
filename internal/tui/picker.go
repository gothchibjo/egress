package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gothchibjo/egress/internal/domain"
)

// pickerState is the target selection screen: presets grouped by category,
// custom targets, a filter and the selected set.
type pickerState struct {
	cursor       int
	filter       string
	filtering    bool
	selected     map[string]bool
	custom       []string
	customInput  string
	customActive bool
}

// pickerEntry is one navigable row: a preset or a custom target.
type pickerEntry struct {
	preset domain.Preset
	custom string
}

func (e pickerEntry) isCustom() bool {
	return e.custom != ""
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
	case "g":
		m.picker.cursor = 0
	case "G":
		m.picker.cursor = len(m.pickerEntries()) - 1
	case " ":
		m = m.togglePickerRow()
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
	m.opts.startSelection(presets, custom)
	return m, nil
}

// addCustomTarget stores the typed target. Validation and resolution belong to
// REQ-003; the picker only rejects empty and duplicate input.
func (m Model) addCustomTarget() (tea.Model, tea.Cmd) {
	target := strings.TrimSpace(m.picker.customInput)
	m.picker.customActive = false
	m.picker.customInput = ""
	if target == "" {
		return m.setNotice("custom target is empty")
	}
	for _, existing := range m.picker.custom {
		if existing == target {
			return m.setNotice("target is already selected")
		}
	}
	m.picker.custom = append(m.picker.custom, target)
	m.picker.cursor = len(m.pickerEntries()) - 1
	return m, nil
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
// category, then custom targets. The filter matches presets only.
func (m Model) pickerEntries() []pickerEntry {
	filter := strings.ToLower(strings.TrimSpace(m.picker.filter))
	entries := make([]pickerEntry, 0, len(domain.PresetCatalog())+len(m.picker.custom))

	for _, preset := range domain.PresetCatalog() {
		if filter != "" && !presetMatches(preset, filter) {
			continue
		}
		entries = append(entries, pickerEntry{preset: preset})
	}
	for _, custom := range m.picker.custom {
		entries = append(entries, pickerEntry{custom: custom})
	}
	return entries
}

func presetMatches(preset domain.Preset, filter string) bool {
	haystack := strings.ToLower(preset.DisplayName + " " + preset.Host + " " + preset.Category)
	return strings.Contains(haystack, filter)
}

// pickerSelection returns the selected presets and custom targets.
func (m Model) pickerSelection() ([]domain.Preset, []string) {
	presets := make([]domain.Preset, 0, len(m.picker.selected))
	for _, preset := range domain.PresetCatalog() {
		if m.picker.selected[preset.ID] {
			presets = append(presets, preset)
		}
	}
	custom := append([]string(nil), m.picker.custom...)
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

	bodyHeight := m.height - len(header) - len(footer)
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := block(m.pickerLines(bodyHeight), m.width, bodyHeight)
	lines := append(header, strings.Split(body, "\n")...)
	lines = append(lines, footer...)
	return strings.Join(lines, "\n")
}

// pickerLines renders the grouped rows with checkboxes. The list scrolls so the
// cursor row is always visible.
func (m Model) pickerLines(height int) []string {
	entries := m.pickerEntries()
	if len(entries) == 0 {
		return []string{"", " no targets match the filter"}
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
			lines = append(lines, prefix+"[x] "+entry.custom)
			continue
		}

		box := "[ ]"
		if m.picker.selected[entry.preset.ID] {
			box = "[x]"
		}
		lines = append(lines, fmt.Sprintf("%s%s %-17s %s", prefix, box, entry.preset.DisplayName, entry.preset.Host))
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
