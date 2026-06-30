package claudehistory

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/claude"
)

func testProjects() []claude.Project {
	return []claude.Project{
		{Label: "myproject", Subtitle: "~/work/myproject", Cwd: "/home/alice/work/myproject", Dir: "/home/alice/.claude/projects/myproject"},
		{Label: "otherproject", Subtitle: "~/work/otherproject", Cwd: "/home/alice/work/otherproject", Dir: "/home/alice/.claude/projects/otherproject"},
		{Label: "blf", Subtitle: "~/dev/blf", Cwd: "/home/alice/dev/blf", Dir: "/home/alice/.claude/projects/blf"},
	}
}

func testConversations() []claude.Conversation {
	now := time.Now()
	return []claude.Conversation{
		{Title: "Fix the bug", SessionID: "s1", LastAccessed: now.Add(-1 * time.Hour)},
		{Title: "Add a feature", SessionID: "s2", LastAccessed: now.Add(-2 * time.Hour)},
		{Title: "Refactor code", SessionID: "s3", LastAccessed: now.Add(-3 * time.Hour)},
	}
}

// modelPress sends a key to the model and returns the updated model.
func modelPress(m Model, key string) Model {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEsc}
	case "ctrl+c":
		msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+f":
		msg = tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
	case "ctrl+g":
		msg = tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	case "up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		if len(key) == 1 {
			r := rune(key[0])
			msg = tea.KeyPressMsg{Code: r, Text: key}
		}
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

// modelType types a string query into the model.
func modelType(m Model, query string) Model {
	for _, r := range query {
		m = modelPress(m, string(r))
	}
	return m
}

// loadProjects injects projects into the model via projectsLoadedMsg and sets a
// default window size so rows are visible in View().
func loadProjects(m Model, projects []claude.Project) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	next, _ = m.Update(projectsLoadedMsg{projects: projects})
	return next.(Model)
}

// loadConversations injects conversations into the model via conversationsLoadedMsg.
func loadConversations(m Model, convs []claude.Conversation) Model {
	next, _ := m.Update(conversationsLoadedMsg{conversations: convs})
	return next.(Model)
}

func viewStr(m Model) string {
	v := m.View()
	return v.Content
}

func TestHistoryModelProjectsLoaded(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())

	display := *m.displayRef
	if len(display) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(display))
	}
}

func TestHistoryModelViewShowsProjects(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())

	view := viewStr(m)
	if !strings.Contains(view, "myproject") {
		t.Errorf("view should contain 'myproject', got:\n%s", view)
	}
	if !strings.Contains(view, "otherproject") {
		t.Errorf("view should contain 'otherproject', got:\n%s", view)
	}
}

func TestHistoryModelFuzzyFilter(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelType(m, "blf")

	display := *m.displayRef
	if len(display) != 1 {
		t.Fatalf("expected 1 match for 'blf', got %d", len(display))
	}
	if display[0].Label != "blf" {
		t.Errorf("expected blf, got %q", display[0].Label)
	}
}

func TestHistoryModelFuzzyNoMatch(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelType(m, "zzznomatch")

	display := *m.displayRef
	if len(display) != 0 {
		t.Errorf("expected 0 matches, got %d", len(display))
	}
}

func TestHistoryModelNavigation(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())

	if m.widget.Selected() != 0 {
		t.Fatalf("expected initial selection 0, got %d", m.widget.Selected())
	}
	m = modelPress(m, "down")
	if m.widget.Selected() != 1 {
		t.Errorf("after down: expected 1, got %d", m.widget.Selected())
	}
	m = modelPress(m, "up")
	if m.widget.Selected() != 0 {
		t.Errorf("after up: expected 0, got %d", m.widget.Selected())
	}
}

func TestHistoryModelEmptyState(t *testing.T) {
	m := New("")
	m = loadProjects(m, nil)

	view := viewStr(m)
	if !strings.Contains(view, "No projects found") {
		t.Errorf("empty state should show 'No projects found', got:\n%s", view)
	}
}

func TestHistoryModelProjectsErrorShown(t *testing.T) {
	m := New("")
	next, _ := m.Update(projectsLoadedMsg{err: fmt.Errorf("disk error")})
	m = next.(Model)

	view := viewStr(m)
	if !strings.Contains(view, "Error") {
		t.Errorf("error state should show 'Error', got:\n%s", view)
	}
}

func TestHistoryModelEnterTransitionsToConversations(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	if m.page != pageConversations {
		t.Errorf("expected pageConversations after enter, got %d", m.page)
	}
}

func TestHistoryModelEscFromConversationsGoesBack(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	if m.page != pageConversations {
		t.Fatalf("expected pageConversations after enter, got %d", m.page)
	}
	m = modelPress(m, "esc")
	if m.page != pageProjects {
		t.Errorf("expected pageProjects after esc, got %d", m.page)
	}
}

func TestHistoryModelConversationsLoaded(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	m = loadConversations(m, testConversations())

	display := *m.convDisplayRef
	if len(display) != 3 {
		t.Fatalf("expected 3 conversations, got %d", len(display))
	}
}

func TestHistoryModelConversationsViewShowsTitles(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	// set window size so rows render
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m = loadConversations(m, testConversations())

	view := viewStr(m)
	if !strings.Contains(view, "Fix the bug") {
		t.Errorf("view should contain 'Fix the bug', got:\n%s", view)
	}
	if !strings.Contains(view, "Add a feature") {
		t.Errorf("view should contain 'Add a feature', got:\n%s", view)
	}
}

func TestHistoryModelConversationsEmptyState(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	m = loadConversations(m, nil)

	view := viewStr(m)
	if !strings.Contains(view, "No conversations found") {
		t.Errorf("empty state should show 'No conversations found', got:\n%s", view)
	}
}

