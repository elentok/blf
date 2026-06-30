package apps_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elentok/blf/internal/launcher/apps"
)

func TestLaunchArgsMac(t *testing.T) {
	app := apps.App{Name: "Safari", Path: "/Applications/Safari.app"}
	got := apps.LaunchArgsMac(app)
	want := []string{"open", "-a", "/Applications/Safari.app"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLaunchArgsLinux(t *testing.T) {
	app := apps.App{Name: "Firefox", Path: "/usr/share/applications/firefox.desktop"}
	got := apps.LaunchArgsLinux(app)
	want := []string{"gio", "launch", "/usr/share/applications/firefox.desktop"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apps.json")
	original := &apps.Index{
		Apps: []apps.App{
			{Name: "Safari", Path: "/Applications/Safari.app"},
			{Name: "Firefox", Path: "/Applications/Firefox.app"},
		},
		IndexedAt: time.Unix(1000, 0).UTC(),
	}
	if err := apps.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := apps.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Apps) != 2 {
		t.Fatalf("got %d apps, want 2", len(loaded.Apps))
	}
	if loaded.Apps[0].Name != "Safari" {
		t.Errorf("app[0].Name: got %q, want Safari", loaded.Apps[0].Name)
	}
	if loaded.Apps[1].Path != "/Applications/Firefox.app" {
		t.Errorf("app[1].Path: got %q, want /Applications/Firefox.app", loaded.Apps[1].Path)
	}
}

func TestLoadMissing(t *testing.T) {
	idx, err := apps.Load("/nonexistent/apps.json")
	if err != nil {
		t.Fatalf("Load of missing file should return empty index, got err: %v", err)
	}
	if len(idx.Apps) != 0 {
		t.Errorf("expected empty apps, got %d", len(idx.Apps))
	}
}

func TestScanMacDirs(t *testing.T) {
	dir := t.TempDir()
	// Create synthetic .app bundles
	for _, name := range []string{"Safari.app", "Firefox.app", "not-an-app.txt"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	result := apps.ScanMacDirs([]string{dir})
	if len(result) != 2 {
		t.Fatalf("got %d apps, want 2: %v", len(result), result)
	}
	names := map[string]bool{}
	for _, a := range result {
		names[a.Name] = true
	}
	if !names["Safari"] {
		t.Error("expected Safari in results")
	}
	if !names["Firefox"] {
		t.Error("expected Firefox in results")
	}
}

func TestScanMacDirsSameNameDifferentDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir1, "Safari.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir2, "Safari.app"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Same name in two different dirs: both should appear (dedup is by path only).
	result := apps.ScanMacDirs([]string{dir1, dir2})
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d: %v", len(result), result)
	}
	paths := map[string]bool{}
	for _, a := range result {
		paths[a.Path] = true
	}
	if !paths[filepath.Join(dir1, "Safari.app")] || !paths[filepath.Join(dir2, "Safari.app")] {
		t.Errorf("expected both Safari paths, got %v", result)
	}
}

func TestScanMacDirsRecursive(t *testing.T) {
	root := t.TempDir()
	// Top-level app: no subtitle.
	if err := os.Mkdir(filepath.Join(root, "Safari.app"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Nested app: /<root>/Utilities/Activity Monitor.app
	utils := filepath.Join(root, "Utilities")
	if err := os.Mkdir(utils, 0o755); err != nil {
		t.Fatal(err)
	}
	monitor := filepath.Join(utils, "Activity Monitor.app")
	if err := os.Mkdir(monitor, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file inside the bundle must not be treated as a separate entry / recursed into.
	if err := os.Mkdir(filepath.Join(monitor, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hidden directory must be skipped.
	if err := os.Mkdir(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden", "Secret.app"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := apps.ScanMacDirs([]string{root})
	got := map[string]apps.App{}
	for _, a := range result {
		got[a.Name] = a
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 apps (Safari, Activity Monitor), got %d: %v", len(result), result)
	}
	if got["Safari"].Subtitle != "" {
		t.Errorf("top-level app should have no subtitle, got %q", got["Safari"].Subtitle)
	}
	am, ok := got["Activity Monitor"]
	if !ok {
		t.Fatalf("expected nested Activity Monitor in results: %v", result)
	}
	if am.Path != monitor {
		t.Errorf("Activity Monitor path: got %q, want %q", am.Path, monitor)
	}
	if am.Subtitle != "Utilities" {
		t.Errorf("nested app subtitle: got %q, want %q", am.Subtitle, "Utilities")
	}
}

func TestScanLinuxDirs(t *testing.T) {
	dir := t.TempDir()
	writeDesktop(t, dir, "firefox.desktop", "Firefox")
	writeDesktop(t, dir, "gedit.desktop", "gedit")
	// Empty Name= should be skipped
	if err := os.WriteFile(filepath.Join(dir, "bad.desktop"), []byte("[Desktop Entry]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := apps.ScanLinuxDirs([]string{dir})
	if len(result) != 2 {
		t.Fatalf("got %d apps, want 2: %v", len(result), result)
	}
}

func TestParseDesktopName(t *testing.T) {
	dir := t.TempDir()
	path := writeDesktop(t, dir, "test.desktop", "My App")
	name := apps.ParseDesktopName(path)
	if name != "My App" {
		t.Errorf("got %q, want %q", name, "My App")
	}
}

func writeDesktop(t *testing.T, dir, filename, name string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	content := "[Desktop Entry]\nName=" + name + "\nExec=/usr/bin/app\nType=Application\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
