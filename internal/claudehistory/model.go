package claudehistory

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/claude"
	"github.com/elentok/blf/internal/fuzzyfinder"
)

type page int

const (
	pageProjects page = iota
)

type projectsLoadedMsg struct {
	projects []claude.Project
	err      error
}

// Model is the root bubbletea model for the claude history TUI.
type Model struct {
	page        page
	allProjects []claude.Project
	displayRef  *[]claude.Project
	queryRef    *string
	widget      fuzzyfinder.Model
	projectsErr error
	width       int
	height      int
}

// New creates a new history Model. It returns a model that starts on the
// projects page and immediately triggers an async load of projects.
func New(projectsRoot string) Model {
	displayRef := new([]claude.Project)
	*displayRef = nil
	queryRef := new(string)

	m := Model{
		page:       pageProjects,
		displayRef: displayRef,
		queryRef:   queryRef,
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

	return m
}

// Init loads projects asynchronously.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.widget.Init(),
		loadProjectsCmd(""),
	)
}

func loadProjectsCmd(root string) tea.Cmd {
	return func() tea.Msg {
		projects, err := claude.ListProjects(root)
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.widget.SetSize(msg.Width, msg.Height)
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
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			// stub for this slice — next slice wires conversation page
			return m, nil
		}
	}

	prevQuery := m.widget.Query()
	var cmd tea.Cmd
	m.widget, cmd = m.widget.Update(msg)
	if m.widget.Query() != prevQuery {
		*m.queryRef = m.widget.Query()
		m.recomputeFilter()
	}

	return m, cmd
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

func (m Model) View() tea.View {
	var content string
	if m.projectsErr != nil {
		content = errorStyle.Render("Error: " + m.projectsErr.Error())
	} else {
		content = m.widget.View()
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
