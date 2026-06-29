package fuzzyfinder

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// press sends a single named key to the model and returns the updated model.
func press(t *testing.T, m Model, key string) Model {
	t.Helper()
	var msg tea.KeyPressMsg
	switch key {
	case "up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown}
	case "ctrl+k":
		msg = tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	case "ctrl+j":
		msg = tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}
	default:
		if len(key) == 1 {
			r := rune(key[0])
			msg = tea.KeyPressMsg{Code: r, Text: key}
		}
	}
	next, _ := m.Update(msg)
	return next
}

// newTestModel builds a Model with n items and height h.
func newTestModel(n, h int) Model {
	m := New(Config{
		ItemCount: n,
		RenderRow: func(i int, _ bool) string { return "" },
		Footer:    "test",
	})
	m.SetSize(80, h)
	return m
}

func TestNavDownMovesSelection(t *testing.T) {
	m := newTestModel(3, 20)
	if m.Selected() != 0 {
		t.Fatalf("expected initial selection 0, got %d", m.Selected())
	}
	m = press(t, m, "down")
	if m.Selected() != 1 {
		t.Errorf("after one down: expected 1, got %d", m.Selected())
	}
	m = press(t, m, "down")
	if m.Selected() != 2 {
		t.Errorf("after two downs: expected 2, got %d", m.Selected())
	}
	// At last item — must not move further.
	m = press(t, m, "down")
	if m.Selected() != 2 {
		t.Errorf("past last item: expected 2, got %d", m.Selected())
	}
}

func TestNavUpMovesSelection(t *testing.T) {
	m := newTestModel(3, 20)
	m.SetSelected(2)

	m = press(t, m, "up")
	if m.Selected() != 1 {
		t.Errorf("after one up: expected 1, got %d", m.Selected())
	}
	m = press(t, m, "up")
	if m.Selected() != 0 {
		t.Errorf("after two ups: expected 0, got %d", m.Selected())
	}
	// At first item — must not move further.
	m = press(t, m, "up")
	if m.Selected() != 0 {
		t.Errorf("past first item: expected 0, got %d", m.Selected())
	}
}

func TestCtrlJCtrlKAliases(t *testing.T) {
	m := newTestModel(3, 20)

	m = press(t, m, "ctrl+j")
	if m.Selected() != 1 {
		t.Errorf("ctrl+j: expected 1, got %d", m.Selected())
	}
	m = press(t, m, "ctrl+k")
	if m.Selected() != 0 {
		t.Errorf("ctrl+k: expected 0, got %d", m.Selected())
	}
}

// Height 7 → visibleRows = 7-5 = 2.
func TestViewportScrollDown(t *testing.T) {
	const h = 7
	m := newTestModel(5, h)

	if got := m.visibleRows(); got != 2 {
		t.Fatalf("expected 2 visible rows at height %d, got %d", h, got)
	}

	m = press(t, m, "down") // selected=1, 1 < 0+2 → offset stays 0
	if m.offset != 0 {
		t.Errorf("offset after 1 down: expected 0, got %d", m.offset)
	}

	m = press(t, m, "down") // selected=2, 2 >= 0+2 → offset=1
	if m.offset != 1 {
		t.Errorf("offset after 2 downs: expected 1, got %d", m.offset)
	}

	m = press(t, m, "down") // selected=3, 3 >= 1+2 → offset=2
	if m.offset != 2 {
		t.Errorf("offset after 3 downs: expected 2, got %d", m.offset)
	}
}

func TestViewportScrollUp(t *testing.T) {
	const h = 7
	m := newTestModel(5, h)
	m.SetSelected(4) // offset = 4-2+1 = 3

	if m.offset != 3 {
		t.Fatalf("offset after SetSelected(4): expected 3, got %d", m.offset)
	}

	m = press(t, m, "up") // selected=3, 3 >= offset(3) → offset unchanged
	if m.offset != 3 {
		t.Errorf("offset after 1 up: expected 3, got %d", m.offset)
	}

	m = press(t, m, "up") // selected=2, 2 < offset(3) → offset=2
	if m.offset != 2 {
		t.Errorf("offset after 2 ups: expected 2, got %d", m.offset)
	}
}

func TestSetItemCountClampsSelection(t *testing.T) {
	m := newTestModel(5, 20)
	m.SetSelected(4)

	m.SetItemCount(3) // selected 4 → clamped to 2
	if m.Selected() != 2 {
		t.Errorf("after SetItemCount(3): expected selected=2, got %d", m.Selected())
	}
}

func TestMatchRangesExact(t *testing.T) {
	ranges, ok := MatchRanges("foo", "foobar")
	if !ok {
		t.Fatal("expected match")
	}
	if len(ranges) < 3 {
		t.Fatalf("expected ≥3 ranges, got %v", ranges)
	}
	if ranges[0] != 0 || ranges[1] != 1 || ranges[2] != 2 {
		t.Errorf("unexpected match positions: %v", ranges)
	}
}

func TestMatchRangesSkip(t *testing.T) {
	ranges, ok := MatchRanges("fb", "foobar")
	if !ok {
		t.Fatal("expected match")
	}
	found0, found3 := false, false
	for _, r := range ranges {
		if r == 0 {
			found0 = true
		}
		if r == 3 {
			found3 = true
		}
	}
	if !found0 || !found3 {
		t.Errorf("expected positions 0 and 3 in ranges, got %v", ranges)
	}
}

func TestMatchRangesNoMatch(t *testing.T) {
	_, ok := MatchRanges("xyz", "foobar")
	if ok {
		t.Error("expected no match for 'xyz' in 'foobar'")
	}
}

func TestMatchRangesEmptyQuery(t *testing.T) {
	ranges, ok := MatchRanges("", "foobar")
	if !ok {
		t.Error("empty query should match everything")
	}
	if ranges != nil {
		t.Errorf("empty query should return nil ranges, got %v", ranges)
	}
}

func TestQueryReflectsTyping(t *testing.T) {
	m := newTestModel(3, 20)
	if m.Query() != "" {
		t.Fatalf("expected empty query, got %q", m.Query())
	}
	for _, r := range "hello" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if m.Query() != "hello" {
		t.Errorf("expected query 'hello', got %q", m.Query())
	}
}
