package kitty

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

type agentDataTickMsg struct {
	agents []Agent
	err    error
}

type agentSpinnerTickMsg struct{}

type agentPreviewReadyMsg struct {
	text       string
	debounceID int
}

var previewPaneStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#585b70")).
	Faint(true).
	Padding(0, 1)

// agentPickerModel is a bubbletea model for the goto-agent fuzzy picker.
// It embeds the fuzzyfinder widget and holds the list of agents.
type agentPickerModel struct {
	allAgents          []Agent
	displayRef         *[]Agent // pointer kept alive across Update copies for the RenderRow closure
	spinnerFrameRef    *int     // pointer kept alive across Update copies for the RenderRow closure
	widget             fuzzyfinder.Model
	selectedID         int // agent ID to focus after exit (set on enter)
	highlightedAgentID int // agent ID of the currently highlighted row, for ID-stable selection
	helpMode           bool
	deps               Deps
	width              int
	height             int
	previewText        string
	previewDebounceID  int
}

func newAgentPickerModel(agents []Agent, d Deps) agentPickerModel {
	if agents == nil {
		agents = []Agent{}
	}
	sortAgents(agents)
	displayRef := new([]Agent)
	*displayRef = agents
	spinnerFrameRef := new(int)

	m := agentPickerModel{
		allAgents:       agents,
		displayRef:      displayRef,
		spinnerFrameRef: spinnerFrameRef,
		deps:            d,
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
			return renderAgentPickerRow(display[i], selected, *spinnerFrameRef)
		},
		Footer:    "type: filter  ↑/↓: move  enter: open  esc: cancel  ?: help",
		ItemCount: max(len(agents), 1),
	})

	if len(agents) > 0 {
		m.highlightedAgentID = agents[0].ID
	}

	return m
}

func renderAgentPickerRow(agent Agent, selected bool, spinnerFrame int) string {
	cursor := "  "
	if selected {
		cursor = fuzzyfinder.HighlightStyle.Render("▶ ")
	}
	glyph := statusGlyph(agent.Status)
	if agent.Status == StatusWorking {
		frame := spinnerFrames[spinnerFrame%len(spinnerFrames)]
		glyph = workingStatusStyle.Render(frame)
	}
	return fmt.Sprintf("%s%s %s: %s (%s)",
		cursor,
		glyph,
		agent.Dir,
		titleStyle.Render(agent.Title),
		agentNameStyle.Render(agent.Name),
	)
}

func (m agentPickerModel) Init() tea.Cmd {
	return tea.Batch(
		m.widget.Init(),
		agentDataFetchCmd(m.deps),
		agentSpinnerTickCmd(),
	)
}

// agentDataFetchCmd fetches agents immediately (no sleep) — used for the initial load.
func agentDataFetchCmd(d Deps) tea.Cmd {
	return func() tea.Msg {
		agents, err := ListAgents(d)
		return agentDataTickMsg{agents: agents, err: err}
	}
}

// agentDataTickCmd sleeps ~1s then fetches agents — used for periodic refresh.
func agentDataTickCmd(d Deps) tea.Cmd {
	return func() tea.Msg {
		<-time.After(time.Second)
		agents, err := ListAgents(d)
		return agentDataTickMsg{agents: agents, err: err}
	}
}

func agentSpinnerTickCmd() tea.Cmd {
	return func() tea.Msg {
		<-time.After(100 * time.Millisecond)
		return agentSpinnerTickMsg{}
	}
}

// agentPreviewFetchCmd debounces preview fetching. sleepMS=0 for immediate fetch.
func agentPreviewFetchCmd(agentID int, d Deps, debounceID int, sleepMS int) tea.Cmd {
	return func() tea.Msg {
		if sleepMS > 0 {
			<-time.After(time.Duration(sleepMS) * time.Millisecond)
		}
		text, _ := RenderAgentPreview(strconv.Itoa(agentID), d)
		return agentPreviewReadyMsg{text: text, debounceID: debounceID}
	}
}

