package beads

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

// IssueLister is the subset of Adapter behavior the TUI list depends on,
// letting tests inject a stub instead of shelling out to bd.
type IssueLister interface {
	List(scope Scope) ([]Issue, error)
	Ready() (map[string]bool, error)
}

// issuesLoadedMsg carries the async result of a List call.
type issuesLoadedMsg struct {
	issues []Issue
	err    error
}

// readyLoadedMsg carries the async result of a Ready call.
type readyLoadedMsg struct {
	ready map[string]bool
	err   error
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
	readyRef   *map[string]bool
	widget     fuzzyfinder.Model

	scope    Scope
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
	readyRef := new(map[string]bool)

	m := Model{
		cfg:        cfg,
		queryRef:   queryRef,
		displayRef: displayRef,
		readyRef:   readyRef,
		scope:      cfg.Scope,
		loading:    true,
	}

	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			display := *displayRef
			if i >= len(display) {
				return ""
			}
			return renderIssueRow(display[i], *readyRef, *queryRef, selected)
		},
		Footer:    "enter: copy id & quit  ctrl+f: cycle scope  esc/ctrl+c: quit",
		ItemCount: 1,
	})

	return m
}

// Init starts the widget cursor blink and kicks off the initial issue and
// ready-set loads.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.widget.Init(), loadIssuesCmd(m.cfg.Lister, m.scope), loadReadyCmd(m.cfg.Lister))
}

func loadIssuesCmd(lister IssueLister, scope Scope) tea.Cmd {
	return func() tea.Msg {
		issues, err := lister.List(scope)
		return issuesLoadedMsg{issues: issues, err: err}
	}
}

func loadReadyCmd(lister IssueLister) tea.Cmd {
	return func() tea.Msg {
		ready, err := lister.Ready()
		return readyLoadedMsg{ready: ready, err: err}
	}
}

// nextScope returns the next scope in the ctrl+f cycle: actionable ->
// ready-only -> blocked-only -> all incl. closed -> back to actionable.
func nextScope(s Scope) Scope {
	switch s {
	case ScopeReady:
		return ScopeBlocked
	case ScopeBlocked:
		return ScopeAll
	case ScopeAll:
		return ScopeActionable
	default:
		return ScopeReady
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
		SortIssues(m.allItems, *m.readyRef)
		m.recomputeFilter()
		return m, nil

	case readyLoadedMsg:
		if msg.err != nil {
			return m, nil
		}
		*m.readyRef = msg.ready
		SortIssues(m.allItems, *m.readyRef)
		m.recomputeFilter()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "ctrl+f":
			m.scope = nextScope(m.scope)
			m.loading = true
			m.loadErr = nil
			return m, loadIssuesCmd(m.cfg.Lister, m.scope)

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

// readinessGlyph is the row's readiness indicator, distinct from statusIcon:
// a filled dot for unblocked, a triangle for blocked, a dim middot otherwise.
func readinessGlyph(r Readiness) string {
	switch r {
	case Unblocked:
		return "●"
	case Blocked:
		return "▲"
	default:
		return "·"
	}
}

func readinessStyle(r Readiness) lipgloss.Style {
	switch r {
	case Unblocked:
		return readinessUnblockedStyle
	case Blocked:
		return readinessBlockedStyle
	default:
		return readinessOtherStyle
	}
}

// readinessBadge renders the dim "↓N ↑M" blocker/dependent badge from
// DependencyCount/DependentCount, hiding either side that's zero and
// returning "" when both are zero.
func readinessBadge(issue Issue) string {
	var parts []string
	if issue.DependencyCount > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", issue.DependencyCount))
	}
	if issue.DependentCount > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", issue.DependentCount))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// rowTag returns the dim hierarchy tag for a row: "epic" for epic issues, a
// "↳ <parent>" breadcrumb for subtasks, or "" for standalone issues.
func rowTag(issue Issue) string {
	if issue.IssueType == "epic" {
		return "epic"
	}
	if issue.Parent != "" {
		return "↳ " + issue.Parent
	}
	return ""
}

// renderIssueRow renders "{readiness glyph} {status icon} {id}  {title}  {tag}  {badge}",
// with title fuzzy-match characters highlighted. tag and badge are omitted
// when empty.
func renderIssueRow(issue Issue, readyIDs map[string]bool, query string, selected bool) string {
	readiness := ClassifyReadiness(issue, readyIDs)
	glyphStyle := readinessStyle(readiness)
	if selected {
		glyphStyle = glyphStyle.Background(fuzzyfinder.SelectedBg)
	}
	glyph := glyphStyle.Render(readinessGlyph(readiness) + " ")

	icon := statusIcon(issue.Status) + " "
	if selected {
		icon = lipgloss.NewStyle().Background(fuzzyfinder.SelectedBg).Render(icon)
	}

	ranges, _ := fuzzyfinder.MatchRanges(query, issue.Title)

	plain := lipgloss.NewStyle()
	id := fuzzyfinder.Highlight(issue.ID, nil, fuzzyfinder.SubtitleStyle, selected)
	sep := fuzzyfinder.Highlight("  ", nil, plain, selected)
	title := fuzzyfinder.Highlight(issue.Title, ranges, plain, selected)

	line := glyph + icon + id + sep + title

	if tag := rowTag(issue); tag != "" {
		line += fuzzyfinder.Highlight("  ", nil, plain, selected) +
			fuzzyfinder.Highlight(tag, nil, fuzzyfinder.SubtitleStyle, selected)
	}

	if badge := readinessBadge(issue); badge != "" {
		line += fuzzyfinder.Highlight("  ", nil, plain, selected) +
			fuzzyfinder.Highlight(badge, nil, fuzzyfinder.SubtitleStyle, selected)
	}

	return line
}
