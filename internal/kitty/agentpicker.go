package kitty

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

// agentPickerModel is a bubbletea model for the goto-agent fuzzy picker.
// It embeds the fuzzyfinder widget and holds the list of agents.
type agentPickerModel struct {
	allAgents  []Agent
	displayRef *[]Agent // pointer kept alive across Update copies for the RenderRow closure
	widget     fuzzyfinder.Model
	selectedID int
	helpMode   bool
}

func newAgentPickerModel(agents []Agent) agentPickerModel {
	if agents == nil {
		agents = []Agent{}
	}
	sortAgents(agents)
	displayRef := new([]Agent)
	*displayRef = agents

	m := agentPickerModel{
		allAgents:  agents,
		displayRef: displayRef,
	}
	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			display := *displayRef
			if len(display) == 0 {
				return fuzzyfinder.RowNormalStyle.Render("  No agent windows")
			}
			if i >= len(display) {
				return ""
			}
			return renderAgentPickerRow(display[i], selected)
		},
		Footer:    "type: filter  ↑/↓: move  enter: open  esc: cancel  ?: help",
		ItemCount: max(len(agents), 1),
	})
	return m
}

func renderAgentPickerRow(agent Agent, selected bool) string {
	cursor := "  "
	if selected {
		cursor = fuzzyfinder.HighlightStyle.Render("▶ ")
	}
	return fmt.Sprintf("%s%s %s: %s (%s)",
		cursor,
		statusGlyph(agent.Status),
		agent.Dir,
		titleStyle.Render(agent.Title),
		agentNameStyle.Render(agent.Name),
	)
}

func (m agentPickerModel) Init() tea.Cmd {
	return m.widget.Init()
}

func (m agentPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			display := *m.displayRef
			if len(display) > 0 {
				sel := m.widget.Selected()
				if sel < len(display) {
					m.selectedID = display[sel].ID
				}
			}
			return m, tea.Quit
		case "?":
			m.helpMode = !m.helpMode
			return m, nil
		}
	}

	prevQuery := m.widget.Query()
	var cmd tea.Cmd
	m.widget, cmd = m.widget.Update(msg)
	if m.widget.Query() != prevQuery {
		m.recomputeFilter()
	}
	return m, cmd
}

func (m *agentPickerModel) recomputeFilter() {
	query := m.widget.Query()
	var display []Agent
	if query == "" {
		display = m.allAgents
	} else {
		for _, a := range m.allAgents {
			searchable := a.Dir + " " + a.Title + " " + a.Name
			if _, ok := fuzzyfinder.MatchRanges(query, searchable); ok {
				display = append(display, a)
			}
		}
	}
	*m.displayRef = display
	m.widget.SetItemCount(max(len(display), 1))
	m.widget.SetSelected(0)
}

func (m agentPickerModel) View() tea.View {
	var content string
	if m.helpMode {
		content = m.renderHelp()
	} else {
		content = m.widget.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m agentPickerModel) renderHelp() string {
	lines := []string{
		"  Agent Picker — Help",
		"",
		"  type         filter agents by dir / title / agent",
		"  ↑ / k        move up",
		"  ↓ / j        move down",
		"  ctrl+k/j     same as ↑/↓",
		"  enter        focus selected agent window",
		"  esc          cancel without focusing",
		"  ?            toggle this help",
	}
	return strings.Join(lines, "\n")
}
