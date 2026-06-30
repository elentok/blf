package claudehistory

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/claude"
	"github.com/elentok/blf/internal/fuzzyfinder"
)

type page int

const (
	pageProjects page = iota
	pageConversations
)

type projectsLoadedMsg struct {
	projects []claude.Project
	err      error
}

type conversationsLoadedMsg struct {
	conversations []claude.Conversation
	err           error
}

// Model is the root bubbletea model for the claude history TUI.
type Model struct {
	page        page
	allProjects []claude.Project
	displayRef  *[]claude.Project
	queryRef    *string
	widget      fuzzyfinder.Model
	projectsErr error

	allConversations     []claude.Conversation
	convDisplayRef       *[]claude.Conversation
	convQueryRef         *string
	convWidget           fuzzyfinder.Model
	conversationsErr     error
	conversationsLoading bool

	width  int
	height int
}

// New creates a new history Model. It returns a model that starts on the
// projects page and immediately triggers an async load of projects.
func New(projectsRoot string) Model {
	displayRef := new([]claude.Project)
	*displayRef = nil
	queryRef := new(string)

	convDisplayRef := new([]claude.Conversation)
	*convDisplayRef = nil
	convQueryRef := new(string)

	m := Model{
		page:           pageProjects,
		displayRef:     displayRef,
		queryRef:       queryRef,
		convDisplayRef: convDisplayRef,
		convQueryRef:   convQueryRef,
	}
	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			display := *displayRef
			if len(display) == 0 {
				return fuzzyfinder.RowNormalStyle.Render("No projects found")
			}
			if i >= len(display) {
				return ""
			}
			return renderProjectRow(display[i], *queryRef, selected)
		},
		Footer:    "type: filter  ↑/↓: move  enter: open  esc: quit",
		ItemCount: 1,
	})
	m.convWidget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			display := *convDisplayRef
			if len(display) == 0 {
				return fuzzyfinder.RowNormalStyle.Render("No conversations found")
			}
			if i >= len(display) {
				return ""
			}
			return renderConversationRow(display[i], *convQueryRef, selected)
		},
		Footer:    "type: filter  ↑/↓: move  enter: open  esc: back",
		ItemCount: 1,
	})

	return m
}

// Init loads projects asynchronously.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.widget.Init(),
		m.convWidget.Init(),
		loadProjectsCmd(""),
	)
}

