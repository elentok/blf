package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/herdr"
	"github.com/elentok/blf/internal/herdrplugin"
)

// pluginTestDeps returns a deps set up with real filesystem operations
// rooted at a fresh temp "home" dir (XDG_DATA_HOME cleared so
// herdrplugin.Dir falls back to <home>/.local/share/blf/herdr-plugin), plus
// a fake command runner for the herdr CLI and a fake socket client.
func pluginTestDeps(t *testing.T, home string, client herdr.Client, lookPathErr error, runCommand func(string, ...string) ([]byte, error), env map[string]string) (deps, *strings.Builder) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", "")
	stderr := &strings.Builder{}

	return deps{
		stderr: stderr,
		lookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		lookPath: func(name string) (string, error) {
			if name == "herdr" {
				if lookPathErr != nil {
					return "", lookPathErr
				}
				return "/usr/local/bin/herdr", nil
			}
			return "", errors.New("blf not found")
		},
		evalSymlinks:   func(path string) (string, error) { return path, nil },
		executablePath: func() (string, error) { return "/home/david/dev/blf/blf", nil },
		runCommand:     runCommand,
		fileExists:     fileExists,
		readFile:       os.ReadFile,
		writeFile:      os.WriteFile,
		mkdirAll:       os.MkdirAll,
		removeAll:      os.RemoveAll,
		userHomeDir:    func() (string, error) { return home, nil },
		newHerdrClient: func(string) herdr.Client { return client },
	}, stderr
}

func TestRunInstallPluginHerdrNotOnPath(t *testing.T) {
	home := t.TempDir()
	d, _ := pluginTestDeps(t, home, &fakeHerdrClient{}, errors.New("not found"), nil, nil)

	err := runInstallPlugin(d)
	if err == nil {
		t.Fatal("runInstallPlugin() = nil, want error")
	}

	if exists, _ := fileExists(herdrplugin.ManifestPath(home)); exists {
		t.Error("manifest was written although herdr is not on PATH")
	}
}

func TestRunInstallPluginWritesLinksAndSyncsInsideHerdr(t *testing.T) {
	home := t.TempDir()
	var linkCalls [][]string
	runCommand := func(name string, args ...string) ([]byte, error) {
		linkCalls = append(linkCalls, append([]string{name}, args...))
		return nil, nil
	}
	client := &fakeHerdrClient{
		snapshot: herdr.SnapshotResult{},
	}
	env := map[string]string{"HERDR_SOCKET_PATH": "/tmp/herdr.sock"}
	d, stderr := pluginTestDeps(t, home, client, nil, runCommand, env)

	if err := runInstallPlugin(d); err != nil {
		t.Fatalf("runInstallPlugin() returned error: %v, stderr: %s", err, stderr)
	}

	manifestPath := herdrplugin.ManifestPath(home)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	want := herdrplugin.GenerateManifest("/home/david/dev/blf/blf")
	if string(data) != want {
		t.Errorf("manifest content =\n%s\nwant:\n%s", data, want)
	}

	if len(linkCalls) != 1 || !equalArgs(linkCalls[0], []string{"herdr", "plugin", "link", herdrplugin.Dir(home)}) {
		t.Errorf("linkCalls = %v, want one herdr plugin link call", linkCalls)
	}

	if client.snapshotCalls != 1 {
		t.Errorf("snapshotCalls = %d, want 1 (full-session sync run inside herdr)", client.snapshotCalls)
	}
}

func TestRunInstallPluginSkipsSyncOutsideHerdr(t *testing.T) {
	home := t.TempDir()
	runCommand := func(name string, args ...string) ([]byte, error) { return nil, nil }
	client := &fakeHerdrClient{}
	d, _ := pluginTestDeps(t, home, client, nil, runCommand, nil)

	if err := runInstallPlugin(d); err != nil {
		t.Fatalf("runInstallPlugin() returned error: %v", err)
	}

	if client.snapshotCalls != 0 {
		t.Errorf("snapshotCalls = %d, want 0 outside herdr", client.snapshotCalls)
	}
}

func TestRunInstallPluginLinkFailureKeepsManifest(t *testing.T) {
	home := t.TempDir()
	runCommand := func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("link: plugin root must be absolute")
	}
	d, stderr := pluginTestDeps(t, home, &fakeHerdrClient{}, nil, runCommand, nil)

	err := runInstallPlugin(d)
	if err == nil {
		t.Fatal("runInstallPlugin() = nil, want error")
	}
	if err.Error() == "" {
		t.Error("runInstallPlugin() returned an already-printed (empty) error; main would print nothing")
	}

	if !strings.Contains(stderr.String(), "plugin root must be absolute") {
		t.Errorf("stderr = %q, want herdr's error message", stderr.String())
	}

	if exists, _ := fileExists(herdrplugin.ManifestPath(home)); !exists {
		t.Error("manifest was removed although it should stay after a failed link")
	}
}

