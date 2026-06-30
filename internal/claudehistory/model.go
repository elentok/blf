package claudehistory

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/elentok/blf/internal/claude"
	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/platform"
)

type page int

const (
	pageProjects page = iota
	pageConversations
	pageGrep
)

type grepScope int

const (
	grepScopeGlobal  grepScope = iota
	grepScopeProject           // single project dir
)

type projectsLoadedMsg struct {
	projects []claude.Project
	err      error
}

type conversationsLoadedMsg struct {
	conversations []claude.Conversation
	err           error
}

type convExportedMsg struct {
	path string
	err  error
}

type editorFinishedMsg struct{ err error }

type resumeFinishedMsg struct{ err error }

type grepDebounceMsg struct {
	seq   int
	query string
}

type grepResultsMsg struct {
	results []claude.GrepResult
	err     error
	seq     int
}

type grepExportedMsg struct {
	path   string
	query  string
	editor string
	err    error
}

type grepEditorFinishedMsg struct{ err error }

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
	conversationsStatus  string
	conversationsLoading bool
	convProjectDir       string // project dir of the open conversations page
	convProjectCwd       string // project cwd of the open conversations page

	// grep page state
	grepWidget           fuzzyfinder.Model
	grepResults          []claude.GrepResult
	grepResultsRef       *[]claude.GrepResult
	grepSeq              int
	grepRunning          bool
	grepErr              error
	grepStatus           string
	rgNotFound           bool
	grepFromPage         page
	grepScope            grepScope
	grepScopeProjDir     string // dir for grepScopeProject
	widthRef             *int
	grepPreviewBoxHeight int // total preview frame height (border included), set on resize

	width  int
	height int
}

// grepPreviewBoxHeight returns the total preview frame height (border
// included) as a third of the available terminal height, clamped so both the
// results list and the preview stay usable on small and large terminals.
func grepPreviewBoxHeight(termHeight int) int {
	h := termHeight / 3
	h = min(max(h, 8), 20)
	if termHeight-h < 6 {
		h = max(termHeight-6, 3)
	}
	return h
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

	grepResultsRef := new([]claude.GrepResult)
	widthRef := new(int)

	m := Model{
		page:           pageProjects,
		displayRef:     displayRef,
		queryRef:       queryRef,
		convDisplayRef: convDisplayRef,
		convQueryRef:   convQueryRef,
		grepResultsRef: grepResultsRef,
		widthRef:       widthRef,
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
		Footer:    "type: filter  ↑/↓: move  enter: open  ctrl+f: grep  esc: quit",
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
		Footer:    "type: filter  ↑/↓: move  enter: open  ctrl+f: grep  ctrl+r: resume  ctrl+y: yank session id  esc: back",
		ItemCount: 1,
	})

	m.grepWidget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			return renderGrepRow(grepResultsRef, widthRef, i, selected)
		},
		Footer:    "type: search  ↑/↓: move  enter: open  ctrl+g: toggle scope  ctrl+r: resume  esc: back",
		ItemCount: 1,
		Prompt:    "grep ",
	})

	return m
}

