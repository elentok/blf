package tmuxtargets

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/platform"
)

const (
	baseColorPrefix     = "\x1b[38;5;245m"
	selectedColorPrefix = "\x1b[30;43m"
	resetColor          = "\x1b[0m"
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
		case "y":
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
		v := tea.NewView(baseColorPrefix + "" + resetColor)
		v.AltScreen = true
		return v
	}

	spansByLine := make(map[int]target, len(m.targets))
	if len(m.targets) > 0 {
		spansByLine[m.current().line] = m.current()
	}

	out := strings.Builder{}
	for i, line := range m.lines {
		if t, ok := spansByLine[i]; ok {
			if t.start > len(line) {
				t.start = len(line)
			}
			if t.end > len(line) {
				t.end = len(line)
			}
			left := line[:t.start]
			mid := line[t.start:t.end]
			right := line[t.end:]
			out.WriteString(baseColorPrefix)
			out.WriteString(left)
			out.WriteString(resetColor)
			out.WriteString(selectedColorPrefix)
			out.WriteString(mid)
			out.WriteString(resetColor)
			out.WriteString(baseColorPrefix)
			out.WriteString(right)
			out.WriteString(resetColor)
		} else {
			out.WriteString(baseColorPrefix)
			out.WriteString(line)
			out.WriteString(resetColor)
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
