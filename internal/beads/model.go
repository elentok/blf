package beads

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/fuzzyfinder"
)

// previewMinWidth is the terminal width below which the issue preview moves
// from a side-by-side pane to a stacked one below the list, to keep the list
// column from getting squeezed unreadably narrow; tab overrides this.
const previewMinWidth = 100

// previewMinWidthForStack is the terminal width below which even a stacked
// preview doesn't fit usefully and the preview auto-hides entirely; tab
// overrides this too.
const previewMinWidthForStack = 40

// previewDebounceDelay is how long the preview fetch waits after a selection
// change before shelling out, so scrolling the list doesn't spawn a bd call
// per row.
const previewDebounceDelay = 100 * time.Millisecond

// IssueLister is the subset of Adapter behavior the TUI list depends on,
// letting tests inject a stub instead of shelling out to bd.
type IssueLister interface {
	List(all bool) ([]Issue, error)
}

// IssueMutator is the subset of Adapter behavior the TUI actions depend on.
type IssueMutator interface {
	Create(title string, opts CreateOptions) (Issue, error)
	UpdateStatus(id, status string) (Issue, error)
	Close(id string) (Issue, error)
	Reopen(id string) (Issue, error)
}

type modeState int

const (
	modeBrowse modeState = iota
	modeCreate
	modeStatus
)

// issuesLoadedMsg carries the async result of a List call.
type issuesLoadedMsg struct {
	issues []Issue
	err    error
}

// previewDebounceMsg fires previewDebounceDelay after a selection change; if
// seq no longer matches the model's current previewSeq, the selection moved
// on again and this fetch is stale.
type previewDebounceMsg struct {
	seq int
	id  string
}

// previewLoadedMsg carries the async result of fetching an id's preview
// trees.
type previewLoadedMsg struct {
	seq  int
	id   string
	data previewData
}

type mutationFinishedMsg struct {
	issue Issue
	err   error
}

type shellFinishedMsg struct {
	issueID string
	err     error
	refresh bool
}

// ModelConfig holds injectable dependencies for the beads TUI model.
type ModelConfig struct {
	Lister     IssueLister
	Preview    PreviewFetcher
	Mutator    IssueMutator
	All        bool               // mirrors `bd list --all` (closed included) when true
	CopyText   func(string) error // optional; nil disables the clipboard write on enter
	EditIssue  func(string) tea.Cmd
	GraphIssue func(string) tea.Cmd
}

// Model is the bubbletea model for the `blf beads` picker: an interactive
// view of `bd list`'s issue tree. Enter copies the selected issue id (via
// CopyText) and quits; the id is also made available to the caller via
// SelectedID once the program returns.
type Model struct {
	cfg ModelConfig

	queryRef   *string
	displayRef *[]TreeRow
	modeRef    *modeState
	widget     fuzzyfinder.Model

	all      bool
	allItems []Issue
	loading  bool
	loadErr  error

	selectedID string
	mode       modeState
	helpMode   bool

	modeSavedQuery      string
	modeSavedSelectedID string
	createParentID      string
	createStandalone    bool
	pendingSelectID     string

	// previewCache holds fetched trees per issue id; previewSeq guards
	// against a stale debounce/fetch replying after the selection moved on
	// again (mirrors internal/claudehistory's grep debounce).
	previewCache   map[string]previewData
	previewSeq     int
	previewLoading bool
	// previewToggled flips whatever the width-based previewMinWidth default
	// would otherwise decide (tab key), so it works both to hide the
	// preview on a wide terminal and to force it on a narrow one.
	previewToggled bool
	previewWidth   int
	previewHeight  int
	previewStacked bool

	width, height int
}