// Init loads projects asynchronously.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.widget.Init(),
		m.convWidget.Init(),
		m.grepWidget.Init(),
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
		*m.widthRef = msg.Width
		m.widget.SetSize(msg.Width, msg.Height)
		m.convWidget.SetSize(msg.Width, msg.Height)
		m.grepPreviewBoxHeight = grepPreviewBoxHeight(msg.Height)
		m.grepWidget.SetSize(msg.Width, max(msg.Height-m.grepPreviewBoxHeight, 4))
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

	case convExportedMsg:
		if msg.err != nil {
			m.conversationsErr = msg.err
			return m, nil
		}
		editor := resolveEditor()
		return m, tea.ExecProcess(exec.Command(editor, msg.path), func(err error) tea.Msg {
			return editorFinishedMsg{err: err}
		})

	case editorFinishedMsg:
		return m, nil

	case resumeFinishedMsg:
		if msg.err != nil {
			if m.page == pageGrep {
				m.grepErr = msg.err
			} else {
				m.conversationsErr = msg.err
			}
		}
		return m, nil

	case grepDebounceMsg:
		if msg.seq != m.grepSeq || msg.query != m.grepWidget.Query() {
			return m, nil // stale
		}
		if len([]rune(msg.query)) < 3 {
			m.grepResults = nil
			*m.grepResultsRef = nil
			m.grepRunning = false
			m.grepWidget.SetItemCount(1)
			return m, nil
		}
		m.grepRunning = true
		seq := m.grepSeq
		query := msg.query
		dirs := m.grepDirs()
		return m, func() tea.Msg {
			results, err := claude.GrepTranscripts(query, dirs)
			return grepResultsMsg{results: results, err: err, seq: seq}
		}

	case grepResultsMsg:
		if msg.seq != m.grepSeq {
			return m, nil // stale
		}
		m.grepRunning = false
		if msg.err != nil {
			if errors.Is(msg.err, claude.ErrRgNotFound) {
				m.rgNotFound = true
			} else {
				m.grepErr = msg.err
			}
			return m, nil
		}
		m.grepResults = msg.results
		*m.grepResultsRef = msg.results
		m.grepWidget.SetItemCount(max(len(msg.results), 1))
		m.grepWidget.SetSelected(0)
		return m, nil

	case grepExportedMsg:
		if msg.err != nil {
			m.grepErr = msg.err
			return m, nil
		}
		cmd := buildEditorCmd(msg.editor, msg.path, msg.query)
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return grepEditorFinishedMsg{err: err}
		})

	case grepEditorFinishedMsg:
		// Stay on grep page.
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
			case "ctrl+f":
				return m.enterGrep()
			}
		case pageConversations:
			switch key.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				return m.exitConversations()
			case "enter":
				return m.openConversation()
			case "ctrl+f":
				return m.enterGrep()
			case "ctrl+r":
				return m.resumeConversation()
			case "ctrl+y":
				return m.yankConversationSessionID()
			}
		case pageGrep:
			switch key.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				return m.exitGrep()
			case "enter":
				return m.openGrepResult()
			case "ctrl+g":
				return m.toggleGrepScope()
			case "ctrl+r":
				return m.resumeGrepResult()
			case "ctrl+y":
				return m.yankGrepSessionID()
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
			m.conversationsStatus = ""
		}
		return m, cmd

	case pageGrep:
		prevQuery := m.grepWidget.Query()
		var cmd tea.Cmd
		m.grepWidget, cmd = m.grepWidget.Update(msg)
		if m.grepWidget.Query() != prevQuery {
			m.grepStatus = ""
			cmd = tea.Batch(cmd, m.startGrepDebounce())
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
	m.convProjectDir = p.Dir
	m.convProjectCwd = p.Cwd
	m.allConversations = nil
	*m.convDisplayRef = nil
	*m.convQueryRef = ""
	m.convWidget.SetItemCount(1)
	m.convWidget.SetSelected(0)
	m.convWidget.SetQuery("")
	m.conversationsErr = nil
	m.conversationsStatus = ""
	m.conversationsLoading = true
	return m, loadConversationsCmd(p.Dir)
}

func (m Model) exitConversations() (tea.Model, tea.Cmd) {
	m.page = pageProjects
	return m, nil
}

func (m Model) openConversation() (tea.Model, tea.Cmd) {
	display := *m.convDisplayRef
	sel := m.convWidget.Selected()
	if len(display) == 0 || sel >= len(display) {
		return m, nil
	}
	conv := display[sel]
	return m, exportConvCmd(conv)
}

func (m Model) resumeConversation() (tea.Model, tea.Cmd) {
	display := *m.convDisplayRef
	sel := m.convWidget.Selected()
	if len(display) == 0 || sel >= len(display) {
		return m, nil
	}
	conv := display[sel]
	if conv.SessionID == "" {
		m.conversationsErr = errors.New("conversation has no session ID to resume")
		return m, nil
	}
	return m, resumeSessionCmd(conv.SessionID, m.convProjectCwd)
}

