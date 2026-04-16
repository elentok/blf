package targets

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestViewColorsNonSelectedTargetsBlue(t *testing.T) {
	lines := []string{"a https://one.test b https://two.test"}
	tgts := []Target{
		{Line: 0, Start: 2, End: 18, Text: "https://one.test", Openable: true, OpenTarget: "https://one.test"},
		{Line: 0, Start: 21, End: 37, Text: "https://two.test", Openable: true, OpenTarget: "https://two.test"},
	}

	m := newModel(lines, tgts, "Test", func(string) {})
	v := m.View()

	if !strings.Contains(v.Content, selectedStyle.Render("https://one.test")) {
		t.Fatalf("expected selected target styling in view content: %q", v.Content)
	}
	if !strings.Contains(v.Content, targetStyle.Render("https://two.test")) {
		t.Fatalf("expected non-selected target blue styling in view content: %q", v.Content)
	}
}

func TestSearchTypingFiltersAndSelectsFirstMatch(t *testing.T) {
	m := newModel(
		[]string{"alpha beta gamma"},
		[]Target{{Text: "alpha"}, {Text: "beta"}, {Text: "gamma"}},
		"Test",
		func(string) {},
	)

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	m = m2.(model)
	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	m = m2.(model)

	if !m.searchMode {
		t.Fatal("expected search mode")
	}
	if m.query != "g" {
		t.Fatalf("query = %q", m.query)
	}
	if len(m.filteredIdx) != 1 || m.filteredIdx[0] != 2 {
		t.Fatalf("filteredIdx = %#v", m.filteredIdx)
	}
	if m.selected != 2 {
		t.Fatalf("selected = %d", m.selected)
	}
}

func TestSearchEnterLocksAndNavigatesFilteredOnly(t *testing.T) {
	m := newModel(
		[]string{"alpha", "alpine", "beta"},
		[]Target{{Line: 0, Start: 0, Text: "alpha"}, {Line: 1, Start: 0, Text: "alpine"}, {Line: 2, Start: 0, Text: "beta"}},
		"Test",
		func(string) {},
	)

	for _, k := range []tea.KeyPressMsg{
		tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}),
		tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}),
		tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}),
	} {
		m2, _ := m.Update(k)
		m = m2.(model)
	}

	if !m.filterLocked || m.searchMode {
		t.Fatalf("expected locked filtered mode, got locked=%v search=%v", m.filterLocked, m.searchMode)
	}
	if m.selected != 1 {
		t.Fatalf("selected = %d, want 1", m.selected)
	}
}

func TestVerticalMovementDoesNotWrapOrMoveHorizontally(t *testing.T) {
	m := newModel(
		[]string{"a b", "c"},
		[]Target{
			{Line: 0, Start: 0, Text: "a"},
			{Line: 0, Start: 2, Text: "b"},
			{Line: 1, Start: 0, Text: "c"},
		},
		"Test",
		func(string) {},
	)

	if m.selected != 0 {
		t.Fatalf("initial selected = %d, want 0", m.selected)
	}

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	m = m2.(model)
	if m.selected != 2 {
		t.Fatalf("selected after j = %d, want 2", m.selected)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	m = m2.(model)
	if m.selected != 2 {
		t.Fatalf("selected after second j = %d, want 2 (no wrap)", m.selected)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	m = m2.(model)
	if m.selected != 0 {
		t.Fatalf("selected after k = %d, want 0", m.selected)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "k", Code: 'k'}))
	m = m2.(model)
	if m.selected != 0 {
		t.Fatalf("selected after second k = %d, want 0 (no wrap)", m.selected)
	}
}

