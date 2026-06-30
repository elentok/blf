package launcher

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher/history"
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
	if len(m.results) != 0 {
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
	if len(m.results) != 0 {
		t.Errorf("expected results cleared, got %d", len(m.results))
	}

	runCmd(cmd)
	if !hidden {
		t.Error("expected HideTerminal to be called")
	}
}

func TestCtrlXDeletesSelectedHistoryEntry(t *testing.T) {
	h := history.New()
	h.Append("alpha")
	h.Append("beta")
	h.Append("gamma") // most-recent first: gamma, beta, alpha
	m := NewModel(ModelConfig{History: h})

	// Empty input shows history rows; select the middle one (beta).
	m.recomputeResults()
	if len(m.results) != 3 {
		t.Fatalf("expected 3 history rows, got %d", len(m.results))
	}
	m.selected = 1
	if got := m.results[1].Action.Target; got != "beta" {
		t.Fatalf("expected selected row beta, got %q", got)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)

	for _, e := range h.Entries() {
		if e == "beta" {
			t.Fatal("expected beta removed from history")
		}
	}
	if len(m.results) != 2 {
		t.Errorf("expected 2 rows after delete, got %d", len(m.results))
	}
	if m.selected < 0 || m.selected >= len(m.results) {
		t.Errorf("selection out of bounds: %d", m.selected)
	}
}

func TestCtrlXIgnoredForNonHistoryRow(t *testing.T) {
	h := history.New()
	h.Append("noop")
	m := NewModel(ModelConfig{
		Providers: []Provider{CalcProvider{}},
		History:   h,
	})

	// Typing a calc query shows a calc result, not a history row.
	m = typeText(t, m, "1+1")
	if len(m.results) == 0 || m.results[0].Action.Type != ActionCopy {
		t.Fatalf("expected a calc (copy) result")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)

	if h.Len() != 1 {
		t.Errorf("expected history untouched, len=%d", h.Len())
	}
}

func TestResetShowsRecentHistory(t *testing.T) {
	h := history.New()
	h.Append("alpha")
	h.Append("beta")
	m := NewModel(ModelConfig{
		Providers:    []Provider{CalcProvider{}},
		History:      h,
		HideTerminal: func() error { return nil },
		HideDelay:    0,
	})

	// Type a query, then dismiss. The recent list must be ready for the next show,
	// since reopening the quick terminal emits no WindowSizeMsg to trigger a recompute.
	m = typeText(t, m, "1+1")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)

	// esc records the calc query into history, so 2 seeded + 1 = 3 recent rows.
	if len(m.results) != 3 {
		t.Fatalf("expected 3 recent rows after reset, got %d", len(m.results))
	}
	if m.results[0].Action.Type != ActionRecall {
		t.Errorf("expected recall rows, got %v", m.results[0].Action.Type)
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
