package history_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elentok/blf/internal/launcher/history"
)

func TestAppend_dedup_movesToFront(t *testing.T) {
	h := history.New()
	h.Append("a")
	h.Append("b")
	h.Append("c")
	h.Append("b") // should move "b" to front
	got := h.Entries()
	want := []string{"b", "c", "a"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("[%d] got %q want %q", i, got[i], v)
		}
	}
}

func TestAppend_ignoresEmpty(t *testing.T) {
	h := history.New()
	h.Append("")
	h.Append("   ")
	if h.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", h.Len())
	}
}

func TestAppend_cap(t *testing.T) {
	h := history.New()
	for i := range history.MaxEntries + 10 {
		h.Append("entry-" + string(rune('0'+i/100%10)) + string(rune('0'+i/10%10)) + string(rune('0'+i%10)))
	}
	if h.Len() != history.MaxEntries {
		t.Fatalf("len=%d want %d", h.Len(), history.MaxEntries)
	}
}

func TestEntries_returnsCopy(t *testing.T) {
	h := history.New()
	h.Append("x")
	e := h.Entries()
	e[0] = "mutated"
	if h.Entries()[0] != "x" {
		t.Fatal("Entries() should return a copy, not the internal slice")
	}
}

func TestLoadSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "launcher-history")

	h := history.New()
	h.Append("one")
	h.Append("two")
	h.Append("three")

	if err := h.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	h2 := history.Load(path)
	got := h2.Entries()
	want := []string{"three", "two", "one"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("[%d] got %q want %q", i, got[i], v)
		}
	}
}

func TestLoad_missingFile(t *testing.T) {
	h := history.Load("/nonexistent/path/launcher-history")
	if h == nil || h.Len() != 0 {
		t.Fatal("Load of missing file should return empty history")
	}
}

func TestLoad_capEnforced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "launcher-history")
	var sb []byte
	for i := range history.MaxEntries + 5 {
		sb = append(sb, []byte(string(rune('a'+i%26))+"\n")...)
	}
	if err := os.WriteFile(path, sb, 0o600); err != nil {
		t.Fatal(err)
	}
	h := history.Load(path)
	if h.Len() != history.MaxEntries {
		t.Fatalf("len=%d want %d", h.Len(), history.MaxEntries)
	}
}
