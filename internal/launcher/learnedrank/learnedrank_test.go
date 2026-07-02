package learnedrank_test

import (
	"path/filepath"
	"testing"

	"github.com/elentok/blf/internal/launcher/learnedrank"
)

func TestIncrement_thenCounts(t *testing.T) {
	s := learnedrank.New()
	s.Increment("foo", "app:foo")
	got := s.Counts("foo")
	want := map[string]int{"app:foo": 1}
	if len(got) != len(want) || got["app:foo"] != want["app:foo"] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestIncrement_accumulates(t *testing.T) {
	s := learnedrank.New()
	s.Increment("foo", "app:foo")
	s.Increment("foo", "app:foo")
	s.Increment("foo", "app:foo")
	got := s.Counts("foo")
	if got["app:foo"] != 3 {
		t.Fatalf("count=%d want 3", got["app:foo"])
	}
}

func TestIncrement_independentQueriesAndKeys(t *testing.T) {
	s := learnedrank.New()
	s.Increment("foo", "app:foo")
	s.Increment("foo", "app:bar")
	s.Increment("baz", "app:foo")

	gotFoo := s.Counts("foo")
	if gotFoo["app:foo"] != 1 || gotFoo["app:bar"] != 1 {
		t.Fatalf("foo counts = %v", gotFoo)
	}

	gotBaz := s.Counts("baz")
	if gotBaz["app:foo"] != 1 {
		t.Fatalf("baz counts = %v", gotBaz)
	}
	if len(gotBaz) != 1 {
		t.Fatalf("baz counts should have 1 entry, got %v", gotBaz)
	}
}

func TestIncrement_ignoresEmpty(t *testing.T) {
	s := learnedrank.New()
	s.Increment("", "app:foo")
	s.Increment("   ", "app:foo")
	s.Increment("foo", "")
	s.Increment("foo", "   ")

	if len(s.Counts("")) != 0 {
		t.Fatal("empty query should not create an entry")
	}
	if len(s.Counts("foo")) != 0 {
		t.Fatal("empty resultKey should not create an entry")
	}
}

func TestCounts_missingQuery_returnsEmptyNonNilMap(t *testing.T) {
	s := learnedrank.New()
	got := s.Counts("nonexistent")
	if got == nil {
		t.Fatal("Counts should never return nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestCounts_returnsCopy(t *testing.T) {
	s := learnedrank.New()
	s.Increment("foo", "app:foo")
	got := s.Counts("foo")
	got["app:foo"] = 999
	got["app:new"] = 1

	got2 := s.Counts("foo")
	if got2["app:foo"] != 1 {
		t.Fatal("Counts() should return a copy, not the internal map")
	}
	if _, ok := got2["app:new"]; ok {
		t.Fatal("mutating returned map should not affect internal state")
	}
}

func TestLoadSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "learned-rank")

	s := learnedrank.New()
	s.Increment("foo", "app:foo")
	s.Increment("foo", "app:foo")
	s.Increment("foo", "app:bar")
	s.Increment("baz qux", "script:baz")

	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2 := learnedrank.Load(path)

	gotFoo := s2.Counts("foo")
	wantFoo := map[string]int{"app:foo": 2, "app:bar": 1}
	if len(gotFoo) != len(wantFoo) || gotFoo["app:foo"] != 2 || gotFoo["app:bar"] != 1 {
		t.Fatalf("foo counts = %v want %v", gotFoo, wantFoo)
	}

	gotBaz := s2.Counts("baz qux")
	if gotBaz["script:baz"] != 1 || len(gotBaz) != 1 {
		t.Fatalf("baz qux counts = %v", gotBaz)
	}
}

func TestLoad_missingFile(t *testing.T) {
	s := learnedrank.Load("/nonexistent/path/learned-rank")
	if s == nil {
		t.Fatal("Load of missing file should return non-nil store")
	}
	got := s.Counts("anything")
	if got == nil || len(got) != 0 {
		t.Fatal("Load of missing file should return an empty store")
	}
}