// NewModel returns a Model ready to embed/run.
func NewModel(cfg ModelConfig) Model {
	queryRef := new(string)
	displayRef := new([]TreeRow)
	modeRef := new(modeState)

	m := Model{
		cfg:          cfg,
		queryRef:     queryRef,
		displayRef:   displayRef,
		modeRef:      modeRef,
		all:          cfg.All,
		loading:      true,
		previewCache: make(map[string]previewData),
	}

	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			if *modeRef == modeStatus {
				if i >= len(StatusChoices) {
					return ""
				}
				return renderStatusRow(StatusChoices[i], selected)
			}
			display := *displayRef
			if i >= len(display) {
				return ""
			}
			return renderIssueRow(display[i], *queryRef, selected)
		},
		Footer:    browseFooter,
		ItemCount: 1,
	})
	m.syncWidgetChrome()

	return m
}

// Init starts the widget cursor blink and kicks off the initial issue load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.widget.Init(), loadIssuesCmd(m.cfg.Lister, m.all))
}

func loadIssuesCmd(lister IssueLister, all bool) tea.Cmd {
	return func() tea.Msg {
		issues, err := lister.List(all)
		return issuesLoadedMsg{issues: issues, err: err}
	}
}

// SelectedID returns the id of the issue chosen via enter, or "" if the user
// quit without selecting one.
func (m Model) SelectedID() string {
	return m.selectedID
}

// Update forwards to updateInner and then, if the selection moved as a
// result, kicks off a debounced preview fetch for the newly selected issue.
// Centralizing the check here (rather than at every code path that can move
// the selection: nav keys, filtering, scope reload, ready re-sort) means the
// lazy-preview behavior can't be missed by a future call site.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prevID := m.selectedRowID()
	next, cmd := m.updateInner(msg)
	nm := next.(Model)
	if newID := nm.selectedRowID(); newID != prevID {
		cmd = tea.Batch(cmd, nm.startPreviewDebounceCmd(newID))
	}
	return nm, cmd
}

func (m Model) updateInner(msg tea.Msg) (tea.Model, tea.Cmd) {
	*m.modeRef = m.mode
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil

	case issuesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		m.loadErr = nil
		m.allItems = msg.issues
		m.recomputeFilter()
		m.restorePendingSelection()
		return m, nil

	case previewDebounceMsg:
		if msg.seq != m.previewSeq {
			return m, nil // stale: the selection moved on again
		}
		issue, ok := m.selectedIssue()
		if !ok || issue.ID != msg.id || m.cfg.Preview == nil {
			return m, nil
		}
		return m, fetchPreviewCmd(m.cfg.Preview, issue, msg.seq)

	case previewLoadedMsg:
		if msg.seq != m.previewSeq {
			return m, nil // stale: the selection moved on again
		}
		m.previewLoading = false
		m.previewCache[msg.id] = msg.data
		return m, nil

	case mutationFinishedMsg:
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		if m.mode != modeBrowse {
			m.exitMode(false)
		}
		return m, m.startReload(msg.issue.ID, true)

	case shellFinishedMsg:
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		if msg.refresh {
			return m, m.startReload(msg.issueID, false)
		}
		return m, nil

	case tea.KeyMsg:
		if m.helpMode {
			m.helpMode = false
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			if m.mode != modeBrowse {
				m.exitMode(true)
				return m, nil
			}
			return m, tea.Quit

		case "tab":
			if m.mode != modeBrowse {
				return m, nil
			}
			m.previewToggled = !m.previewToggled
			m.applyLayout()
			return m, nil

		case "ctrl+r":
			if m.mode != modeBrowse {
				return m, nil
			}
			return m, m.startReload(m.selectedRowID(), false)

		case "ctrl+a":
			if m.mode != modeBrowse {
				return m, nil
			}
			m.enterCreateMode()
			return m, nil

		case "ctrl+t":
			if m.mode != modeCreate {
				return m, nil
			}
			if m.createParentID == "" {
				return m, nil
			}
			m.createStandalone = !m.createStandalone
			m.syncWidgetChrome()
			return m, nil

		case "ctrl+s":
			if m.mode != modeBrowse {
				return m, nil
			}
			m.enterStatusMode()
			return m, nil

		case "ctrl+x":
			if m.mode != modeBrowse {
				return m, nil
			}
			return m.toggleClosed()

		case "ctrl+e":
			if m.mode != modeBrowse || m.cfg.EditIssue == nil {
				return m, nil
			}
			issue, ok := m.selectedIssue()
			if !ok {
				return m, nil
			}
			return m, m.cfg.EditIssue(issue.ID)

		case "ctrl+g":
			if m.mode != modeBrowse || m.cfg.GraphIssue == nil {
				return m, nil
			}
			issue, ok := m.selectedIssue()
			if !ok {
				return m, nil
			}
			return m, m.cfg.GraphIssue(issue.ID)

		case "?":
			if m.mode != modeBrowse {
				return m, nil
			}
			m.helpMode = true
			return m, nil

		case "enter":
			switch m.mode {
			case modeCreate:
				return m.submitCreate()
			case modeStatus:
				return m.submitStatus()
			}
			display := *m.displayRef
			sel := m.widget.Selected()
			if len(display) == 0 || sel >= len(display) {
				return m, nil
			}
			id := display[sel].Issue.ID
			if m.cfg.CopyText != nil {
				_ = m.cfg.CopyText(id)
			}
			m.selectedID = id
			return m, tea.Quit
		}
	}

	if m.mode == modeCreate {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up", "down", "ctrl+k", "ctrl+j", "ctrl+p", "ctrl+n":
				return m, nil
			}
		}
	}

	prevQuery := m.widget.Query()
	var cmd tea.Cmd
	m.widget, cmd = m.widget.Update(msg)
	if m.widget.Query() != prevQuery {
		switch m.mode {
		case modeBrowse:
			*m.queryRef = m.widget.Query()
			m.recomputeFilter()
		case modeCreate:
			// The create title reuses the input line but must not refilter rows.
		case modeStatus:
			// Status mode owns the row set; typing is inert until the user exits.
		}
	}
	return m, cmd
}