func TestHorizontalMovementStaysOnSameLineWithoutWrapping(t *testing.T) {
	m := newModel(
		[]string{"a b", "c"},
		[]Target{
			{Line: 0, Start: 0, Text: "a"},
			{Line: 0, Start: 2, Text: "b"},
			{Line: 1, Start: 0, Text: "c"},
		},
		"Test",
		func(string) {},
	)

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	m = m2.(model)
	if m.selected != 1 {
		t.Fatalf("selected after l = %d, want 1", m.selected)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	m = m2.(model)
	if m.selected != 1 {
		t.Fatalf("selected after second l = %d, want 1 (no wrap/down)", m.selected)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	m = m2.(model)
	if m.selected != 0 {
		t.Fatalf("selected after h = %d, want 0", m.selected)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	m = m2.(model)
	if m.selected != 0 {
		t.Fatalf("selected after second h = %d, want 0 (no wrap/up)", m.selected)
	}
}

func TestSearchEscClearsFilter(t *testing.T) {
	m := newModel(
		[]string{"alpha beta"},
		[]Target{{Text: "alpha"}, {Text: "beta"}},
		"Test",
		func(string) {},
	)

	for _, k := range []tea.KeyPressMsg{
		tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}),
		tea.KeyPressMsg(tea.Key{Text: "z", Code: 'z'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}),
	} {
		m2, _ := m.Update(k)
		m = m2.(model)
	}

	if m.query != "" || m.searchMode || m.filterLocked {
		t.Fatalf("expected cleared search state, got query=%q search=%v locked=%v", m.query, m.searchMode, m.filterLocked)
	}
	if m.selected != 0 {
		t.Fatalf("selected = %d, want 0", m.selected)
	}
}

func TestSearchNoMatchesClearsSelectionAndCopyNoops(t *testing.T) {
	var notes []string
	m := newModel(
		[]string{"alpha beta"},
		[]Target{{Text: "alpha"}, {Text: "beta"}},
		"Test",
		func(msg string) { notes = append(notes, msg) },
	)

	for _, k := range []tea.KeyPressMsg{
		tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}),
		tea.KeyPressMsg(tea.Key{Text: "z", Code: 'z'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}),
	} {
		m2, _ := m.Update(k)
		m = m2.(model)
	}

	if m.selected != -1 {
		t.Fatalf("selected = %d, want -1", m.selected)
	}
	if len(notes) == 0 || notes[len(notes)-1] != "no targets to copy" {
		t.Fatalf("unexpected notifications: %#v", notes)
	}
}

func TestSearchModeUsesMagentaStylesAndDrawsSearchBox(t *testing.T) {
	lines := []string{
		"row 0 no target",
		"row 1 has https://one.test",
		"row 2 has https://two.test",
		"row 3 no target",
		"row 4 no target",
		"row 5 no target",
		"row 6 no target",
	}
	tgts := []Target{
		{Line: 1, Start: 10, End: 26, Text: "https://one.test", Openable: true, OpenTarget: "https://one.test"},
		{Line: 2, Start: 10, End: 26, Text: "https://two.test", Openable: true, OpenTarget: "https://two.test"},
	}
	m := newModel(lines, tgts, "Test", func(string) {})
	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	m = m2.(model)
	v := m.View()

	if !strings.Contains(v.Content, searchTargetStyle.Render("https://two.test")) {
		t.Fatalf("expected green search target style in view content: %q", v.Content)
	}
	if !strings.Contains(v.Content, "Search: ") {
		t.Fatalf("expected search box text in view content: %q", v.Content)
	}
	if !strings.Contains(v.Content, "╭") || !strings.Contains(v.Content, "╯") {
		t.Fatalf("expected rounded search box border in view content: %q", v.Content)
	}
}

func TestHelpKeyOpensAndClosesHelpView(t *testing.T) {
	m := newModel(
		[]string{"row with https://one.test"},
		[]Target{{Line: 0, Start: 9, End: 25, Text: "https://one.test", Openable: true, OpenTarget: "https://one.test"}},
		"Test",
		func(string) {},
	)

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	m = m2.(model)
	if !m.helpMode {
		t.Fatal("expected helpMode=true after ?")
	}
	v := m.View()
	if !strings.Contains(v.Content, "Test Help") {
		t.Fatalf("expected help page content, got: %q", v.Content)
	}

	m2, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m = m2.(model)
	if m.helpMode {
		t.Fatal("expected helpMode=false after esc")
	}
}

func TestEnterOnResumeTargetRunsCommandAndQuits(t *testing.T) {
	var ran string
	m := newModel(
		[]string{"codex resume abc123"},
		[]Target{{Line: 0, Start: 0, End: 20, Kind: KindResumeCommand, Text: "codex resume abc123"}},
		"Test",
		func(string) {},
	)
	m.runResumeCmd = func(command string) error {
		ran = command
		return nil
	}

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if ran != "codex resume abc123" {
		t.Fatalf("ran = %q", ran)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestEnterOnResumeTargetShowsFailure(t *testing.T) {
	m := newModel(
		[]string{"codex resume abc123"},
		[]Target{{Line: 0, Start: 0, End: 20, Kind: KindResumeCommand, Text: "codex resume abc123"}},
		"Test",
		func(string) {},
	)
	m.runResumeCmd = func(string) error {
		return errors.New("boom")
	}

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = m2.(model)

	if m.status != "failed to run resume command" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestNonOpenableShowsInBottomBar(t *testing.T) {
	m := newModel(
		[]string{"deadbeef", "next line"},
		[]Target{{Line: 0, Start: 0, End: 8, Text: "deadbeef", Openable: false}},
		"Test",
		func(string) {},
	)

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	m = m2.(model)
	v := m.View()
	if !strings.Contains(v.Content, "selected target is not openable") {
		t.Fatalf("expected bottom bar notification in view content: %q", v.Content)
	}
}