func loadProjectsCmd(root string) tea.Cmd {
	return func() tea.Msg {
		projects, err := claude.ListProjects(root)
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func loadConversationsCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		convs, err := claude.ListConversations(dir)
		return conversationsLoadedMsg{conversations: convs, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.widget.SetSize(msg.Width, msg.Height)
		m.convWidget.SetSize(msg.Width, msg.Height)
		return m, nil

	case projectsLoadedMsg:
		if msg.err != nil {
			m.projectsErr = msg.err
			return m, nil
		}
		m.allProjects = msg.projects
		*m.displayRef = msg.projects
		m.widget.SetItemCount(max(len(msg.projects), 1))
		return m, nil

	case conversationsLoadedMsg:
		m.conversationsLoading = false
		if msg.err != nil {
			m.conversationsErr = msg.err
			return m, nil
		}
		m.allConversations = msg.conversations
		*m.convDisplayRef = msg.conversations
		m.convWidget.SetItemCount(max(len(msg.conversations), 1))
		return m, nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch m.page {
		case pageProjects:
			switch key.String() {
			case "esc", "ctrl+c":
				return m, tea.Quit
			case "enter":
				return m.enterConversations()
			}
		case pageConversations:
			switch key.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				return m.exitConversations()
			case "enter":
				return m, nil // stub — next slice wires export
			}
		}
	}

	switch m.page {
	case pageProjects:
		prevQuery := m.widget.Query()
		var cmd tea.Cmd
		m.widget, cmd = m.widget.Update(msg)
		if m.widget.Query() != prevQuery {
			*m.queryRef = m.widget.Query()
			m.recomputeFilter()
		}
		return m, cmd

	case pageConversations:
		prevQuery := m.convWidget.Query()
		var cmd tea.Cmd
		m.convWidget, cmd = m.convWidget.Update(msg)
		if m.convWidget.Query() != prevQuery {
			*m.convQueryRef = m.convWidget.Query()
			m.recomputeConvFilter()
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) enterConversations() (tea.Model, tea.Cmd) {
	display := *m.displayRef
	sel := m.widget.Selected()
	if len(display) == 0 || sel >= len(display) {
		return m, nil
	}
	p := display[sel]
	m.page = pageConversations
	m.allConversations = nil
	*m.convDisplayRef = nil
	*m.convQueryRef = ""
	m.convWidget.SetItemCount(1)
	m.convWidget.SetSelected(0)
	m.convWidget.SetQuery("")
	m.conversationsErr = nil
	m.conversationsLoading = true
	return m, loadConversationsCmd(p.Dir)
}

func (m Model) exitConversations() (tea.Model, tea.Cmd) {
	m.page = pageProjects
	return m, nil
}

func (m *Model) recomputeFilter() {
	query := m.widget.Query()
	var display []claude.Project
	if query == "" {
		display = m.allProjects
	} else {
		for _, p := range m.allProjects {
			if _, ok := fuzzyfinder.MatchRanges(query, projectSearchable(p)); ok {
				display = append(display, p)
			}
		}
	}
	*m.displayRef = display
	m.widget.SetItemCount(max(len(display), 1))
	m.widget.SetSelected(0)
}

func (m *Model) recomputeConvFilter() {
	query := m.convWidget.Query()
	var display []claude.Conversation
	if query == "" {
		display = m.allConversations
	} else {
		for _, c := range m.allConversations {
			if _, ok := fuzzyfinder.MatchRanges(query, c.Title); ok {
				display = append(display, c)
			}
		}
	}
	*m.convDisplayRef = display
	m.convWidget.SetItemCount(max(len(display), 1))
	m.convWidget.SetSelected(0)
}

func (m Model) View() tea.View {
	var content string
	switch m.page {
	case pageProjects:
		if m.projectsErr != nil {
			content = errorStyle.Render("Error: " + m.projectsErr.Error())
		} else {
			content = m.widget.View()
		}
	case pageConversations:
		if m.conversationsErr != nil {
			content = errorStyle.Render("Error: " + m.conversationsErr.Error())
		} else if m.conversationsLoading {
			content = "Loading..."
		} else {
			content = m.convWidget.View()
		}
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// projectSearchable returns the string matched against the fuzzy query.
func projectSearchable(p claude.Project) string {
	return p.Label + " " + p.Subtitle
}

func renderProjectRow(p claude.Project, query string, selected bool) string {
	searchable := projectSearchable(p)
	ranges, _ := fuzzyfinder.MatchRanges(query, searchable)

	labelLen := len([]rune(p.Label))
	subtitleStart := labelLen + 1

	var labelR, subtitleR []int
	for _, pos := range ranges {
		if pos < labelLen {
			labelR = append(labelR, pos)
		} else if pos >= subtitleStart {
			subtitleR = append(subtitleR, pos-subtitleStart)
		}
	}

	plain := lipgloss.NewStyle()
	sep := fuzzyfinder.Highlight(" ", nil, plain, selected)
	label := fuzzyfinder.Highlight(p.Label, labelR, plain, selected)
	subtitle := fuzzyfinder.Highlight(p.Subtitle, subtitleR, fuzzyfinder.SubtitleStyle, selected)
	return label + sep + subtitle
}

func renderConversationRow(c claude.Conversation, query string, selected bool) string {
	title := c.Title
	if title == "" {
		title = c.SessionID
	}

	ranges, _ := fuzzyfinder.MatchRanges(query, title)

	plain := lipgloss.NewStyle()
	titleStr := fuzzyfinder.Highlight(title, ranges, plain, selected)

	var timeStr string
	if !c.LastAccessed.IsZero() {
		rel := relativeTime(c.LastAccessed)
		abs := c.LastAccessed.Format("2006-01-02 15:04")
		relStr := fuzzyfinder.Highlight("  "+rel, nil, fuzzyfinder.SubtitleStyle, selected)
		absStr := fuzzyfinder.Highlight("  "+abs, nil, fuzzyfinder.SubtitleStyle, selected)
		timeStr = relStr + absStr
	}

	return titleStr + timeStr
}

func relativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		n := int(diff.Minutes())
		return fmt.Sprintf("%d minute%s ago", n, pluralS(n))
	case diff < 24*time.Hour:
		n := int(diff.Hours())
		return fmt.Sprintf("%d hour%s ago", n, pluralS(n))
	case diff < 7*24*time.Hour:
		n := int(diff.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", n, pluralS(n))
	case diff < 30*24*time.Hour:
		n := int(diff.Hours() / (24 * 7))
		return fmt.Sprintf("%d week%s ago", n, pluralS(n))
	default:
		n := int(diff.Hours() / (24 * 30))
		return fmt.Sprintf("%d month%s ago", n, pluralS(n))
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
