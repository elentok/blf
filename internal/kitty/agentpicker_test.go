package kitty

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
	m := newAgentPickerModel(testPickerAgents()) // input: idle, waiting, working
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
	m := newAgentPickerModel(testPickerAgents())
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
	m := newAgentPickerModel(testPickerAgents())
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
	m := newAgentPickerModel(testPickerAgents())
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
	m := newAgentPickerModel(testPickerAgents())
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
	m := newAgentPickerModel(agents)
	m = pickerPress(m, "enter")
	if m.selectedID != 42 {
		t.Errorf("expected selectedID=42, got %d", m.selectedID)
	}
}

func TestAgentPickerEnterSelectsFirstVisibleAfterSort(t *testing.T) {
	// After sort, waiting (ID 2) is first.
	m := newAgentPickerModel(testPickerAgents())
	m = pickerPress(m, "enter")
	if m.selectedID != 2 {
		t.Errorf("expected selectedID=2 (waiting agent), got %d", m.selectedID)
	}
}

func TestAgentPickerEnterOnEmptyDoesNotSelect(t *testing.T) {
	m := newAgentPickerModel(nil)
	m = pickerPress(m, "enter")
	if m.selectedID != 0 {
		t.Errorf("expected selectedID=0 for empty state, got %d", m.selectedID)
	}
}

func TestAgentPickerEmptyStateViewDoesNotPanic(t *testing.T) {
	m := newAgentPickerModel(nil)
	_ = m.View()
}

func TestAgentPickerNavigation(t *testing.T) {
	m := newAgentPickerModel(testPickerAgents())
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
	m := newAgentPickerModel(testPickerAgents())
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
	m := newAgentPickerModel(testPickerAgents())
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