const browseFooter = "?: help"

func (m *Model) syncWidgetChrome() {
	switch m.mode {
	case modeCreate:
		m.widget.SetPrompt("create ")
		parent := "standalone"
		if m.createParentID != "" && !m.createStandalone {
			parent = "parent " + m.createParentID
		}
		m.widget.SetFooter("enter: create  ctrl+t: toggle parent  esc: cancel  target: " + parent)
	case modeStatus:
		m.widget.SetPrompt("status ")
		m.widget.SetFooter("enter: set status  esc: cancel")
	default:
		m.widget.SetPrompt("")
		m.widget.SetFooter(browseFooter)
	}
}

func (m *Model) enterCreateMode() {
	m.mode = modeCreate
	*m.modeRef = m.mode
	m.modeSavedQuery = m.widget.Query()
	m.modeSavedSelectedID = m.selectedRowID()
	m.createParentID = ""
	m.createStandalone = false
	if issue, ok := m.selectedIssue(); ok && issue.IssueType == "epic" {
		m.createParentID = issue.ID
	}
	m.widget.SetQuery("")
	*m.queryRef = ""
	m.syncWidgetChrome()
}

func (m *Model) enterStatusMode() {
	issue, ok := m.selectedIssue()
	if !ok {
		return
	}
	m.mode = modeStatus
	*m.modeRef = m.mode
	m.modeSavedQuery = m.widget.Query()
	m.modeSavedSelectedID = issue.ID
	m.widget.SetQuery("")
	*m.queryRef = ""
	m.widget.SetItemCount(len(StatusChoices))
	m.widget.SetSelected(statusChoiceIndex(issue.Status))
	m.syncWidgetChrome()
}

func (m *Model) exitMode(restoreQuery bool) {
	m.mode = modeBrowse
	*m.modeRef = m.mode
	m.createParentID = ""
	m.createStandalone = false
	if restoreQuery {
		m.widget.SetQuery(m.modeSavedQuery)
		*m.queryRef = m.modeSavedQuery
		m.recomputeFilter()
		m.selectByID(m.modeSavedSelectedID)
	} else {
		m.widget.SetQuery("")
		*m.queryRef = ""
		m.recomputeFilter()
	}
	m.modeSavedQuery = ""
	m.modeSavedSelectedID = ""
	m.syncWidgetChrome()
}

