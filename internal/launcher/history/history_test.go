package history_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/elentok/blf/internal/launcher/history"
)

// actionTypeLaunch mirrors launcher.ActionLaunch's iota value (1) without
// importing the launcher package (would create an import cycle).
const actionTypeLaunch = 1

func copyEntry(label string) history.Entry {
	return history.Entry{Label: label, ActionType: history.ActionTypeCopy}
}

func launchEntry(label, target string) history.Entry {
	return history.Entry{Label: label, ActionType: actionTypeLaunch, Target: target}
}

func labels(entries []history.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Label
	}
	return out
}

func assertLabels(t *testing.T, got []history.Entry, want []string) {
	t.Helper()
	gotLabels := labels(got)
	if len(gotLabels) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(gotLabels), len(want), gotLabels)
	}
	for i, v := range want {
		if gotLabels[i] != v {
			t.Errorf("[%d] got %q want %q", i, gotLabels[i], v)
		}
	}
}

func TestAppend_copyEntries_dedupByLabel_movesToFront(t *testing.T) {
	h := history.New()
	h.Append(copyEntry("a"))
	h.Append(copyEntry("b"))
	h.Append(copyEntry("c"))
	h.Append(copyEntry("b")) // should move "b" to front
	assertLabels(t, h.Entries(), []string{"b", "c", "a"})
}

func TestAppend_launchEntries_dedupByActionTargetNotLabel(t *testing.T) {
	h := history.New()
	h.Append(launchEntry("kit", "/Applications/kitty.app"))
	h.Append(launchEntry("other", "/Applications/other.app"))
	// Same (ActionType, Target) as the first entry, different Label (e.g. a
	// different query resolved to the same app): should move to front and
	// adopt the new Label, not duplicate.
	h.Append(launchEntry("Kitty", "/Applications/kitty.app"))
	got := h.Entries()
	assertLabels(t, got, []string{"Kitty", "other"})
	if got[0].Target != "/Applications/kitty.app" {
		t.Errorf("target=%q want /Applications/kitty.app", got[0].Target)
	}
}

func TestAppend_ignoresEmptyLabel(t *testing.T) {
	h := history.New()
	h.Append(copyEntry(""))
	h.Append(copyEntry("   "))
	if h.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", h.Len())
	}
}

func TestAppend_cap(t *testing.T) {
	h := history.New()
	for i := range history.MaxEntries + 10 {
		h.Append(copyEntry("entry-" + string(rune('0'+i/100%10)) + string(rune('0'+i/10%10)) + string(rune('0'+i%10))))
	}
	if h.Len() != history.MaxEntries {
		t.Fatalf("len=%d want %d", h.Len(), history.MaxEntries)
	}
}

func TestEntries_returnsCopy(t *testing.T) {
	h := history.New()
	h.Append(copyEntry("x"))
	e := h.Entries()
	e[0].Label = "mutated"
	if h.Entries()[0].Label != "x" {
		t.Fatal("Entries() should return a copy, not the internal slice")
	}
}

func TestRemove_launchEntry_matchesByActionAndTarget_notLabel(t *testing.T) {
	h := history.New()
	h.Append(launchEntry("Same Name", "/Applications/a.app"))
	h.Append(launchEntry("Same Name", "/Applications/b.app"))
	if !h.Remove(launchEntry("Same Name", "/Applications/a.app")) {
		t.Fatal("expected Remove to report an entry was removed")
	}
	got := h.Entries()
	if len(got) != 1 || got[0].Target != "/Applications/b.app" {
		t.Fatalf("got %+v, want only the b.app entry to remain", got)
	}
}

func TestRemove_copyEntry_matchesByLabel(t *testing.T) {
	h := history.New()
	h.Append(copyEntry("10+20"))
	h.Append(copyEntry("1$"))
	if !h.Remove(copyEntry("10+20")) {
		t.Fatal("expected Remove to report an entry was removed")
	}
	assertLabels(t, h.Entries(), []string{"1$"})
}

func TestLoadSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "launcher-history")

	h := history.New()
	h.Append(copyEntry("one"))
	h.Append(launchEntry("Kitty", "/Applications/kitty.app"))
	h.Append(copyEntry("three"))

	if err := h.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	h2 := history.Load(path)
	got := h2.Entries()
	want := []history.Entry{
		copyEntry("three"),
		launchEntry("Kitty", "/Applications/kitty.app"),
		copyEntry("one"),
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %+v", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("[%d] got %+v want %+v", i, got[i], v)
		}
	}
}

func TestLoad_missingFile(t *testing.T) {
	h := history.Load("/nonexistent/path/launcher-history")
	if h == nil || h.Len() != 0 {
		t.Fatal("Load of missing file should return empty history")
	}
}

func TestLoad_preUpgradePlainTextFile_returnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "launcher-history")
	// Pre-upgrade format: one bare query per line, not JSON.
	if err := os.WriteFile(path, []byte("kit\nfoo\nbar\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := history.Load(path)
	if h.Len() != 0 {
		t.Fatalf("expected empty history from unparseable legacy file, got %d entries: %+v", h.Len(), h.Entries())
	}
}

func TestLoad_capEnforced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "launcher-history")
	// Write more than MaxEntries lines directly (bypassing Append's own cap)
	// to verify Load() enforces the cap independently.
	var sb []byte
	for i := range history.MaxEntries + 5 {
		line, err := json.Marshal(copyEntry(string(rune('a' + i%26))))
		if err != nil {
			t.Fatal(err)
		}
		sb = append(sb, line...)
		sb = append(sb, '\n')
	}
	if err := os.WriteFile(path, sb, 0o600); err != nil {
		t.Fatal(err)
	}
	h := history.Load(path)
	if h.Len() != history.MaxEntries {
		t.Fatalf("len=%d want %d", h.Len(), history.MaxEntries)
	}
}
