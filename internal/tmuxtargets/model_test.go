package tmuxtargets

import (
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
		[]string{"alpha alpine beta"},
		[]target{{text: "alpha"}, {text: "alpine"}, {text: "beta"}},
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
