package kitty

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

var arcSpinnerFrames = []string{"◜", "◠", "◝", "◞", "◡", "◟"}

type agentDataTickMsg struct {
	agents []Agent
	err    error
}

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
	queryRef           *string  // current query, kept alive for the RenderRow closure (drives match highlighting)
	spinner            Spinner
	widget             fuzzyfinder.Model
	selected           *Agent // agent to focus after exit (set on enter; nil when cancelled)
	highlightedAgentID int    // agent ID of the currently highlighted row, for ID-stable selection
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
	queryRef := new(string)
	spinner := newSpinner(arcSpinnerFrames, 100*time.Millisecond)

	m := agentPickerModel{
		allAgents:  agents,
		displayRef: displayRef,
		queryRef:   queryRef,
		spinner:    spinner,
		deps:       d,
	}
	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			display := *displayRef
			if len(display) == 0 {
				return fuzzyfinder.RowNormalStyle.Render("No agent windows")
			}
			if i >= len(display) {
				return ""
			}
			return renderAgentPickerRow(display[i], *queryRef, spinner.Frame(), selected)
		},
		Footer:    "type: filter  ↑/↓: move  enter: open  esc: cancel  ?: help",
		ItemCount: max(len(agents), 1),
	})

	if len(agents) > 0 {
		m.highlightedAgentID = agents[0].ID
	}

	return m
}

// agentSearchable is the text matched against the query; the visible row's
// fields (dir, title, name) line up with this string's rune offsets, separated
// by a single space, so global match ranges map cleanly onto each field.
func agentSearchable(a Agent) string {
	return a.Dir + " " + a.Title + " " + a.Name
}

// renderAgentPickerRow renders one row as "{glyph} {dir}: {title} ({name})",
// highlighting the runes that fuzzy-matched query. Match positions are computed
// against agentSearchable and split back into per-field local ranges so each
// field is highlighted independently via the shared fuzzyfinder.Highlight.
func renderAgentPickerRow(agent Agent, query, spinnerFrame string, selected bool) string {
	ranges, _ := fuzzyfinder.MatchRanges(query, agentSearchable(agent))

	// Split global match ranges into per-field local rune indices. Layout:
	// dir [0,d)  space@d  title [d+1,d+1+t)  space@d+1+t  name [d+2+t, …).
	d := len([]rune(agent.Dir))
	t := len([]rune(agent.Title))
	titleStart := d + 1
	nameStart := d + 1 + t + 1
	var dirR, titleR, nameR []int
	for _, p := range ranges {
		switch {
		case p < d:
			dirR = append(dirR, p)
		case p >= titleStart && p < titleStart+t:
			titleR = append(titleR, p-titleStart)
		case p >= nameStart:
			nameR = append(nameR, p-nameStart)
		}
	}

	plain := lipgloss.NewStyle()
	sep := func(s string) string { return fuzzyfinder.Highlight(s, nil, plain, selected) }

	return agentStatusGlyph(agent, spinnerFrame, selected) +
		fuzzyfinder.Highlight(agent.Dir, dirR, plain, selected) +
		sep(": ") +
		fuzzyfinder.Highlight(agent.Title, titleR, titleStyle, selected) +
		sep(" (") +
		fuzzyfinder.Highlight(agent.Name, nameR, agentNameStyle, selected) +
		sep(")")
}

// agentStatusGlyph renders the leading status glyph plus its trailing space.
// Working uses the live spinner frame; the trailing space is part of the styled
// run so the selection background stays continuous on the active row.
func agentStatusGlyph(agent Agent, spinnerFrame string, selected bool) string {
	var style lipgloss.Style
	var g string
	switch agent.Status {
	case StatusWorking:
		// The arc spinner frame is one cell wide; the idle/waiting nerd glyphs are
		// wider, so pad with a space to keep all rows' text left-aligned.
		style, g = workingStatusStyle, spinnerFrame+" "
	case StatusWaiting:
		style, g = waitingStatusStyle, waitingGlyph
	default:
		style, g = idleStatusStyle, idleGlyph
	}
	if selected {
		style = style.Background(fuzzyfinder.SelectedBg)
	}
	return style.Render(g + " ")
}

func (m agentPickerModel) Init() tea.Cmd {
	return tea.Batch(
		m.widget.Init(),
		agentDataFetchCmd(m.deps),
		m.spinner.TickCmd(),
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

	case spinnerTickMsg:
		m.spinner.Advance()
		return m, m.spinner.TickCmd()

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
					agent := display[sel]
					m.selected = &agent
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
	*m.queryRef = query
	var display []Agent
	if query == "" {
		display = m.allAgents
	} else {
		for _, a := range m.allAgents {
			if _, ok := fuzzyfinder.MatchRanges(query, agentSearchable(a)); ok {
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
	*m.queryRef = query
	var display []Agent
	if query == "" {
		display = agents
	} else {
		for _, a := range agents {
			if _, ok := fuzzyfinder.MatchRanges(query, agentSearchable(a)); ok {
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
		"  ctrl+p/n     same as ↑/↓",
		"  enter        focus selected agent window",
		"  esc          cancel without focusing",
		"  ?            toggle this help",
	}
	return strings.Join(lines, "\n")
}