func (m *Model) submitCreate() (tea.Model, tea.Cmd) {
	if m.cfg.Mutator == nil {
		return *m, nil
	}
	title := strings.TrimSpace(m.widget.Query())
	if title == "" {
		return *m, nil
	}
	opts := CreateOptions{Type: "task"}
	if m.createParentID != "" && !m.createStandalone {
		opts.Parent = m.createParentID
	}
	return *m, func() tea.Msg {
		issue, err := m.cfg.Mutator.Create(title, opts)
		return mutationFinishedMsg{issue: issue, err: err}
	}
}

func (m *Model) submitStatus() (tea.Model, tea.Cmd) {
	if m.cfg.Mutator == nil {
		return *m, nil
	}
	issueID := m.modeSavedSelectedID
	if issueID == "" {
		return *m, nil
	}
	idx := m.widget.Selected()
	if idx < 0 || idx >= len(StatusChoices) {
		return *m, nil
	}
	status := StatusChoices[idx]
	return *m, func() tea.Msg {
		issue, err := m.cfg.Mutator.UpdateStatus(issueID, status)
		return mutationFinishedMsg{issue: issue, err: err}
	}
}

func (m *Model) toggleClosed() (tea.Model, tea.Cmd) {
	if m.cfg.Mutator == nil {
		return *m, nil
	}
	issue, ok := m.selectedIssue()
	if !ok {
		return *m, nil
	}
	return *m, func() tea.Msg {
		var updated Issue
		var err error
		if issue.Status == "closed" {
			updated, err = m.cfg.Mutator.Reopen(issue.ID)
		} else {
			updated, err = m.cfg.Mutator.Close(issue.ID)
		}
		return mutationFinishedMsg{issue: updated, err: err}
	}
}

func (m *Model) startReload(selectID string, invalidatePreview bool) tea.Cmd {
	m.loading = true
	m.loadErr = nil
	m.pendingSelectID = selectID
	if invalidatePreview && selectID != "" {
		delete(m.previewCache, selectID)
	}
	return loadIssuesCmd(m.cfg.Lister, m.all)
}

func (m *Model) restorePendingSelection() {
	if m.pendingSelectID == "" {
		return
	}
	m.selectByID(m.pendingSelectID)
}

func (m *Model) selectByID(id string) {
	if id == "" {
		return
	}
	display := *m.displayRef
	for i, row := range display {
		if row.Issue.ID == id {
			m.widget.SetSelected(i)
			return
		}
	}
}

func statusChoiceIndex(status string) int {
	for i, candidate := range StatusChoices {
		if candidate == status {
			return i
		}
	}
	return 0
}

// selectedIssue returns the issue at the widget's current selection, or
// false if the display list is empty or the selection is out of range.
func (m Model) selectedIssue() (Issue, bool) {
	if m.mode == modeStatus {
		for _, row := range *m.displayRef {
			if row.Issue.ID == m.modeSavedSelectedID {
				return row.Issue, true
			}
		}
		return Issue{}, false
	}
	display := *m.displayRef
	sel := m.widget.Selected()
	if len(display) == 0 || sel >= len(display) {
		return Issue{}, false
	}
	return display[sel].Issue, true
}

// selectedRowID returns the id of the currently selected row, or "" if
// nothing is selected.
func (m Model) selectedRowID() string {
	issue, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	return issue.ID
}

// startPreviewDebounceCmd bumps previewSeq and, unless id's preview is
// already cached, returns a Cmd that waits previewDebounceDelay before
// firing a previewDebounceMsg — scrolling through the list rapidly only
// triggers the actual bd calls once the cursor settles.
func (m *Model) startPreviewDebounceCmd(id string) tea.Cmd {
	m.previewSeq++
	seq := m.previewSeq
	if id == "" {
		return nil
	}
	if _, cached := m.previewCache[id]; cached {
		return nil
	}
	m.previewLoading = true
	return func() tea.Msg {
		time.Sleep(previewDebounceDelay)
		return previewDebounceMsg{seq: seq, id: id}
	}
}

