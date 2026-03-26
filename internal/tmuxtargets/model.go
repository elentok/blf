package tmuxtargets

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/elentok/blf/internal/platform"
)

type model struct {
	lines        []string
	targets      []target
	selected     int
	pendingG     bool
	searchMode   bool
	filterLocked bool
	query        string
	filteredIdx  []int
	notify       func(string)
	copyText     func(string) error
	openURL      func(string) error
}

func newModel(lines []string, targets []target, notify func(string)) model {
	m := model{
		lines:    lines,
		targets:  targets,
		selected: -1,
		notify:   notify,
		copyText: platform.CopyText,
		openURL:  platform.OpenURL,
	}
	if len(targets) > 0 {
		m.selected = 0
	}
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch k := msg.(type) {
	case tea.KeyMsg:
		key := k.String()

		if m.searchMode {
			return m.updateSearchMode(k, key)
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.filterLocked || m.query != "" {
				m.clearSearch()
				return m, nil
			}
			return m, tea.Quit
		case "/":
			m.pendingG = false
			m.searchMode = true
			m.recomputeFilter()
			return m, nil
		case "j", "down":
			m.pendingG = false
			m.moveVertical(1)
			return m, nil
		case "k", "up":
			m.pendingG = false
			m.moveVertical(-1)
			return m, nil
		case "l", "right":
			m.pendingG = false
			m.moveHorizontal(1)
			return m, nil
		case "h", "left":
			m.pendingG = false
			m.moveHorizontal(-1)
			return m, nil
		case "g":
			if m.pendingG {
				m.moveToFirst()
				m.pendingG = false
			} else {
				m.pendingG = true
			}
			return m, nil
		case "G":
			m.pendingG = false
			m.moveToLast()
			return m, nil
		case "y", "c":
			m.pendingG = false
			t, ok := m.selectedTarget()
			if !ok {
				m.notify("no targets to copy")
				return m, nil
			}
			if err := m.copyText(t.text); err != nil {
				m.notify("failed to copy target")
				return m, nil
			}
			return m, tea.Quit
		case "enter", "o":
			m.pendingG = false
			t, ok := m.selectedTarget()
			if !ok {
				m.notify("no targets to open")
				return m, nil
			}
			if !t.openable {
				m.notify("selected target is not openable")
				return m, nil
			}
			if err := m.openURL(t.openTarget); err != nil {
				m.notify("failed to open target")
				return m, nil
			}
			return m, tea.Quit
		default:
			m.pendingG = false
			return m, nil
		}
	}

	return m, nil
}

func (m model) updateSearchMode(k tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.clearSearch()
		return m, nil
	case "enter":
		m.searchMode = false
		m.filterLocked = true
		m.recomputeFilter()
		return m, nil
	case "backspace", "ctrl+h":
		m.query = trimLastRune(m.query)
		m.recomputeFilter()
		return m, nil
	case "q", "ctrl+c":
		return m, tea.Quit
	default:
		if text := k.Key().Text; text != "" {
			m.query += text
			m.recomputeFilter()
		}
		return m, nil
	}
}

