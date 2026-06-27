package launcher

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// typeText feeds each rune to the model as a key press, returning the updated model.
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		msg := tea.KeyPressMsg{Code: r, Text: string(r)}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

// runCmd executes a tea.Cmd (if non-nil) and returns its message.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestEscResetsAndHides(t *testing.T) {
	hidden := false
	m := NewModel(ModelConfig{
		Providers:    []Provider{CalcProvider{}},
		HideTerminal: func() error { hidden = true; return nil },
		HideDelay:    0,
	})

	m = typeText(t, m, "1+1")
	if m.input.Value() != "1+1" {
		t.Fatalf("expected input %q, got %q", "1+1", m.input.Value())
	}
	if len(m.results) == 0 {
		t.Fatal("expected results before esc")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)

	if m.input.Value() != "" {
		t.Errorf("expected input reset, got %q", m.input.Value())
	}
	if m.results != nil {
		t.Errorf("expected results cleared, got %d", len(m.results))
	}
	if m.selected != 0 || m.offset != 0 {
		t.Errorf("expected selection reset, got selected=%d offset=%d", m.selected, m.offset)
	}

	runCmd(cmd)
	if !hidden {
		t.Error("expected HideTerminal to be called")
	}
}

func TestSyncEnterResetsAndHides(t *testing.T) {
	hidden := false
	copied := ""
	m := NewModel(ModelConfig{
		Providers:    []Provider{CalcProvider{}},
		CopyText:     func(s string) error { copied = s; return nil },
		HideTerminal: func() error { hidden = true; return nil },
		HideDelay:    0,
	})

	m = typeText(t, m, "2*3")
	if len(m.results) == 0 {
		t.Fatal("expected calc result")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if copied == "" {
		t.Error("expected calc value copied to clipboard")
	}
	if m.input.Value() != "" {
		t.Errorf("expected input reset, got %q", m.input.Value())
	}
	if m.results != nil {
		t.Errorf("expected results cleared, got %d", len(m.results))
	}

	runCmd(cmd)
	if !hidden {
		t.Error("expected HideTerminal to be called")
	}
}

func TestResetAndHideNilHideTerminal(t *testing.T) {
	m := NewModel(ModelConfig{HideDelay: 0})
	m.input.SetValue("stale")
	m.status = "oops"

	cmd := m.resetAndHide()

	if m.input.Value() != "" {
		t.Errorf("expected input reset, got %q", m.input.Value())
	}
	if m.status != "" {
		t.Errorf("expected status cleared, got %q", m.status)
	}
	if cmd != nil {
		t.Error("expected nil cmd when HideTerminal is unset")
	}
}
