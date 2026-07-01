package launcher_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/directories"
)

func TestDirectoryProvider_filtersMissingAndNonDirectories(t *testing.T) {
	tmp := t.TempDir()
	realDir := filepath.Join(tmp, "Notes")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(realFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs := []directories.Directory{
		{Name: "Notes", Path: realDir},
		{Name: "Missing", Path: filepath.Join(tmp, "does-not-exist")},
		{Name: "NotADir", Path: realFile},
	}
	p := launcher.NewDirectoryProvider(dirs, 1.0)

	results := p.Query("Notes")
	if len(results) != 1 || results[0].Title != "Notes" {
		t.Fatalf("expected only 'Notes' to match, got %+v", results)
	}

	if got := p.Query("Missing"); len(got) != 0 {
		t.Errorf("expected 'Missing' to be filtered out, got %+v", got)
	}
	if got := p.Query("NotADir"); len(got) != 0 {
		t.Errorf("expected 'NotADir' to be filtered out, got %+v", got)
	}
}