func (m model) View() tea.View {
	if len(m.lines) == 0 {
		v := tea.NewView(baseStyle.Render(""))
		v.AltScreen = true
		return v
	}

	spansByLine := make(map[int][]target, len(m.targets))
	targetIndexBySpan := make(map[string]int, len(m.targets))
	if len(m.targets) > 0 {
		for idx, t := range m.targets {
			spansByLine[t.line] = append(spansByLine[t.line], t)
			targetIndexBySpan[spanKey(t)] = idx
		}
		for line := range spansByLine {
			sort.Slice(spansByLine[line], func(i, j int) bool {
				return spansByLine[line][i].start < spansByLine[line][j].start
			})
		}
	}

	out := strings.Builder{}
	selectedIdx, hasSelected := m.currentSelectedIndex()
	filtered := m.isFilteringByQuery()
	searchActive := m.searchMode || m.filterLocked
	activeSet := make(map[int]struct{})
	if filtered {
		for _, idx := range m.activeIndexes() {
			activeSet[idx] = struct{}{}
		}
	}

	for i, line := range m.lines {
		lineTargets := spansByLine[i]
		if len(lineTargets) > 0 {
			cursor := 0
			for _, t := range lineTargets {
				start := t.start
				end := t.end
				if start > len(line) {
					start = len(line)
				}
				if end > len(line) {
					end = len(line)
				}
				if start < cursor {
					start = cursor
				}
				if start > cursor {
					out.WriteString(baseStyle.Render(line[cursor:start]))
				}
				if end > start {
					idx := targetIndexBySpan[spanKey(t)]
					if hasSelected && idx == selectedIdx {
						if searchActive {
							out.WriteString(searchSelectedStyle.Render(line[start:end]))
						} else {
							out.WriteString(selectedStyle.Render(line[start:end]))
						}
					} else if filtered {
						if _, ok := activeSet[idx]; ok {
							if searchActive {
								out.WriteString(searchTargetStyle.Render(line[start:end]))
							} else {
								out.WriteString(targetStyle.Render(line[start:end]))
							}
						} else {
							out.WriteString(baseStyle.Render(line[start:end]))
						}
					} else {
						if searchActive {
							out.WriteString(searchTargetStyle.Render(line[start:end]))
						} else {
							out.WriteString(targetStyle.Render(line[start:end]))
						}
					}
				}
				cursor = end
			}
			if cursor < len(line) {
				out.WriteString(baseStyle.Render(line[cursor:]))
			}
		} else {
			out.WriteString(baseStyle.Render(line))
		}
		if i < len(m.lines)-1 {
			out.WriteByte('\n')
		}
	}
	rendered := strings.Split(out.String(), "\n")
	if searchActive {
		rendered = m.overlaySearchBox(rendered)
	}
	v := tea.NewView(strings.Join(rendered, "\n"))
	v.AltScreen = true
	return v
}

func (m model) overlaySearchBox(lines []string) []string {
	if len(lines) < 3 {
		return lines
	}

	matchCount := len(m.activeIndexes())
	label := "SEARCH"
	if m.filterLocked {
		label = "FILTERED"
	}
	prefix := "Search: "
	if label == "FILTERED" {
		prefix = "Filtered: "
	}
	content := fmt.Sprintf("%s%s (%d/%d)", prefix, m.query, matchCount, len(m.targets))
	content = trimToWidth(content, 38)
	box := searchBoxStyle.Render(content)
	boxLines := strings.Split(box, "\n")
	if len(boxLines) != 3 {
		return lines
	}

	width := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > width {
			width = w
		}
	}
	boxWidth := lipgloss.Width(boxLines[0])
	left := 0
	if width > boxWidth {
		left = (width - boxWidth) / 2
	}

	y := len(lines) - 3
	for y >= 0 && m.rowsContainTargets(y, y+2) {
		y--
	}
	if y < 0 {
		y = 0
	}

	out := append([]string(nil), lines...)
	for i := 0; i < 3; i++ {
		row := strings.Repeat(" ", left) + boxLines[i]
		out[y+i] = row
	}
	return out
}

func (m model) rowsContainTargets(start, end int) bool {
	for _, t := range m.targets {
		if t.line >= start && t.line <= end {
			return true
		}
	}
	return false
}

func (m *model) moveVertical(delta int) {
	indexes := m.activeIndexes()
	if len(indexes) == 0 {
		m.selected = -1
		return
	}
	cur, ok := m.selectedTarget()
	if !ok {
		m.selected = indexes[0]
		return
	}

	bestIdx := -1
	bestLineDistance := 0
	bestColDistance := 0
	for _, idx := range indexes {
		cand := m.targets[idx]
		lineDistance := cand.line - cur.line
		if delta < 0 {
			if lineDistance >= 0 {
				continue
			}
			lineDistance = -lineDistance
		} else {
			if lineDistance <= 0 {
				continue
			}
		}

		colDistance := cand.start - cur.start
		if colDistance < 0 {
			colDistance = -colDistance
		}

		if bestIdx == -1 || lineDistance < bestLineDistance || (lineDistance == bestLineDistance && colDistance < bestColDistance) {
			bestIdx = idx
			bestLineDistance = lineDistance
			bestColDistance = colDistance
		}
	}
	if bestIdx != -1 {
		m.selected = bestIdx
	}
}

