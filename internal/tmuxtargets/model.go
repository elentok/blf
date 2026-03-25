package tmuxtargets

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/platform"
)

var (
	baseStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	targetStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("11"))
)

type model struct {
	lines        []string
	targets      []target
	selected     int
	pendingG     bool
	notify       func(string)
	copyText     func(string) error
	openURL      func(string) error
	shouldQuit   bool
	quitWithErr  error
	lastFeedback string
}

func newModel(lines []string, targets []target, notify func(string)) model {
	return model{
		lines:    lines,
		targets:  targets,
		selected: 0,
		notify:   notify,
		copyText: platform.CopyText,
		openURL:  platform.OpenURL,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch k := msg.(type) {
	case tea.KeyMsg:
		key := k.String()
		switch key {
		case "q", "esc", "ctrl+c":
			m.shouldQuit = true
			return m, tea.Quit
		case "j", "down", "l", "right":
			m.pendingG = false
			m.move(1)
			return m, nil
		case "k", "up", "h", "left":
			m.pendingG = false
			m.move(-1)
			return m, nil
		case "g":
			if m.pendingG {
				m.selected = 0
				m.pendingG = false
			} else {
				m.pendingG = true
			}
			return m, nil
		case "G":
			m.pendingG = false
			if len(m.targets) > 0 {
				m.selected = len(m.targets) - 1
			}
			return m, nil
		case "y", "c":
			m.pendingG = false
			if len(m.targets) == 0 {
				m.notify("blf tmux-targets: no targets to copy")
				return m, nil
			}
			if err := m.copyText(m.current().text); err != nil {
				m.notify("blf tmux-targets: failed to copy target")
				return m, nil
			}
			m.shouldQuit = true
			return m, tea.Quit
		case "enter", "o":
			m.pendingG = false
			if len(m.targets) == 0 {
				m.notify("blf tmux-targets: no targets to open")
				return m, nil
			}
			t := m.current()
			if !t.openable {
				m.notify("blf tmux-targets: selected target is not openable")
				return m, nil
			}
			if err := m.openURL(t.openTarget); err != nil {
				m.notify("blf tmux-targets: failed to open target")
				return m, nil
			}
			m.shouldQuit = true
			return m, tea.Quit
		default:
			m.pendingG = false
			return m, nil
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	if len(m.lines) == 0 {
		v := tea.NewView(baseStyle.Render(""))
		v.AltScreen = true
		return v
	}

	spansByLine := make(map[int][]target, len(m.targets))
	if len(m.targets) > 0 {
		for _, t := range m.targets {
			spansByLine[t.line] = append(spansByLine[t.line], t)
		}
		for line := range spansByLine {
			sort.Slice(spansByLine[line], func(i, j int) bool {
				return spansByLine[line][i].start < spansByLine[line][j].start
			})
		}
	}

	out := strings.Builder{}
	for i, line := range m.lines {
		lineTargets := spansByLine[i]
		if len(lineTargets) > 0 {
			selected := m.current()
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
					if t.line == selected.line && t.start == selected.start && t.end == selected.end {
						out.WriteString(selectedStyle.Render(line[start:end]))
					} else {
						out.WriteString(targetStyle.Render(line[start:end]))
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
	v := tea.NewView(out.String())
	v.AltScreen = true
	return v
}

func (m *model) move(delta int) {
	if len(m.targets) == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = len(m.targets) - 1
	}
	if m.selected >= len(m.targets) {
		m.selected = 0
	}
}

func (m model) current() target {
	if len(m.targets) == 0 {
		return target{}
	}
	idx := m.selected
	if idx < 0 || idx >= len(m.targets) {
		idx = 0
	}
	return m.targets[idx]
}

func runPopupUI(lines []string, targets []target, notify func(string)) error {
	m := newModel(lines, targets, notify)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run popup ui: %w", err)
	}
	return nil
}
