package beads

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

// IssueLister is the subset of Adapter behavior the TUI list depends on,
// letting tests inject a stub instead of shelling out to bd.
type IssueLister interface {
	List(scope Scope) ([]Issue, error)
}

// issuesLoadedMsg carries the async result of the initial List call.
type issuesLoadedMsg struct {
	issues []Issue
	err    error
}

// ModelConfig holds injectable dependencies for the beads TUI model.
type ModelConfig struct {
	Lister   IssueLister
	Scope    Scope
	CopyText func(string) error // optional; nil disables the clipboard write on enter
}

// Model is the bubbletea model for the `blf beads` picker: a flat fuzzy list
// of issues in scope. Enter copies the selected issue id (via CopyText) and
// quits; the id is also made available to the caller via SelectedID once the
// program returns.
type Model struct {
	cfg ModelConfig

	queryRef   *string
	displayRef *[]Issue
	widget     fuzzyfinder.Model

	allItems []Issue
	loading  bool
	loadErr  error

	selectedID string

	width, height int
}

// NewModel returns a Model ready to embed/run.
func NewModel(cfg ModelConfig) Model {
	queryRef := new(string)
	displayRef := new([]Issue)

	m := Model{
		cfg:        cfg,
		queryRef:   queryRef,
		displayRef: displayRef,
		loading:    true,
	}

	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			display := *displayRef
			if i >= len(display) {
				return ""
			}
			return renderIssueRow(display[i], *queryRef, selected)
		},
		Footer:    "enter: copy id & quit  esc/ctrl+c: quit",
		ItemCount: 1,
	})

	return m
}

// Init starts the widget cursor blink and kicks off the initial issue load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.widget.Init(), loadIssuesCmd(m.cfg.Lister, m.cfg.Scope))
}

func loadIssuesCmd(lister IssueLister, scope Scope) tea.Cmd {
	return func() tea.Msg {
		issues, err := lister.List(scope)
		return issuesLoadedMsg{issues: issues, err: err}
	}
}

// SelectedID returns the id of the issue chosen via enter, or "" if the user
// quit without selecting one.
func (m Model) SelectedID() string {
	return m.selectedID
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.widget.SetSize(msg.Width, msg.Height)
		return m, nil

	case issuesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		m.allItems = msg.issues
		m.recomputeFilter()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			display := *m.displayRef
			sel := m.widget.Selected()
			if len(display) == 0 || sel >= len(display) {
				return m, nil
			}
			id := display[sel].ID
			if m.cfg.CopyText != nil {
				_ = m.cfg.CopyText(id)
			}
			m.selectedID = id
			return m, tea.Quit
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

// recomputeFilter re-derives the displayed issue list from allItems and the
// widget's current query, via fuzzyfinder.Find (AND-semantics multi-word
// match). An empty query shows every item in adapter order, matching Find's
// documented "empty query returns nil" behavior rather than treating it as
// "no matches".
func (m *Model) recomputeFilter() {
	query := m.widget.Query()
	if query == "" {
		*m.displayRef = m.allItems
	} else {
		candidates := make([]string, len(m.allItems))
		for i, issue := range m.allItems {
			candidates[i] = issue.ID + " " + issue.Title
		}
		matches := fuzzyfinder.Find(query, candidates)
		display := make([]Issue, len(matches))
		for i, match := range matches {
			display[i] = m.allItems[match.Index]
		}
		*m.displayRef = display
	}
	m.widget.SetItemCount(max(len(*m.displayRef), 1))
	m.widget.SetSelected(0)
}

func (m Model) View() tea.View {
	var content string
	switch {
	case m.loadErr != nil:
		content = errorStyle.Render("Error: " + m.loadErr.Error())
	case m.loading:
		content = "Loading issues…"
	case len(m.allItems) == 0:
		content = emptyStateStyle.Render("No issues in scope.")
	default:
		content = m.widget.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// statusIcon mirrors the glyphs bd's own CLI output uses for these statuses.
func statusIcon(status string) string {
	switch status {
	case "open":
		return "○"
	case "in_progress":
		return "◐"
	case "closed":
		return "✓"
	default:
		return "●"
	}
}

// renderIssueRow renders "{status icon} {id}  {title}", with title fuzzy-match
// characters highlighted.
func renderIssueRow(issue Issue, query string, selected bool) string {
	icon := statusIcon(issue.Status) + " "
	if selected {
		icon = lipgloss.NewStyle().Background(fuzzyfinder.SelectedBg).Render(icon)
	}

	ranges, _ := fuzzyfinder.MatchRanges(query, issue.Title)

	plain := lipgloss.NewStyle()
	id := fuzzyfinder.Highlight(issue.ID, nil, fuzzyfinder.SubtitleStyle, selected)
	sep := fuzzyfinder.Highlight("  ", nil, plain, selected)
	title := fuzzyfinder.Highlight(issue.Title, ranges, plain, selected)

	return icon + id + sep + title
}
