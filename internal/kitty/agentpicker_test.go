package kitty

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func testPickerAgents() []Agent {
	return []Agent{
		{ID: 1, Status: StatusIdle, Name: "claude", Dir: "alpha", Title: "Task A"},
		{ID: 2, Status: StatusWaiting, Name: "codex", Dir: "beta", Title: "Task B"},
		{ID: 3, Status: StatusWorking, Name: "claude", Dir: "gamma", Title: "Task C"},
	}
}

// pickerPress sends a named key to the model and returns the updated model.
func pickerPress(m agentPickerModel, key string) agentPickerModel {
	var msg tea.KeyPressMsg
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
	case "?":
		msg = tea.KeyPressMsg{Code: '?', Text: "?"}
	default:
		if len(key) == 1 {
			r := rune(key[0])
			msg = tea.KeyPressMsg{Code: r, Text: key}
		}
	}
	next, _ := m.Update(msg)
	return next.(agentPickerModel)
}

// pickerType types a string query into the model.
func pickerType(m agentPickerModel, query string) agentPickerModel {
	for _, r := range query {
		msg := tea.KeyPressMsg{Code: r, Text: string(r)}
		next, _ := m.Update(msg)
		m = next.(agentPickerModel)
	}
	return m
}

func TestAgentPickerStatusFirstOrder(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{}) // input: idle, waiting, working
	display := *m.displayRef
	if len(display) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(display))
	}
	if display[0].Status != StatusWaiting {
		t.Errorf("first should be waiting, got %s", display[0].Status)
	}
	if display[1].Status != StatusWorking {
		t.Errorf("second should be working, got %s", display[1].Status)
	}
	if display[2].Status != StatusIdle {
		t.Errorf("third should be idle, got %s", display[2].Status)
	}
}

func TestAgentPickerFuzzyFilterByDir(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	m = pickerType(m, "alpha")
	display := *m.displayRef
	if len(display) != 1 {
		t.Fatalf("expected 1 match for 'alpha', got %d", len(display))
	}
	if display[0].ID != 1 {
		t.Errorf("expected agent ID 1, got %d", display[0].ID)
	}
}

func TestAgentPickerFuzzyFilterByTitle(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	m = pickerType(m, "Task B")
	display := *m.displayRef
	if len(display) != 1 {
		t.Fatalf("expected 1 match for 'Task B', got %d", len(display))
	}
	if display[0].ID != 2 {
		t.Errorf("expected agent ID 2, got %d", display[0].ID)
	}
}

func TestAgentPickerFuzzyFilterNoMatch(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	m = pickerType(m, "zzznomatch")
	display := *m.displayRef
	if len(display) != 0 {
		t.Errorf("expected 0 matches, got %d", len(display))
	}
}

func pickerBackspace(m agentPickerModel) agentPickerModel {
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	return next.(agentPickerModel)
}

func TestAgentPickerClearFilterRestoresAll(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	m = pickerType(m, "alpha")
	for range 5 {
		m = pickerBackspace(m)
	}
	display := *m.displayRef
	if len(display) != 3 {
		t.Errorf("expected 3 agents after clearing query, got %d", len(display))
	}
}

func TestAgentPickerEnterRecordsID(t *testing.T) {
	agents := []Agent{{ID: 42, Status: StatusIdle, Name: "claude", Dir: "proj", Title: "task"}}
	m := newAgentPickerModel(agents, Deps{})
	m = pickerPress(m, "enter")
	if m.selectedID != 42 {
		t.Errorf("expected selectedID=42, got %d", m.selectedID)
	}
}

func TestAgentPickerEnterSelectsFirstVisibleAfterSort(t *testing.T) {
	// After sort, waiting (ID 2) is first.
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	m = pickerPress(m, "enter")
	if m.selectedID != 2 {
		t.Errorf("expected selectedID=2 (waiting agent), got %d", m.selectedID)
	}
}

func TestAgentPickerEnterOnEmptyDoesNotSelect(t *testing.T) {
	m := newAgentPickerModel(nil, Deps{})
	m = pickerPress(m, "enter")
	if m.selectedID != 0 {
		t.Errorf("expected selectedID=0 for empty state, got %d", m.selectedID)
	}
}

func TestAgentPickerEmptyStateViewDoesNotPanic(t *testing.T) {
	m := newAgentPickerModel(nil, Deps{})
	_ = m.View()
}

func TestAgentPickerNavigation(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	if m.widget.Selected() != 0 {
		t.Fatalf("expected initial selection 0, got %d", m.widget.Selected())
	}
	m = pickerPress(m, "down")
	if m.widget.Selected() != 1 {
		t.Errorf("after down: expected 1, got %d", m.widget.Selected())
	}
	m = pickerPress(m, "up")
	if m.widget.Selected() != 0 {
		t.Errorf("after up: expected 0, got %d", m.widget.Selected())
	}
}

