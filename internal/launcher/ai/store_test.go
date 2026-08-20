package ai_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elentok/blf/internal/launcher/ai"
)

func run(id string) ai.Run {
	return ai.Run{
		ID:        id,
		Timestamp: time.Unix(1700000000, 0).UTC(),
		Kind:      ai.KindAI,
		Input:     "input-" + id,
		Response:  "response-" + id,
		Status:    ai.StatusSuccess,
	}
}

func ids(runs []ai.Run) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.ID
	}
	return out
}

func assertIDs(t *testing.T, got []ai.Run, want []string) {
	t.Helper()
	gotIDs := ids(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(gotIDs), len(want), gotIDs)
	}
	for i, v := range want {
		if gotIDs[i] != v {
			t.Errorf("[%d] got %q want %q", i, gotIDs[i], v)
		}
	}
}

func TestAppend_mostRecentFirst(t *testing.T) {
	s := ai.NewStore()
	s.Append(run("a"))
	s.Append(run("b"))
	s.Append(run("c"))
	assertIDs(t, s.Runs(), []string{"c", "b", "a"})
}

func TestAppend_cap(t *testing.T) {
	s := ai.NewStore()
	for i := range ai.MaxRuns + 10 {
		s.Append(run(string(rune('a' + i%26))))
	}
	if s.Len() != ai.MaxRuns {
		t.Fatalf("len=%d want %d", s.Len(), ai.MaxRuns)
	}
}

func TestRuns_returnsCopy(t *testing.T) {
	s := ai.NewStore()
	s.Append(run("a"))
	got := s.Runs()
	got[0].ID = "mutated"
	if s.Runs()[0].ID != "a" {
		t.Fatal("Runs() should return a copy, not the internal slice")
	}
}

func TestDelete_removesExactlyOne(t *testing.T) {
	s := ai.NewStore()
	s.Append(run("a"))
	s.Append(run("b"))
	s.Append(run("c"))
	if !s.Delete("b") {
		t.Fatal("expected Delete to report a run was removed")
	}
	assertIDs(t, s.Runs(), []string{"c", "a"})
}

func TestDelete_missingID_returnsFalse(t *testing.T) {
	s := ai.NewStore()
	s.Append(run("a"))
	if s.Delete("missing") {
		t.Fatal("expected Delete to report false for a missing id")
	}
	if s.Len() != 1 {
		t.Fatalf("len=%d want 1", s.Len())
	}
}

func TestLoadSave_roundtrip_preservesFieldsAndOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "ai-runs.jsonl")

	s := ai.NewStore()
	s.Append(run("one"))
	s.Append(ai.Run{
		ID:        "two",
		Timestamp: time.Unix(1700000100, 0).UTC(),
		Kind:      ai.KindImprove,
		Input:     "fix this",
		Response:  "fixed this",
		Status:    ai.StatusFailure,
	})
	s.Append(run("three"))

	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2 := ai.LoadStore(path)
	got := s2.Runs()
	want := []ai.Run{
		run("three"),
		{
			ID:        "two",
			Timestamp: time.Unix(1700000100, 0).UTC(),
			Kind:      ai.KindImprove,
			Input:     "fix this",
			Response:  "fixed this",
			Status:    ai.StatusFailure,
		},
		run("one"),
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %+v", len(got), len(want), got)
	}
	for i, v := range want {
		if !got[i].Timestamp.Equal(v.Timestamp) || got[i].ID != v.ID || got[i].Kind != v.Kind ||
			got[i].Input != v.Input || got[i].Response != v.Response || got[i].Status != v.Status {
			t.Errorf("[%d] got %+v want %+v", i, got[i], v)
		}
	}
}

func TestSave_rewritesWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai-runs.jsonl")

	s := ai.NewStore()
	for i := range 5 {
		s.Append(run(string(rune('a' + i))))
	}
	if err := s.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s2 := ai.LoadStore(path)
	s2.Delete("c")
	if err := s2.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s3 := ai.LoadStore(path)
	if s3.Len() != 4 {
		t.Fatalf("len=%d want 4, file should reflect the rewritten (deleted) state, not an appended line", s3.Len())
	}
}

func TestLoadStore_missingFile(t *testing.T) {
	s := ai.LoadStore("/nonexistent/path/ai-runs.jsonl")
	if s == nil || s.Len() != 0 {
		t.Fatal("LoadStore of missing file should return an empty store")
	}
}

func TestLoadStore_corruptFile_returnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai-runs.jsonl")
	if err := os.WriteFile(path, []byte("not json\nnot json either\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := ai.LoadStore(path)
	if s.Len() != 0 {
		t.Fatalf("expected empty store from corrupt file, got %d runs: %+v", s.Len(), s.Runs())
	}
}

func TestLoadStore_capEnforced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai-runs.jsonl")
	// Write more than MaxRuns lines directly (bypassing Append's own cap) to
	// verify LoadStore enforces the cap independently.
	var sb []byte
	for i := range ai.MaxRuns + 5 {
		line, err := json.Marshal(run(string(rune('a' + i%26))))
		if err != nil {
			t.Fatal(err)
		}
		sb = append(sb, line...)
		sb = append(sb, '\n')
	}
	if err := os.WriteFile(path, sb, 0o600); err != nil {
		t.Fatal(err)
	}
	s := ai.LoadStore(path)
	if s.Len() != ai.MaxRuns {
		t.Fatalf("len=%d want %d", s.Len(), ai.MaxRuns)
	}
}