// fetchPreviewCmd fetches root's subtasks + blocked-by trees via fetcher and
// reports the result tagged with seq so a stale reply can be discarded.
func fetchPreviewCmd(fetcher PreviewFetcher, root Issue, seq int) tea.Cmd {
	return func() tea.Msg {
		return previewLoadedMsg{seq: seq, id: root.ID, data: fetchPreviewData(fetcher, root)}
	}
}

// previewVisible reports whether the preview pane should render at all,
// combining the width-based auto-hide (terminals too narrow even for a
// stacked pane) with the user's tab toggle, which flips whatever the width
// would otherwise decide.
func (m Model) previewVisible() bool {
	fits := m.width >= previewMinWidthForStack
	return fits != m.previewToggled
}

// applyLayout re-derives the list/preview split from the current terminal
// size: side-by-side (list ~40% left, preview ~60% right) on wide terminals,
// stacked (list on top, preview below) on narrower ones down to
// previewMinWidthForStack, and full-width list with no preview below that.
func (m *Model) applyLayout() {
	if !m.previewVisible() {
		m.widget.SetSize(m.width, m.height)
		m.previewWidth = 0
		m.previewHeight = 0
		m.previewStacked = false
		return
	}

	if m.width >= previewMinWidth {
		m.previewStacked = false
		leftWidth := m.width * 2 / 5
		m.widget.SetSize(leftWidth, m.height)
		m.previewWidth = m.width - leftWidth
		m.previewHeight = 0
		return
	}

	m.previewStacked = true
	topHeight := m.height * 3 / 5
	m.widget.SetSize(m.width, topHeight)
	m.previewWidth = m.width
	m.previewHeight = m.height - topHeight
}

// recomputeFilter re-derives the displayed issue tree from allItems and the
// widget's current query. An empty query builds the full **issue tree**
// (see CONTEXT.md) undimmed; a non-empty query fuzzy-matches (via
// fuzzyfinder.Find, AND-semantics multi-word match) and keeps only matches
// plus their non-matching ancestors, which render dimmed for tree context.
func (m *Model) recomputeFilter() {
	query := m.widget.Query()
	var matchIDs map[string]bool
	if query != "" {
		candidates := make([]string, len(m.allItems))
		for i, issue := range m.allItems {
			candidates[i] = issue.ID + " " + issue.Title
		}
		matches := fuzzyfinder.Find(query, candidates)
		matchIDs = make(map[string]bool, len(matches))
		for _, match := range matches {
			matchIDs[m.allItems[match.Index].ID] = true
		}
	}
	*m.displayRef = BuildIssueTree(m.allItems, matchIDs)
	m.widget.SetItemCount(max(len(*m.displayRef), 1))
	m.widget.SetSelected(0)
}