func TestRunInstallPluginSyncFailurePrintsAndExits(t *testing.T) {
	home := t.TempDir()
	runCommand := func(name string, args ...string) ([]byte, error) { return nil, nil }
	client := &fakeHerdrClient{snapshotErr: errors.New("socket unreachable")}
	env := map[string]string{"HERDR_SOCKET_PATH": "/tmp/herdr.sock"}
	d, _ := pluginTestDeps(t, home, client, nil, runCommand, env)

	err := runInstallPlugin(d)
	if err == nil {
		t.Fatal("runInstallPlugin() = nil, want error")
	}

	if exists, _ := fileExists(herdrplugin.ManifestPath(home)); !exists {
		t.Error("manifest should stay in place after a failed post-link sync")
	}
}

func TestRunInstallPluginRerunOverwritesAndRelinks(t *testing.T) {
	home := t.TempDir()
	linkCount := 0
	runCommand := func(name string, args ...string) ([]byte, error) {
		linkCount++
		return nil, nil
	}
	client := &fakeHerdrClient{}
	d, _ := pluginTestDeps(t, home, client, nil, runCommand, nil)

	if err := runInstallPlugin(d); err != nil {
		t.Fatalf("first run: %v", err)
	}

	manifestPath := herdrplugin.ManifestPath(home)
	if err := os.WriteFile(manifestPath, []byte("hand-edited"), 0o644); err != nil {
		t.Fatalf("hand-edit manifest: %v", err)
	}

	if err := runInstallPlugin(d); err != nil {
		t.Fatalf("second run: %v", err)
	}

	if linkCount != 2 {
		t.Errorf("linkCount = %d, want 2 (link runs again on re-run)", linkCount)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if string(data) == "hand-edited" {
		t.Error("re-run did not overwrite the hand-edited manifest")
	}
}

func TestRunUninstallPluginHerdrNotOnPath(t *testing.T) {
	home := t.TempDir()
	d, _ := pluginTestDeps(t, home, &fakeHerdrClient{}, errors.New("not found"), nil, nil)

	err := runUninstallPlugin(d)
	if err == nil {
		t.Fatal("runUninstallPlugin() = nil, want error")
	}
}

func TestRunUninstallPluginUnlinksAndDeletesFolder(t *testing.T) {
	home := t.TempDir()
	dir := herdrplugin.Dir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "herdr-plugin.toml"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup manifest: %v", err)
	}

	var unlinkCalls [][]string
	runCommand := func(name string, args ...string) ([]byte, error) {
		unlinkCalls = append(unlinkCalls, append([]string{name}, args...))
		return nil, nil
	}
	d, _ := pluginTestDeps(t, home, &fakeHerdrClient{}, nil, runCommand, nil)

	if err := runUninstallPlugin(d); err != nil {
		t.Fatalf("runUninstallPlugin() returned error: %v", err)
	}

	if len(unlinkCalls) != 1 || !equalArgs(unlinkCalls[0], []string{"herdr", "plugin", "unlink", "blf"}) {
		t.Errorf("unlinkCalls = %v, want one herdr plugin unlink call", unlinkCalls)
	}

	if exists, _ := fileExists(dir); exists {
		t.Error("plugin dir still exists after uninstall")
	}
}

func TestRunUninstallPluginAlreadyUninstalledIsNotAnError(t *testing.T) {
	home := t.TempDir()
	runCommand := func(name string, args ...string) ([]byte, error) { return nil, nil }
	d, _ := pluginTestDeps(t, home, &fakeHerdrClient{}, nil, runCommand, nil)

	if err := runUninstallPlugin(d); err != nil {
		t.Fatalf("runUninstallPlugin() on an already-uninstalled plugin returned error: %v", err)
	}
}

func TestResolveBlfPathUsesRunningBinaryWhenNotOnPath(t *testing.T) {
	d := deps{
		lookPath:       func(string) (string, error) { return "", errors.New("not found") },
		evalSymlinks:   func(path string) (string, error) { return path, nil },
		executablePath: func() (string, error) { return "/home/david/dev/blf/blf", nil },
	}

	got, err := resolveBlfPath(d)
	if err != nil {
		t.Fatalf("resolveBlfPath() returned error: %v", err)
	}
	if got != "/home/david/dev/blf/blf" {
		t.Errorf("resolveBlfPath() = %q, want running binary path", got)
	}
}

func TestResolveBlfPathUsesPathEntryWhenSymlinkMatches(t *testing.T) {
	d := deps{
		lookPath:       func(string) (string, error) { return "/opt/homebrew/bin/blf", nil },
		evalSymlinks:   func(string) (string, error) { return "/opt/homebrew/Cellar/blf/1.0/bin/blf", nil },
		executablePath: func() (string, error) { return "/opt/homebrew/Cellar/blf/1.0/bin/blf", nil },
	}

	got, err := resolveBlfPath(d)
	if err != nil {
		t.Fatalf("resolveBlfPath() returned error: %v", err)
	}
	if got != "/opt/homebrew/bin/blf" {
		t.Errorf("resolveBlfPath() = %q, want the unresolved PATH entry", got)
	}
}

func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
