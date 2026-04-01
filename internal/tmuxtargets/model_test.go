package tmuxtargets

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestViewColorsNonSelectedTargetsBlue(t *testing.T) {
	lines := []string{"a https://one.test b https://two.test"}
	targets := []target{
		{line: 0, start: 2, end: 18, text: "https://one.test", openable: true, openTarget: "https://one.test"},
		{line: 0, start: 21, end: 37, text: "https://two.test", openable: true, openTarget: "https://two.test"},
	}

	m := newModel(lines, targets, func(string) {})
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
		[]target{{text: "alpha"}, {text: "beta"}, {text: "gamma"}},
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
		[]target{{line: 0, start: 0, text: "alpha"}, {line: 1, start: 0, text: "alpine"}, {line: 2, start: 0, text: "beta"}},
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
		[]target{
			{line: 0, start: 0, text: "a"},
			{line: 0, start: 2, text: "b"},
			{line: 1, start: 0, text: "c"},
		},
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
		[]target{
			{line: 0, start: 0, text: "a"},
			{line: 0, start: 2, text: "b"},
			{line: 1, start: 0, text: "c"},
		},
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
		[]target{{text: "alpha"}, {text: "beta"}},
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
		[]target{{text: "alpha"}, {text: "beta"}},
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
	targets := []target{
		{line: 1, start: 10, end: 26, text: "https://one.test", openable: true, openTarget: "https://one.test"},
		{line: 2, start: 10, end: 26, text: "https://two.test", openable: true, openTarget: "https://two.test"},
	}
	m := newModel(lines, targets, func(string) {})
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
		[]target{{line: 0, start: 9, end: 25, text: "https://one.test", openable: true, openTarget: "https://one.test"}},
		func(string) {},
	)

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	m = m2.(model)
	if !m.helpMode {
		t.Fatal("expected helpMode=true after ?")
	}
	v := m.View()
	if !strings.Contains(v.Content, "Tmux Targets Help") {
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
		[]target{{line: 0, start: 0, end: 20, kind: kindResumeCommand, text: "codex resume abc123"}},
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
		[]target{{line: 0, start: 0, end: 20, kind: kindResumeCommand, text: "codex resume abc123"}},
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
		[]target{{line: 0, start: 0, end: 8, text: "deadbeef", openable: false}},
		func(string) {},
	)

	m2, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	m = m2.(model)
	v := m.View()
	if !strings.Contains(v.Content, "selected target is not openable") {
		t.Fatalf("expected bottom bar notification in view content: %q", v.Content)
	}
}
