package beads

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stubLister is a fake IssueLister for model tests, so they don't shell out
// to bd.
type stubLister struct {
	issues []Issue
	err    error
}

func (s stubLister) List(scope Scope) ([]Issue, error) {
	return s.issues, s.err
}

func testIssues() []Issue {
	return []Issue{
		{ID: "abc-1", Title: "fix the bug", Status: "open"},
		{ID: "abc-2", Title: "add a feature", Status: "in_progress"},
		{ID: "abc-3", Title: "refactor code", Status: "closed"},
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
	default:
		if len(key) == 1 {
			r := rune(key[0])
			msg = tea.KeyPressMsg{Code: r, Text: key}
		}
	}
	next, cmd := m.Update(msg)
	_ = cmd
	return next.(Model)
}

func modelType(m Model, query string) Model {
	for _, r := range query {
		m = modelPress(m, string(r))
	}
	return m
}

// loadIssues sizes the model and injects issues directly via issuesLoadedMsg,
// mirroring how internal/claudehistory's tests skip running Init's real cmd.
func loadIssues(m Model, issues []Issue) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	next, _ = m.Update(issuesLoadedMsg{issues: issues})
	return next.(Model)
}

func TestEnterYieldsSelectedID(t *testing.T) {
	copied := ""
	m := NewModel(ModelConfig{
		Lister:   stubLister{issues: testIssues()},
		Scope:    ScopeActionable,
		CopyText: func(s string) error { copied = s; return nil },
	})
	m = loadIssues(m, testIssues())

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.SelectedID() != "abc-1" {
		t.Errorf("SelectedID() = %q, want %q", m.SelectedID(), "abc-1")
	}
	if copied != "abc-1" {
		t.Errorf("CopyText got %q, want %q", copied, "abc-1")
	}
	if cmd == nil {
		t.Error("expected a quit cmd after enter")
	}
}

func TestEnterAfterFilterYieldsFilteredSelection(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())

	m = modelType(m, "feature")
	if len(*m.displayRef) != 1 || (*m.displayRef)[0].ID != "abc-2" {
		t.Fatalf("expected filtered list to contain only abc-2, got %+v", *m.displayRef)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.SelectedID() != "abc-2" {
		t.Errorf("SelectedID() = %q, want %q", m.SelectedID(), "abc-2")
	}
}

func TestEnterOnEmptyListIsNoop(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: nil}})
	m = loadIssues(m, nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.SelectedID() != "" {
		t.Errorf("expected no selection, got %q", m.SelectedID())
	}
	if cmd != nil {
		t.Error("expected no cmd for enter on an empty list")
	}
}

func TestEscQuitsWithoutSelection(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a quit cmd for esc")
	}
}

func TestLoadErrorIsSurfaced(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{err: errors.New("bd exploded")}})
	next, _ := m.Update(issuesLoadedMsg{err: errors.New("bd exploded")})
	m = next.(Model)

	if m.loadErr == nil {
		t.Error("expected loadErr to be set")
	}
}

func TestEmptyScopeShowsEmptyState(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: nil}})
	m = loadIssues(m, nil)

	if len(m.allItems) != 0 {
		t.Fatalf("expected no items, got %d", len(m.allItems))
	}
}
