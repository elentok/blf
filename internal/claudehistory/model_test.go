package claudehistory

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/claude"
)

func testProjects() []claude.Project {
	return []claude.Project{
		{Label: "myproject", Subtitle: "~/work/myproject", Cwd: "/home/alice/work/myproject"},
		{Label: "otherproject", Subtitle: "~/work/otherproject", Cwd: "/home/alice/work/otherproject"},
		{Label: "blf", Subtitle: "~/dev/blf", Cwd: "/home/alice/dev/blf"},
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

func TestHistoryModelEnterIsNoOp(t *testing.T) {
	m := New("")
	m = loadProjects(m, testProjects())
	// Enter should not panic and model should remain on projects page.
	m = modelPress(m, "enter")
	if m.page != pageProjects {
		t.Errorf("expected pageProjects after enter stub, got %d", m.page)
	}
}

func TestHistoryModelViewAltScreen(t *testing.T) {
	m := New("")
	v := m.View()
	if !v.AltScreen {
		t.Error("expected AltScreen=true on view")
	}
}