func (m agentPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.widget.SetSize(msg.Width/2, msg.Height)
		return m, nil

	case agentDataTickMsg:
		var previewCmd tea.Cmd
		if msg.err == nil {
			previewCmd = m.applyDataRefresh(msg.agents)
		}
		return m, tea.Batch(previewCmd, agentDataTickCmd(m.deps))

	case agentSpinnerTickMsg:
		*m.spinnerFrameRef = (*m.spinnerFrameRef + 1) % len(spinnerFrames)
		return m, agentSpinnerTickCmd()

	case agentPreviewReadyMsg:
		if msg.debounceID == m.previewDebounceID {
			m.previewText = msg.text
		}
		return m, nil
	}

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
	prevHighlightedID := m.highlightedAgentID
	var cmd tea.Cmd
	m.widget, cmd = m.widget.Update(msg)
	if m.widget.Query() != prevQuery {
		m.recomputeFilter()
	}

	// Track highlighted agent ID and debounce preview on selection change.
	display := *m.displayRef
	sel := m.widget.Selected()
	if sel < len(display) {
		newID := display[sel].ID
		if newID != prevHighlightedID {
			m.highlightedAgentID = newID
			m.previewDebounceID++
			previewCmd := agentPreviewFetchCmd(newID, m.deps, m.previewDebounceID, 80)
			cmd = tea.Batch(cmd, previewCmd)
		}
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

	if len(display) > 0 {
		m.highlightedAgentID = display[0].ID
	} else {
		m.highlightedAgentID = 0
	}
}

// applyDataRefresh updates allAgents and displayRef with fresh data, preserving
// the highlighted agent by ID. Returns a Cmd to refresh the preview if needed.
func (m *agentPickerModel) applyDataRefresh(agents []Agent) tea.Cmd {
	sortAgents(agents)
	m.allAgents = agents

	query := m.widget.Query()
	var display []Agent
	if query == "" {
		display = agents
	} else {
		for _, a := range agents {
			searchable := a.Dir + " " + a.Title + " " + a.Name
			if _, ok := fuzzyfinder.MatchRanges(query, searchable); ok {
				display = append(display, a)
			}
		}
	}
	*m.displayRef = display

	// ID-stable selection: find the previously highlighted agent in the new display.
	newIdx := 0
	if m.highlightedAgentID != 0 {
		for i, a := range display {
			if a.ID == m.highlightedAgentID {
				newIdx = i
				break
			}
		}
	}

	m.widget.SetItemCount(max(len(display), 1))
	m.widget.SetSelected(newIdx)

	if len(display) > 0 {
		m.highlightedAgentID = display[newIdx].ID
		m.previewDebounceID++
		return agentPreviewFetchCmd(display[newIdx].ID, m.deps, m.previewDebounceID, 0)
	}
	m.highlightedAgentID = 0
	return nil
}

func (m agentPickerModel) View() tea.View {
	var content string
	if m.helpMode {
		content = m.renderHelp()
	} else if m.width > 0 {
		leftWidth := m.width / 2
		rightWidth := m.width - leftWidth
		m.widget.SetSize(leftWidth, m.height)
		widgetStr := m.widget.View()
		previewStr := m.renderPreview(rightWidth, m.height)
		content = lipgloss.JoinHorizontal(lipgloss.Top, widgetStr, previewStr)
	} else {
		content = m.widget.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m agentPickerModel) renderPreview(width, height int) string {
	text := m.previewText
	if text == "" {
		display := *m.displayRef
		if len(display) == 0 {
			text = "(no agents)"
		} else {
			text = "(loading...)"
		}
	}

	// previewPaneStyle has a 1-char border + 1-char padding on each side.
	// Height(h)/Width(w) set the outer dimensions; inner = h−2 / w−4.
	// Truncate wide lines (prevents lipgloss line-wrapping which inflates height),
	// strip trailing empty lines, then take the last innerHeight lines.
	innerHeight := max(height-2, 1)
	innerWidth := max(width-4, 1)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if ansi.StringWidth(line) > innerWidth {
			lines[i] = ansi.Truncate(line, innerWidth, "")
		}
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > innerHeight {
		lines = lines[len(lines)-innerHeight:]
	}

	return previewPaneStyle.Width(width).Height(height).Render(strings.Join(lines, "\n"))
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