func TestAgentPickerEnterAfterNavSelectsCorrectAgent(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	// Move to second item (working, ID 3 after sort: waiting=2, working=3, idle=1)
	m = pickerPress(m, "down")
	display := *m.displayRef
	wantID := display[1].ID // second in sorted order
	m = pickerPress(m, "enter")
	if m.selectedID != wantID {
		t.Errorf("expected selectedID=%d, got %d", wantID, m.selectedID)
	}
}

func TestAgentPickerHelpModeToggle(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	if m.helpMode {
		t.Error("expected helpMode=false initially")
	}
	m = pickerPress(m, "?")
	if !m.helpMode {
		t.Error("expected helpMode=true after ?")
	}
	m = pickerPress(m, "?")
	if m.helpMode {
		t.Error("expected helpMode=false after second ?")
	}
}

func TestAgentPickerIDStableSelectionOnRefresh(t *testing.T) {
	// Start: sorted order is waiting(2), working(3), idle(1).
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	// Move selection to working agent (index 1, ID 3).
	m = pickerPress(m, "down")
	if m.highlightedAgentID != 3 {
		t.Fatalf("expected highlighted ID 3, got %d", m.highlightedAgentID)
	}

	// Data refresh: agent 3 now becomes waiting, so it sorts to index 0.
	refreshed := []Agent{
		{ID: 1, Status: StatusIdle, Name: "claude", Dir: "alpha", Title: "Task A"},
		{ID: 2, Status: StatusWaiting, Name: "codex", Dir: "beta", Title: "Task B"},
		{ID: 3, Status: StatusWaiting, Name: "claude", Dir: "gamma", Title: "Task C"},
	}
	next, _ := m.Update(agentDataTickMsg{agents: refreshed})
	m = next.(agentPickerModel)

	// Agent ID 3 should still be selected (now at a different index).
	if m.highlightedAgentID != 3 {
		t.Errorf("after refresh: expected highlighted ID 3, got %d", m.highlightedAgentID)
	}
	display := *m.displayRef
	gotIdx := m.widget.Selected()
	if gotIdx >= len(display) || display[gotIdx].ID != 3 {
		t.Errorf("widget selection should point to agent 3, got idx=%d display=%v", gotIdx, display)
	}
}

func TestAgentPickerWorkingToWaitingResortToTop(t *testing.T) {
	// Start: waiting(2), working(3), idle(1).
	m := newAgentPickerModel(testPickerAgents(), Deps{})

	// Data refresh: agent 3 transitions working→waiting.
	refreshed := []Agent{
		{ID: 1, Status: StatusIdle, Name: "claude", Dir: "alpha", Title: "Task A"},
		{ID: 2, Status: StatusWaiting, Name: "codex", Dir: "beta", Title: "Task B"},
		{ID: 3, Status: StatusWaiting, Name: "claude", Dir: "gamma", Title: "Task C"},
	}
	next, _ := m.Update(agentDataTickMsg{agents: refreshed})
	m = next.(agentPickerModel)

	display := *m.displayRef
	if len(display) < 2 {
		t.Fatalf("expected at least 2 display agents, got %d", len(display))
	}
	// Both waiting agents should be at the top (indices 0 and 1).
	for i := 0; i < 2; i++ {
		if display[i].Status != StatusWaiting {
			t.Errorf("display[%d] should be waiting after working→waiting transition, got %s", i, display[i].Status)
		}
	}
}

func TestRenderPreviewFitsFrame(t *testing.T) {
	const width = 40
	const height = 10

	// Build fake preview content wider and taller than the frame.
	var sb strings.Builder
	for i := range height * 3 {
		fmt.Fprintf(&sb, "line %03d: %s\n", i, strings.Repeat("x", width*2))
	}

	m := newAgentPickerModel(testPickerAgents(), Deps{})
	m.previewText = sb.String()

	out := m.renderPreview(width, height)

	lines := strings.Split(out, "\n")
	// lipgloss may append a trailing empty line; strip it for the count check.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if got := len(lines); got != height {
		t.Errorf("renderPreview height = %d lines, want %d\noutput:\n%s", got, height, out)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got > width {
			t.Errorf("line %d width = %d, want ≤ %d: %q", i, got, width, line)
		}
	}
}

func TestAgentPickerSpinnerFrameAdvances(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents(), Deps{})
	initial := *m.spinner.frameRef

	next, _ := m.Update(spinnerTickMsg{})
	m = next.(agentPickerModel)

	after := *m.spinner.frameRef
	expected := (initial + 1) % len(arcSpinnerFrames)
	if after != expected {
		t.Errorf("expected spinner frame %d, got %d", expected, after)
	}
}