func (m Model) yankConversationSessionID() (tea.Model, tea.Cmd) {
	display := *m.convDisplayRef
	sel := m.convWidget.Selected()
	if len(display) == 0 || sel >= len(display) {
		return m, nil
	}
	conv := display[sel]
	if conv.SessionID == "" {
		m.conversationsStatus = "no session ID to copy"
		return m, nil
	}
	if err := platform.CopyText(conv.SessionID); err != nil {
		m.conversationsStatus = "failed to copy session ID"
		return m, nil
	}
	m.conversationsStatus = "copied session ID: " + conv.SessionID
	return m, nil
}

func (m Model) enterGrep() (tea.Model, tea.Cmd) {
	m.grepFromPage = m.page
	m.page = pageGrep
	m.grepResults = nil
	*m.grepResultsRef = nil
	m.grepErr = nil
	m.grepStatus = ""
	m.grepRunning = false
	m.grepSeq++
	m.grepWidget.SetQuery("")
	m.grepWidget.SetItemCount(1)
	m.grepWidget.SetSelected(0)

	if m.grepFromPage == pageConversations && m.convProjectDir != "" {
		m.grepScope = grepScopeProject
		m.grepScopeProjDir = m.convProjectDir
	} else {
		m.grepScope = grepScopeGlobal
	}
	return m, nil
}

func (m Model) exitGrep() (tea.Model, tea.Cmd) {
	m.page = m.grepFromPage
	return m, nil
}

func (m Model) toggleGrepScope() (tea.Model, tea.Cmd) {
	if m.grepScope == grepScopeGlobal {
		if m.convProjectDir != "" {
			m.grepScope = grepScopeProject
			m.grepScopeProjDir = m.convProjectDir
		}
	} else {
		m.grepScope = grepScopeGlobal
	}
	// Re-run the current query with the new scope.
	m.grepSeq++
	m.grepResults = nil
	*m.grepResultsRef = nil
	m.grepWidget.SetItemCount(1)
	m.grepWidget.SetSelected(0)
	query := m.grepWidget.Query()
	if len([]rune(query)) >= 3 {
		m.grepRunning = true
		seq := m.grepSeq
		dirs := m.grepDirs()
		return m, func() tea.Msg {
			results, err := claude.GrepTranscripts(query, dirs)
			return grepResultsMsg{results: results, err: err, seq: seq}
		}
	}
	return m, nil
}

func (m Model) openGrepResult() (tea.Model, tea.Cmd) {
	if len(m.grepResults) == 0 {
		return m, nil
	}
	sel := m.grepWidget.Selected()
	if sel >= len(m.grepResults) {
		return m, nil
	}
	result := m.grepResults[sel]
	query := m.grepWidget.Query()
	editor := resolveEditor()
	return m, func() tea.Msg {
		md, err := claude.ExportMarkdown(result.FilePath)
		if err != nil {
			return grepExportedMsg{err: err}
		}
		f, err := os.CreateTemp("", "claude-grep-*.md")
		if err != nil {
			return grepExportedMsg{err: err}
		}
		if _, err := f.WriteString(md); err != nil {
			f.Close()
			return grepExportedMsg{err: err}
		}
		f.Close()
		return grepExportedMsg{path: f.Name(), query: query, editor: editor}
	}
}

func (m Model) resumeGrepResult() (tea.Model, tea.Cmd) {
	if len(m.grepResults) == 0 {
		return m, nil
	}
	sel := m.grepWidget.Selected()
	if sel >= len(m.grepResults) {
		return m, nil
	}
	result := m.grepResults[sel]
	if result.SessionID == "" {
		m.grepErr = errors.New("grep result has no session ID to resume")
		return m, nil
	}
	cwd, ok := m.projectCwdForFile(result.FilePath)
	if !ok {
		m.grepErr = errors.New("could not find project for grep result")
		return m, nil
	}
	return m, resumeSessionCmd(result.SessionID, cwd)
}

