package herdrplugin

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateManifest(t *testing.T) {
	got := GenerateManifest("/opt/homebrew/bin/blf")

	want := `id = "blf"
name = "blf tab titles"
version = "1"
platforms = ["macos", "linux"]
min_herdr_version = "0.9.3"

[[events]]
on = "pane.focused"
command = ["/opt/homebrew/bin/blf", "herdr", "sync-tab-title"]

[[events]]
on = "tab.created"
command = ["/opt/homebrew/bin/blf", "herdr", "sync-tab-title"]

[[events]]
on = "pane.agent_detected"
command = ["/opt/homebrew/bin/blf", "herdr", "sync-tab-title"]

[[events]]
on = "pane.agent_status_changed"
command = ["/opt/homebrew/bin/blf", "herdr", "sync-tab-title"]

[[events]]
on = "pane.closed"
command = ["/opt/homebrew/bin/blf", "herdr", "sync-tab-title"]

[[startup]]
command = ["/opt/homebrew/bin/blf", "herdr", "sync-tab-title"]
`

	if got != want {
		t.Errorf("GenerateManifest() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateManifestEscapesPath(t *testing.T) {
	got := GenerateManifest(`C:\Program Files\blf"2".exe`)

	if !strings.Contains(got, `command = ["C:\\Program Files\\blf\"2\".exe", "herdr", "sync-tab-title"]`) {
		t.Errorf("GenerateManifest() did not escape path correctly:\n%s", got)
	}
}

func TestResolveBlfPathMatchingSymlink(t *testing.T) {
	got := ResolveBlfPath("/opt/homebrew/bin/blf", "/opt/homebrew/Cellar/blf/1.0/bin/blf", "/opt/homebrew/Cellar/blf/1.0/bin/blf")
	want := "/opt/homebrew/bin/blf"
	if got != want {
		t.Errorf("ResolveBlfPath() = %q, want %q", got, want)
	}
}

func TestResolveBlfPathNonMatchingSymlink(t *testing.T) {
	got := ResolveBlfPath("/usr/local/bin/blf", "/usr/local/bin/blf", "/home/david/dev/blf/blf")
	want := "/home/david/dev/blf/blf"
	if got != want {
		t.Errorf("ResolveBlfPath() = %q, want %q", got, want)
	}
}

func TestResolveBlfPathNoBlfOnPath(t *testing.T) {
	got := ResolveBlfPath("", "", "/home/david/dev/blf/blf")
	want := "/home/david/dev/blf/blf"
	if got != want {
		t.Errorf("ResolveBlfPath() = %q, want %q", got, want)
	}
}

func TestDirAndManifestPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	home := "/home/david"

	wantDir := filepath.Join(home, ".local", "share", "blf", "herdr-plugin")
	if got := Dir(home); got != wantDir {
		t.Errorf("Dir() = %q, want %q", got, wantDir)
	}

	wantManifest := filepath.Join(wantDir, "herdr-plugin.toml")
	if got := ManifestPath(home); got != wantManifest {
		t.Errorf("ManifestPath() = %q, want %q", got, wantManifest)
	}
}
