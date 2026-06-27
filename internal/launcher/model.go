package launcher

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const maxResults = 200

// ModelConfig holds injectable dependencies for the launcher model.
type ModelConfig struct {
	Providers    []Provider
	ConfigErr    error
	CopyText     func(string) error
	HideTerminal func() error
	UseNerdFont  bool
}

// Model is the bubbletea model for the launcher TUI.
type Model struct {
	cfg      ModelConfig
	input    textinput.Model
	results  []Result
	selected int
	offset   int // viewport scroll offset into results
	width    int
	height   int
	helpMode bool
	status   string // transient status / error message
}

// NewModel creates a launcher Model ready to run.
func NewModel(cfg ModelConfig) Model {
	ti := textinput.New()
	ti.Placeholder = "Type to search or calculate…"
	ti.Prompt = inputPromptStyle.Render("> ")
	_ = ti.Focus()

	return Model{
		cfg:   cfg,
		input: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if m.helpMode {
			m.helpMode = false
			return m, nil
		}
		switch key {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Reset and hide the quick terminal (ADR 0002)
			m.input.Reset()
			m.results = nil
			m.selected = 0
			m.offset = 0
			m.status = ""
			if m.cfg.HideTerminal != nil {
				_ = m.cfg.HideTerminal()
			}
			return m, nil

		case "up":
			if m.selected > 0 {
				m.selected--
				if m.selected < m.offset {
					m.offset = m.selected
				}
			}
			return m, nil

		case "down":
			if m.selected < len(m.results)-1 {
				m.selected++
				visibleRows := m.visibleResultRows()
				if m.selected >= m.offset+visibleRows {
					m.offset = m.selected - visibleRows + 1
				}
			}
			return m, nil

		case "enter":
			if len(m.results) == 0 {
				return m, nil
			}
			result := m.results[m.selected]
			if err := m.act(result); err != nil {
				m.status = err.Error()
				return m, nil
			}
			// Success: record input, reset, hide
			m.input.Reset()
			m.results = nil
			m.selected = 0
			m.offset = 0
			m.status = ""
			if m.cfg.HideTerminal != nil {
				_ = m.cfg.HideTerminal()
			}
			return m, nil

		case "?":
			m.helpMode = true
			return m, nil

		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.recomputeResults()
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) recomputeResults() {
	query := m.input.Value()
	var all []Result
	for _, p := range m.cfg.Providers {
		all = append(all, p.Query(query)...)
	}
	ranked := Rank(all)
	if len(ranked) > maxResults {
		ranked = ranked[:maxResults]
	}
	m.results = ranked
	if m.selected >= len(m.results) {
		m.selected = 0
		m.offset = 0
	}
}

func (m *Model) act(r Result) error {
	switch r.Action.Type {
	case ActionCopy:
		if m.cfg.CopyText == nil {
			return fmt.Errorf("copy not available")
		}
		return m.cfg.CopyText(r.Action.Target)
	default:
		return fmt.Errorf("action not yet implemented")
	}
}

// visibleResultRows returns how many result rows fit in the current terminal.
func (m Model) visibleResultRows() int {
	// Layout: border top (1) + input (1) + separator (1) + footer (1) + border bottom (1) = 5 overhead
	// Plus config-error row if present
	overhead := 5
	if m.cfg.ConfigErr != nil {
		overhead++
	}
	n := m.height - overhead
	if n < 1 {
		n = 1
	}
	return n
}

func (m Model) View() tea.View {
	if m.helpMode {
		content := m.renderHelp()
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	inner := m.renderInner()
	// Wrap in outer rounded border
	w := m.width - 2 // account for border
	if w < 10 {
		w = 10
	}
	content := borderStyle.Width(w).Render(inner)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) renderInner() string {
	var sb strings.Builder

	// Config error notice (non-blocking)
	if m.cfg.ConfigErr != nil {
		sb.WriteString(errorStyle.Render("config: "+m.cfg.ConfigErr.Error()) + "\n")
	}

	// Input line
	sb.WriteString(m.input.View() + "\n")

	// Separator
	w := m.width - 4 // border (2) + padding (2)
	if w < 1 {
		w = 1
	}
	sb.WriteString(separatorStyle.Render(strings.Repeat("─", w)) + "\n")

	// Results viewport
	visibleRows := m.visibleResultRows()
	end := m.offset + visibleRows
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := m.offset; i < end; i++ {
		sb.WriteString(m.renderResult(m.results[i], i == m.selected) + "\n")
	}

	// Status / help footer
	footer := m.status
	if footer == "" {
		footer = "↑↓ select  enter: act  esc: dismiss  ?: help"
	}
	sb.WriteString(helpBarStyle.Width(w).Render(footer))

	return sb.String()
}

func (m Model) renderResult(r Result, selected bool) string {
	icon := Icon(r.Icon, m.cfg.UseNerdFont)
	w := m.width - 4
	if w < 10 {
		w = 10
	}

	// Title with fuzzy-match highlights
	title := m.renderTitle(r, selected)

	// Subtitle (right-padded)
	subtitle := ""
	if r.Subtitle != "" {
		subtitle = " " + subtitleStyle.Render(r.Subtitle)
	}

	// Source hint (right-aligned)
	source := ""
	if r.Source != "" {
		source = sourceStyle.Render("[" + r.Source + "]")
	}

	line := icon + title + subtitle
	lineWidth := lipgloss.Width(line)
	sourceWidth := lipgloss.Width(source)
	if source != "" && lineWidth+sourceWidth+2 <= w {
		padding := w - lineWidth - sourceWidth
		if padding < 1 {
			padding = 1
		}
		line += strings.Repeat(" ", padding) + source
	}

	if selected {
		return resultSelectedStyle.Width(w).Render(line)
	}
	return resultNormalStyle.Width(w).Render(line)
}

// renderTitle renders the result title, highlighting fuzzy match positions.
func (m Model) renderTitle(r Result, selected bool) string {
	if len(r.MatchRanges) == 0 {
		return r.Title
	}
	// Build a set of highlight positions
	pos := make(map[int]bool, len(r.MatchRanges))
	for _, p := range r.MatchRanges {
		pos[p] = true
	}
	runes := []rune(r.Title)
	var sb strings.Builder
	for i, ch := range runes {
		s := string(ch)
		if pos[i] {
			if selected {
				sb.WriteString(resultSelectedHighlightStyle.Render(s))
			} else {
				sb.WriteString(resultHighlightStyle.Render(s))
			}
		} else {
			sb.WriteString(s)
		}
	}
	return sb.String()
}

func (m Model) renderHelp() string {
	lines := []string{
		helpTitleStyle.Render("blf launcher — help"),
		"",
		"  ↑ / ↓       select result",
		"  enter        act on selected result (copy / launch / run)",
		"  esc          dismiss launcher and clear input",
		"  ?            toggle this help",
		"  ctrl+c       quit launcher process",
		"",
		helpHintStyle.Render("  press any key to close help"),
	}
	return strings.Join(lines, "\n")
}