func (m Model) yankGrepSessionID() (tea.Model, tea.Cmd) {
	if len(m.grepResults) == 0 {
		return m, nil
	}
	sel := m.grepWidget.Selected()
	if sel >= len(m.grepResults) {
		return m, nil
	}
	result := m.grepResults[sel]
	if result.SessionID == "" {
		m.grepStatus = "no session ID to copy"
		return m, nil
	}
	if err := platform.CopyText(result.SessionID); err != nil {
		m.grepStatus = "failed to copy session ID"
		return m, nil
	}
	m.grepStatus = "copied session ID: " + result.SessionID
	return m, nil
}

// projectCwdForFile returns the Cwd of the project containing filePath, by
// matching filePath's parent directory against m.allProjects.
func (m Model) projectCwdForFile(filePath string) (string, bool) {
	dir := filepath.Dir(filePath)
	for _, p := range m.allProjects {
		if p.Dir == dir {
			return p.Cwd, true
		}
	}
	return "", false
}

// resumeSessionCmd suspends the TUI and runs `claude --resume <sessionID>`
// in cwd, returning to the TUI when it exits.
func resumeSessionCmd(sessionID, cwd string) tea.Cmd {
	cmd := exec.Command("claude", "--resume", sessionID)
	cmd.Dir = cwd
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return resumeFinishedMsg{err: err}
	})
}

func (m *Model) startGrepDebounce() tea.Cmd {
	m.grepSeq++
	seq := m.grepSeq
	query := m.grepWidget.Query()
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return grepDebounceMsg{seq: seq, query: query}
	}
}

func (m *Model) grepDirs() []string {
	if m.grepScope == grepScopeProject && m.grepScopeProjDir != "" {
		return []string{m.grepScopeProjDir}
	}
	dirs := make([]string, len(m.allProjects))
	for i, p := range m.allProjects {
		dirs[i] = p.Dir
	}
	return dirs
}

func buildEditorCmd(editor, path, query string) *exec.Cmd {
	bin := strings.Fields(editor)[0]
	isNvim := strings.Contains(filepath.Base(bin), "nvim")
	if isNvim && query != "" {
		return exec.Command(bin, "+/"+query, path)
	}
	return exec.Command(bin, path)
}

func exportConvCmd(conv claude.Conversation) tea.Cmd {
	return func() tea.Msg {
		md, err := claude.ExportMarkdown(conv.Path)
		if err != nil {
			return convExportedMsg{err: err}
		}
		f, err := os.CreateTemp("", "claude-history-*.md")
		if err != nil {
			return convExportedMsg{err: err}
		}
		if _, err := f.WriteString(md); err != nil {
			f.Close()
			return convExportedMsg{err: err}
		}
		f.Close()
		return convExportedMsg{path: f.Name()}
	}
}

func resolveEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	for _, e := range []string{"nvim", "vi"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return "vi"
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
			footer := "type: filter  ↑/↓: move  enter: open  ctrl+f: grep  ctrl+r: resume  ctrl+y: yank session id  esc: back"
			if m.conversationsStatus != "" {
				footer = m.conversationsStatus + "  " + footer
			}
			m.convWidget.SetFooter(footer)
			content = m.convWidget.View()
		}
	case pageGrep:
		content = m.viewGrepPage()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) viewGrepPage() string {
	if m.rgNotFound {
		return errorStyle.Render("rg not found — brew install ripgrep")
	}
	if m.grepErr != nil {
		return errorStyle.Render("Error: " + m.grepErr.Error())
	}

	scopeLabel := "global"
	if m.grepScope == grepScopeProject {
		scopeLabel = "project"
	}
	footer := fmt.Sprintf("scope: %s  type: search  ↑/↓: move  enter: open  ctrl+g: toggle scope  ctrl+r: resume  ctrl+y: yank session id  esc: back", scopeLabel)
	if m.grepStatus != "" {
		footer = m.grepStatus + "  " + footer
	}
	if m.grepRunning {
		footer = "searching…  " + footer
	}
	m.grepWidget.SetFooter(footer)

	widgetContent := m.grepWidget.View()
	preview := m.renderGrepPreview()
	return widgetContent + "\n" + preview
}