func TestHistoryModelConversationsLoadingState(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter") // loading starts, no msg injected yet

	view := viewStr(m)
	if !strings.Contains(view, "Loading...") {
		t.Errorf("loading state should show 'Loading...', got:\n%s", view)
	}
}

func TestHistoryModelConversationsFuzzyFilter(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	m = loadConversations(m, testConversations())
	m = modelType(m, "bug")

	display := *m.convDisplayRef
	if len(display) != 1 {
		t.Fatalf("expected 1 match for 'bug', got %d", len(display))
	}
	if display[0].Title != "Fix the bug" {
		t.Errorf("expected 'Fix the bug', got %q", display[0].Title)
	}
}

func TestHistoryModelViewAltScreen(t *testing.T) {
	m := New("")
	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true on view")
	}
}

// ---- Grep page tests ----

func TestHistoryModelCtrlFFromProjectsOpensGlobalGrep(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "ctrl+f")

	if m.page != pageGrep {
		t.Fatalf("expected pageGrep after ctrl+f, got %d", m.page)
	}
	if m.grepScope != grepScopeGlobal {
		t.Errorf("expected grepScopeGlobal from projects page, got %d", m.grepScope)
	}
	if m.grepFromPage != pageProjects {
		t.Errorf("expected grepFromPage=pageProjects, got %d", m.grepFromPage)
	}
}

func TestHistoryModelCtrlFFromConversationsOpensProjectGrep(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter") // go to conversations
	m = loadConversations(m, testConversations())
	m = modelPress(m, "ctrl+f")

	if m.page != pageGrep {
		t.Fatalf("expected pageGrep after ctrl+f, got %d", m.page)
	}
	if m.grepScope != grepScopeProject {
		t.Errorf("expected grepScopeProject from conversations page, got %d", m.grepScope)
	}
	if m.grepFromPage != pageConversations {
		t.Errorf("expected grepFromPage=pageConversations, got %d", m.grepFromPage)
	}
}

func TestHistoryModelGrepEscReturnsToOrigin(t *testing.T) {
	// From projects.
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "ctrl+f")
	if m.page != pageGrep {
		t.Fatalf("expected pageGrep")
	}
	m = modelPress(m, "esc")
	if m.page != pageProjects {
		t.Errorf("expected pageProjects after esc from grep, got %d", m.page)
	}
}

func TestHistoryModelGrepCtrlGTogglesScope(t *testing.T) {
	// Start from conversations so we have a project dir for toggle.
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "enter")
	m = loadConversations(m, testConversations())
	m = modelPress(m, "ctrl+f") // open grep in project scope

	if m.grepScope != grepScopeProject {
		t.Fatalf("expected grepScopeProject initially, got %d", m.grepScope)
	}

	m = modelPress(m, "ctrl+g") // toggle → global
	if m.grepScope != grepScopeGlobal {
		t.Errorf("expected grepScopeGlobal after ctrl+g, got %d", m.grepScope)
	}

	m = modelPress(m, "ctrl+g") // toggle → project again
	if m.grepScope != grepScopeProject {
		t.Errorf("expected grepScopeProject after second ctrl+g, got %d", m.grepScope)
	}
}

func TestHistoryModelGrepViewShowsSearchPrompt(t *testing.T) {
	m := New("")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m = loadProjects(m, testProjects())
	m = modelPress(m, "ctrl+f")

	view := viewStr(m)
	// Grep page should show something (at minimum the empty search prompt).
	if view == "" {
		t.Error("grep page view should not be empty")
	}
}

func TestHistoryModelGrepResultsInjected(t *testing.T) {
	m := New("")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m = loadProjects(m, testProjects())
	m = modelPress(m, "ctrl+f")

	// Inject grep results directly.
	seq := m.grepSeq
	results := []claude.GrepResult{
		{FilePath: "/p/a.jsonl", ConvTitle: "Fibonacci session", Snippet: "fibonacci sequence", SnippetHL: []int{0, 1, 2}},
		{FilePath: "/p/b.jsonl", ConvTitle: "Sort session", Snippet: "sort algorithm", SnippetHL: nil},
	}
	next, _ = m.Update(grepResultsMsg{results: results, err: nil, seq: seq})
	m = next.(Model)

	if len(m.grepResults) != 2 {
		t.Fatalf("expected 2 grep results, got %d", len(m.grepResults))
	}

	view := viewStr(m)
	if !strings.Contains(view, "fibonacci") {
		t.Errorf("grep view should contain 'fibonacci', got:\n%s", view)
	}
}

func TestHistoryModelGrepStaleResultsIgnored(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	m = modelPress(m, "ctrl+f")

	// seq is m.grepSeq; inject with a different seq.
	staleSeq := m.grepSeq - 1
	results := []claude.GrepResult{
		{FilePath: "/p/a.jsonl", ConvTitle: "Stale", Snippet: "stale result"},
	}
	next, _ := m.Update(grepResultsMsg{results: results, err: nil, seq: staleSeq})
	m = next.(Model)

	if len(m.grepResults) != 0 {
		t.Errorf("stale results should be ignored, got %d", len(m.grepResults))
	}
}

func TestHistoryModelGrepRgNotFoundShownInView(t *testing.T) {
	m := New("")
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	m = loadProjects(m, testProjects())
	m = modelPress(m, "ctrl+f")

	seq := m.grepSeq
	next, _ = m.Update(grepResultsMsg{err: claude.ErrRgNotFound, seq: seq})
	m = next.(Model)

	if !m.rgNotFound {
		t.Error("expected rgNotFound to be set")
	}
	view := viewStr(m)
	if !strings.Contains(view, "rg not found") {
		t.Errorf("view should mention rg not found, got:\n%s", view)
	}
}