func (m Model) View() tea.View {
	if m.helpMode {
		w := max(m.width, 14)
		h := max(m.height, 6)
		content := previewStyle.Width(w).Height(h).Render(m.renderHelp())
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	var content string
	switch {
	case m.loadErr != nil:
		content = errorStyle.Render("Error: " + m.loadErr.Error())
	case m.loading:
		content = "Loading issues…"
	case len(m.allItems) == 0:
		content = emptyStateStyle.Render("No issues found.")
	case m.previewVisible() && m.previewStacked:
		content = lipgloss.JoinVertical(lipgloss.Left, m.widget.View(), m.renderPreview())
	case m.previewVisible():
		content = lipgloss.JoinHorizontal(lipgloss.Top, m.widget.View(), m.renderPreview())
	default:
		content = m.widget.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) renderHelp() string {
	type binding struct{ key, desc string }
	bindings := []binding{
		{"↑ / ↓", "select issue"},
		{"ctrl+k / ctrl+j", "(aliases for ↑ / ↓)"},
		{"ctrl+p / ctrl+n", "(aliases for ↑ / ↓)"},
		{"enter", "copy selected issue id and quit"},
		{"ctrl+a", "create a new issue from the input line"},
		{"ctrl+s", "pick a new status for the selected issue"},
		{"ctrl+x", "close or reopen the selected issue"},
		{"ctrl+e", "edit the selected issue in $EDITOR"},
		{"ctrl+g", "open the selected issue's graph"},
		{"ctrl+r", "refresh the list and keep selection"},
		{"tab", "toggle the preview pane"},
		{"esc", "quit the picker"},
		{"?", "toggle this help"},
	}

	maxKeyWidth := 0
	for _, b := range bindings {
		if w := lipgloss.Width(b.key); w > maxKeyWidth {
			maxKeyWidth = w
		}
	}

	var sb strings.Builder
	sb.WriteString(previewTitleStyle.Render("blf beads - help") + "\n\n")
	for _, b := range bindings {
		pad := strings.Repeat(" ", maxKeyWidth-lipgloss.Width(b.key))
		key := fuzzyfinder.SubtitleStyle.Render(b.key) + pad
		desc := lipgloss.NewStyle().Render(b.desc)
		sb.WriteString("  " + key + "  " + desc + "\n")
	}
	sb.WriteString("\n" + previewMetaStyle.Render("  press any key to close help"))
	return sb.String()
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

func renderStatusRow(status string, selected bool) string {
	plain := lipgloss.NewStyle()
	return fuzzyfinder.Highlight(statusIcon(status)+" "+status, nil, plain, selected)
}

// renderIssueRow renders "{indent}{status icon} {id}  {title}", with title
// fuzzy-match characters highlighted. row.Depth indents the row under its
// parent in the **issue tree**; an epic's title renders bold in a distinct
// color, and a row kept only as a non-matching ancestor's context
// (row.Dimmed) renders dim instead.
func renderIssueRow(row TreeRow, query string, selected bool) string {
	issue := row.Issue
	indent := strings.Repeat("  ", row.Depth)

	icon := statusIcon(issue.Status) + " "
	if selected {
		icon = lipgloss.NewStyle().Background(fuzzyfinder.SelectedBg).Render(icon)
	}

	ranges, _ := fuzzyfinder.MatchRanges(query, issue.Title)

	base := lipgloss.NewStyle()
	switch {
	case row.Dimmed:
		base = dimRowStyle
	case issue.IssueType == "epic":
		base = epicRowStyle
	}

	id := fuzzyfinder.Highlight(issue.ID, nil, fuzzyfinder.SubtitleStyle, selected)
	sep := fuzzyfinder.Highlight("  ", nil, base, selected)
	title := fuzzyfinder.Highlight(issue.Title, ranges, base, selected)

	return indent + icon + id + sep + title
}

// renderPreview renders the side pane for the currently selected issue: a
// header (title/status/priority) and description that render instantly from
// data already in hand, followed by the subtasks + blocked-by trees once
// their async fetch lands (or a loading placeholder until it does).
//
// Every variable-length piece of text (title, description, tree lines) is
// hard-wrapped to the box's inner width and the whole result is truncated to
// the box's inner height before rendering. lipgloss's Style.Height pads a
// short block but does not truncate a tall one, and Style.Width word-wraps
// rather than clipping — so an unwrapped long line silently grows the
// rendered block past the terminal height and breaks the side-by-side
// JoinHorizontal alignment with the list pane.
func (m Model) renderPreview() string {
	innerWidth := max(m.previewWidth-4, 20) // border (2) + horizontal padding (2)
	boxHeight := m.height
	if m.previewStacked {
		boxHeight = m.previewHeight
	}
	box := previewStyle.Width(max(m.previewWidth, 10)).Height(max(boxHeight, 3))

	issue, ok := m.selectedIssue()
	if !ok {
		return box.Render("")
	}

	var lines []string
	for _, l := range wrapText(issue.Title, innerWidth) {
		lines = append(lines, previewTitleStyle.Render(l))
	}
	lines = append(lines, previewMetaStyle.Render(
		fmt.Sprintf("%s %s  ·  priority %d", statusIcon(issue.Status), issue.Status, issue.Priority)))
	if issue.Parent != "" {
		lines = append(lines, fuzzyfinder.SubtitleStyle.Render("↳ parent: "+issue.Parent))
	}
	lines = append(lines, "")

	if issue.Description != "" {
		lines = append(lines, wrapText(issue.Description, innerWidth)...)
		lines = append(lines, "")
	}

	data, cached := m.previewCache[issue.ID]
	switch {
	case !cached && m.previewLoading:
		lines = append(lines, previewSectionStyle.Render("Loading…"))
	case cached && data.err != nil:
		lines = append(lines, wrapText(errorStyle.Render("Error: "+data.err.Error()), innerWidth)...)
	case cached:
		if data.subtasks != nil {
			header, body := renderSubtasksSection(*data.subtasks)
			lines = append(lines, previewHeaderStyle.Render(header))
			lines = append(lines, wrapText(strings.Join(body, "\n"), innerWidth)...)
			lines = append(lines, "")
		}
		if data.blockedBy != nil {
			header, body := renderBlockedBySection(*data.blockedBy)
			if header != "" {
				lines = append(lines, previewHeaderStyle.Render(header))
				lines = append(lines, wrapText(strings.Join(body, "\n"), innerWidth)...)
			}
		}
	}

	// Content lines = box height minus the top/bottom border.
	maxLines := max(boxHeight-2, 1)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}

	return box.Render(strings.Join(lines, "\n"))
}

// renderSubtasksSection renders the "Subtasks (x/y done)" header plus the
// nested child lines, kept as a section distinct from renderBlockedBySection
// per CONTEXT.md's issue preview: hierarchy and dependency edges must never
// be merged into one tree.
func renderSubtasksSection(root SubtaskNode) (header string, bodyLines []string) {
	closed, total := root.CompletionCount()
	header = fmt.Sprintf("Subtasks (%d/%d done)", closed, total)
	for _, child := range root.Children {
		bodyLines = append(bodyLines, renderSubtaskLines(child, 0)...)
	}
	return header, bodyLines
}

func renderSubtaskLines(n SubtaskNode, depth int) []string {
	indent := strings.Repeat("  ", depth)
	line := indent + statusIcon(n.Issue.Status) + " " + n.Issue.ID + "  " + n.Issue.Title
	lines := []string{line}
	for _, child := range n.Children {
		lines = append(lines, renderSubtaskLines(child, depth+1)...)
	}
	return lines
}

// renderBlockedBySection renders the "Blocked by" header plus the transitive
// dependency lines, or "" when root has no blockers.
func renderBlockedBySection(root BlockedByNode) (header string, bodyLines []string) {
	for _, child := range root.Children {
		bodyLines = append(bodyLines, renderBlockedByLines(child, 0)...)
	}
	if len(bodyLines) == 0 {
		return "", nil
	}
	return "Blocked by", bodyLines
}

func renderBlockedByLines(n BlockedByNode, depth int) []string {
	indent := strings.Repeat("  ", depth)
	label := n.Issue.ID + "  " + n.Issue.Title
	if n.IsBackRef {
		label = n.Issue.ID + " (see above)"
	}
	line := indent + statusIcon(n.Issue.Status) + " " + label
	lines := []string{line}
	if !n.IsBackRef {
		for _, child := range n.Children {
			lines = append(lines, renderBlockedByLines(child, depth+1)...)
		}
	}
	return lines
}

// wrapText hard-wraps text to at most width runes per line (naive, no word
// boundaries — matches internal/claudehistory's grep preview wrapping),
// splitting first on existing newlines so multi-paragraph text keeps its
// paragraph breaks.
func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 80
	}
	var lines []string
	for para := range strings.SplitSeq(text, "\n") {
		runes := []rune(para)
		if len(runes) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(runes) > width {
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		lines = append(lines, string(runes))
	}
	return lines
}