func (m Model) renderGrepPreview() string {
	boxHeight := max(m.grepPreviewBoxHeight, 3)
	box := previewStyle.Width(max(m.width, 20)).Height(boxHeight)

	if len(m.grepResults) == 0 {
		query := m.grepWidget.Query()
		if query == "" {
			return box.Render("Type to search across transcripts")
		}
		if len([]rune(query)) < 3 {
			return box.Render("Enter at least 3 characters")
		}
		if m.grepRunning {
			return box.Render("Searching…")
		}
		return box.Render("No results")
	}

	sel := m.grepWidget.Selected()
	if sel >= len(m.grepResults) {
		return box.Render("")
	}

	r := m.grepResults[sel]
	text := r.Preview
	if text == "" {
		text = r.Snippet
	}

	// content lines = box height minus top/bottom border.
	contentHeight := max(boxHeight-2, 1)
	bodyMaxLines := contentHeight

	titleLine := ""
	if r.ConvTitle != "" {
		titleLine = fuzzyfinder.SubtitleStyle.Render("conversation: " + r.ConvTitle)
		bodyMaxLines = max(contentHeight-1, 0)
	}

	if r.SessionID != "" {
		sessionLine := fuzzyfinder.SubtitleStyle.Render("session: " + r.SessionID)
		bodyMaxLines = max(bodyMaxLines-1, 0)
		if titleLine != "" {
			titleLine += "\n" + sessionLine
		} else {
			titleLine = sessionLine
		}
	}

	w := max(m.width-4, 20)
	lines := wrapText(text, w)
	if len(lines) > bodyMaxLines {
		lines = lines[:bodyMaxLines]
	}

	content := strings.Join(lines, "\n")
	if titleLine != "" {
		if content != "" {
			content = titleLine + "\n" + content
		} else {
			content = titleLine
		}
	}
	return box.Render(content)
}

// renderGrepRow renders a single grep result row.
func renderGrepRow(resultsRef *[]claude.GrepResult, widthRef *int, i int, selected bool) string {
	results := *resultsRef
	if len(results) == 0 || i >= len(results) {
		return ""
	}

	r := results[i]
	plain := lipgloss.NewStyle()

	projLabel := claude.GrepResultProjectLabel(r.FilePath)
	projStr := fuzzyfinder.Highlight(projLabel+" · ", nil, fuzzyfinder.SubtitleStyle, selected)

	convTitle := r.ConvTitle
	if convTitle == "" {
		convTitle = filepath.Base(r.FilePath)
	}
	convStr := fuzzyfinder.Highlight(convTitle+" · ", nil, fuzzyfinder.SubtitleStyle, selected)

	// Truncate snippet to fit terminal width.
	// Widget overhead: border+padding (4) + gutter (2) = 6 columns.
	snippet := r.Snippet
	hl := r.SnippetHL
	termWidth := *widthRef
	if termWidth > 0 {
		prefixLen := len([]rune(projLabel)) + 3 + len([]rune(convTitle)) + 3
		available := termWidth - prefixLen - 6
		snippetRunes := []rune(snippet)
		if available <= 0 {
			snippet = ""
			hl = nil
		} else if len(snippetRunes) > available {
			cut := max(available-1, 0)
			snippet = string(snippetRunes[:cut]) + "…"
			var trimmedHL []int
			for _, idx := range hl {
				if idx < cut {
					trimmedHL = append(trimmedHL, idx)
				}
			}
			hl = trimmedHL
		}
	}

	snippetStr := fuzzyfinder.Highlight(snippet, hl, plain, selected)

	return projStr + convStr + snippetStr
}

// wrapText wraps text to at most width runes per line (naïve word wrap).
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
