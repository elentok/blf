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

// doctorTestDeps returns a deps set up with real filesystem operations
// rooted at home, and a fake socket client.
func doctorTestDeps(t *testing.T, home string, client herdr.Client, env map[string]string) (deps, *strings.Builder) {
	t.Helper()

	out := &strings.Builder{}
	return deps{
		stdout: out,
		stderr: &strings.Builder{},
		lookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		lookPath:       func(string) (string, error) { return "", errors.New("blf not on PATH") },
		evalSymlinks:   func(path string) (string, error) { return path, nil },
		executablePath: func() (string, error) { return filepath.Join(home, "blf"), nil },
		userHomeDir:    func() (string, error) { return home, nil },
		newHerdrClient: func(string) herdr.Client { return client },
		readFile:       os.ReadFile,
		writeFile:      os.WriteFile,
		mkdirAll:       os.MkdirAll,
		fileExists:     fileExists,
		isExecutable:   isExecutable,
	}, out
}

// setupValidPlugin writes an executable blf binary and a manifest that
// matches herdrplugin.GenerateManifest for it, so doctor's filesystem checks
// (manifest, blf path) all pass.
func setupValidPlugin(t *testing.T, home string) string {
	t.Helper()

	blfPath := filepath.Join(home, "blf")
	if err := os.WriteFile(blfPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write blf binary: %v", err)
	}

	dir := herdrplugin.Dir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	manifest := herdrplugin.GenerateManifest(blfPath)
	if err := os.WriteFile(herdrplugin.ManifestPath(home), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	return blfPath
}

func insideHerdrEnv() map[string]string {
	return map[string]string{"HERDR_SOCKET_PATH": "/tmp/herdr.sock"}
}

func TestRunHerdrDoctorAllPassExitsNil(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.3"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	if err := runHerdrDoctor(d); err != nil {
		t.Fatalf("runHerdrDoctor() = %v, want nil; output:\n%s", err, out)
	}
	if strings.Contains(out.String(), "[FAIL]") {
		t.Errorf("output contains a failing check, want all pass:\n%s", out)
	}
}

func TestRunHerdrDoctorVersionTooOldFails(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.2"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if !strings.Contains(out.String(), "restart the herdr server") {
		t.Errorf("output = %q, want it to mention restarting the herdr server", out.String())
	}
}

func TestRunHerdrDoctorMissingSnapshotFieldFails(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:        herdr.PingResult{Version: "0.9.3"},
		plugins:     []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
		snapshotErr: errors.New("herdr socket reply not understood"),
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if !strings.Contains(out.String(), "session.snapshot") {
		t.Errorf("output = %q, want a failing session.snapshot line", out.String())
	}
}

func TestRunHerdrDoctorMissingWorkspaceFieldFails(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.3"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
		listErr: errors.New("herdr socket reply not understood"),
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if !strings.Contains(out.String(), "workspace.list") {
		t.Errorf("output = %q, want a failing workspace.list line", out.String())
	}
}

func TestRunHerdrDoctorPluginNotLinkedFails(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping: herdr.PingResult{Version: "0.9.3"},
		// empty plugins array: not linked.
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if !strings.Contains(out.String(), "plugin linked") {
		t.Errorf("output = %q, want a failing plugin linked line", out.String())
	}
}

func TestRunHerdrDoctorManifestMismatchFails(t *testing.T) {
	home := t.TempDir()
	blfPath := setupValidPlugin(t, home)
	// Overwrite with a manifest that still carries a parseable blf path but
	// otherwise differs from what GenerateManifest produces.
	manifest := herdrplugin.GenerateManifest(blfPath) + "\nextra = \"field\"\n"
	if err := os.WriteFile(herdrplugin.ManifestPath(home), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.3"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if !strings.Contains(out.String(), "install-plugin") {
		t.Errorf("output = %q, want it to mention `blf herdr install-plugin`", out.String())
	}
}

func TestRunHerdrDoctorManifestDifferingOnlyInBlfPathPasses(t *testing.T) {
	home := t.TempDir()
	// A different blf path than the one currently running (e.g. a
	// Homebrew-installed manifest checked by a dev build), but otherwise a
	// manifest generated the same way, and that path is itself executable.
	otherBlfPath := filepath.Join(home, "other-blf")
	if err := os.WriteFile(otherBlfPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write other blf binary: %v", err)
	}
	dir := herdrplugin.Dir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	manifest := herdrplugin.GenerateManifest(otherBlfPath)
	if err := os.WriteFile(herdrplugin.ManifestPath(home), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.3"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	if err := runHerdrDoctor(d); err != nil {
		t.Fatalf("runHerdrDoctor() = %v, want nil (blf path excluded from manifest comparison); output:\n%s", err, out)
	}
}

func TestRunHerdrDoctorBlfPathNotExecutableFails(t *testing.T) {
	home := t.TempDir()
	notExecPath := filepath.Join(home, "not-exec")
	if err := os.WriteFile(notExecPath, []byte("not a binary"), 0o644); err != nil {
		t.Fatalf("write non-executable file: %v", err)
	}
	dir := herdrplugin.Dir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}
	manifest := herdrplugin.GenerateManifest(notExecPath)
	if err := os.WriteFile(herdrplugin.ManifestPath(home), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.3"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if !strings.Contains(out.String(), "blf path") {
		t.Errorf("output = %q, want a failing blf path line", out.String())
	}
}

func TestRunHerdrDoctorOutsideHerdrSkipsSocketChecksButRunsFilesystemChecks(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{pingErr: errors.New("should not be called")}
	d, out := doctorTestDeps(t, home, client, nil)

	err := runHerdrDoctor(d)
	if err == nil {
		t.Fatal("runHerdrDoctor() = nil, want error (exit 1)")
	}
	if client.snapshotCalls != 0 {
		t.Errorf("snapshotCalls = %d, want 0 outside herdr", client.snapshotCalls)
	}

	failLines := 0
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if strings.HasPrefix(line, "[FAIL]") {
			failLines++
		}
	}
	if failLines != 1 {
		t.Errorf("output had %d failing lines, want exactly 1 (HERDR_SOCKET_PATH):\n%s", failLines, out.String())
	}
	if !strings.Contains(out.String(), "HERDR_SOCKET_PATH") {
		t.Errorf("output = %q, want a failing HERDR_SOCKET_PATH line", out.String())
	}
	if !strings.Contains(out.String(), "[PASS] manifest") {
		t.Errorf("output = %q, want the filesystem-only manifest check to still run and pass", out.String())
	}
	if !strings.Contains(out.String(), "[PASS] blf path") {
		t.Errorf("output = %q, want the filesystem-only blf path check to still run and pass", out.String())
	}
}