func (m *model) moveHorizontal(delta int) {
	indexes := m.activeIndexes()
	if len(indexes) == 0 {
		m.selected = -1
		return
	}
	cur, ok := m.selectedTarget()
	if !ok {
		m.selected = indexes[0]
		return
	}

	bestIdx := -1
	bestDistance := 0
	for _, idx := range indexes {
		cand := m.targets[idx]
		if cand.line != cur.line {
			continue
		}
		distance := cand.start - cur.start
		if delta < 0 {
			if distance >= 0 {
				continue
			}
			distance = -distance
		} else {
			if distance <= 0 {
				continue
			}
		}
		if bestIdx == -1 || distance < bestDistance {
			bestIdx = idx
			bestDistance = distance
		}
	}
	if bestIdx != -1 {
		m.selected = bestIdx
	}
}

func (m *model) moveToFirst() {
	indexes := m.activeIndexes()
	if len(indexes) == 0 {
		m.selected = -1
		return
	}
	m.selected = indexes[0]
}

func (m *model) moveToLast() {
	indexes := m.activeIndexes()
	if len(indexes) == 0 {
		m.selected = -1
		return
	}
	m.selected = indexes[len(indexes)-1]
}

func (m *model) recomputeFilter() {
	if strings.TrimSpace(m.query) == "" {
		m.filteredIdx = nil
		if len(m.targets) == 0 {
			m.selected = -1
			return
		}
		if m.selected < 0 || m.selected >= len(m.targets) {
			m.selected = 0
		}
		return
	}

	candidates := make([]string, len(m.targets))
	for i, t := range m.targets {
		candidates[i] = t.text
	}
	matches := fuzzy.Find(m.query, candidates)
	m.filteredIdx = make([]int, 0, len(matches))
	for _, match := range matches {
		m.filteredIdx = append(m.filteredIdx, match.Index)
	}
	if len(m.filteredIdx) == 0 {
		m.selected = -1
		return
	}
	m.selected = m.filteredIdx[0]
}

func (m *model) clearSearch() {
	m.searchMode = false
	m.filterLocked = false
	m.query = ""
	m.filteredIdx = nil
	if len(m.targets) == 0 {
		m.selected = -1
		return
	}
	m.selected = 0
}

func (m model) isFilteringByQuery() bool {
	return (m.searchMode || m.filterLocked) && strings.TrimSpace(m.query) != ""
}

func (m model) activeIndexes() []int {
	if len(m.targets) == 0 {
		return nil
	}
	if m.isFilteringByQuery() {
		return append([]int(nil), m.filteredIdx...)
	}
	idx := make([]int, len(m.targets))
	for i := range m.targets {
		idx[i] = i
	}
	return idx
}

func (m model) currentSelectedIndex() (int, bool) {
	if m.selected < 0 || m.selected >= len(m.targets) {
		return -1, false
	}
	indexes := m.activeIndexes()
	for _, idx := range indexes {
		if idx == m.selected {
			return idx, true
		}
	}
	return -1, false
}

func (m model) selectedTarget() (target, bool) {
	idx, ok := m.currentSelectedIndex()
	if !ok {
		return target{}, false
	}
	return m.targets[idx], true
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

func trimToWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func spanKey(t target) string {
	return fmt.Sprintf("%d:%d:%d", t.line, t.start, t.end)
}

func runPopupUI(lines []string, targets []target, notify func(string)) error {
	m := newModel(lines, targets, notify)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run popup ui: %w", err)
	}
	return nil
}
