package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/herdr"
	"github.com/elentok/blf/internal/herdrplugin"
	"github.com/elentok/blf/internal/herdrsync"
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
		runCommand:     func(string, ...string) ([]byte, error) { return nil, errors.New("herdr not on PATH") },
		evalSymlinks:   func(path string) (string, error) { return path, nil },
		executablePath: func() (string, error) { return filepath.Join(home, "blf"), nil },
		userHomeDir:    func() (string, error) { return home, nil },
		newHerdrClient: func(string) herdr.Client { return client },
		herdrLabel:     herdrsync.Label,
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

	if err := runHerdrDoctor(d, false); err != nil {
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

	err := runHerdrDoctor(d, false)
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

	err := runHerdrDoctor(d, false)
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

	err := runHerdrDoctor(d, false)
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

	err := runHerdrDoctor(d, false)
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

	err := runHerdrDoctor(d, false)
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

	if err := runHerdrDoctor(d, false); err != nil {
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

	err := runHerdrDoctor(d, false)
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

	err := runHerdrDoctor(d, false)
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

// snapshotWithTabs builds a SessionSnapshot with one tab that needs renaming
// (its label differs from what herdrLabel computes) and one that doesn't.
func snapshotWithTabs() herdr.SnapshotResult {
	return herdr.SnapshotResult{
		Tabs: []herdr.TabInfo{
			{TabID: "tab1", Label: "old-label"},
			{TabID: "tab2", Label: "proj"},
		},
		Panes: []herdr.PaneInfo{
			{PaneID: "pane1", Agent: "claude", TerminalTitle: "claude running"},
			{PaneID: "pane2", Cwd: "/home/user/proj"},
		},
		Layouts: []herdr.LayoutInfo{
			{TabID: "tab1", FocusedPaneID: "pane1"},
			{TabID: "tab2", FocusedPaneID: "pane2"},
		},
	}
}

func TestRunHerdrDoctorDryRunTableShowsOkAndWouldRename(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:     herdr.PingResult{Version: "0.9.3"},
		plugins:  []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
		snapshot: snapshotWithTabs(),
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())

	if err := runHerdrDoctor(d, false); err != nil {
		t.Fatalf("runHerdrDoctor() = %v, want nil (a would-rename row must not fail the report); output:\n%s", err, out)
	}
	var tab1Line, tab2Line string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "tab1") {
			tab1Line = line
		}
		if strings.HasPrefix(line, "tab2") {
			tab2Line = line
		}
	}
	if !strings.HasSuffix(tab1Line, "would rename") {
		t.Errorf("tab1 row = %q, want it to end in would rename", tab1Line)
	}
	if !strings.HasSuffix(tab2Line, "ok") {
		t.Errorf("tab2 row = %q, want it to end in ok", tab2Line)
	}
}

// pluginLogListJSON builds a fake `herdr plugin log list` reply with n
// entries, alternating "succeeded" and "failed" status, oldest first.
func pluginLogListJSON(n int) []byte {
	var logs []string
	for i := 0; i < n; i++ {
		status := "succeeded"
		if i%2 == 1 {
			status = "failed"
		}
		logs = append(logs, fmt.Sprintf(
			`{"event":"pre-create-%d","status":%q,"started_unix_ms":%d,"error":"boom-%d"}`,
			i, status, 1000+i, i))
	}
	return []byte(fmt.Sprintf(`{"id":"cli:plugin","result":{"logs":[%s],"type":"plugin_log_list"}}`, strings.Join(logs, ",")))
}

func TestRunHerdrDoctorFailedHooksOnlyFromLastTen(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:    herdr.PingResult{Version: "0.9.3"},
		plugins: []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())
	d.runCommand = func(name string, args ...string) ([]byte, error) {
		return pluginLogListJSON(14), nil
	}

	if err := runHerdrDoctor(d, false); err != nil {
		t.Fatalf("runHerdrDoctor() = %v, want nil (failed hooks are informational only); output:\n%s", err, out)
	}

	// Entries 0-3 are among the first 14 but outside the last 10 (indices
	// 4-13); only failed ones among the last 10 (5,7,9,11,13) should appear.
	if strings.Contains(out.String(), "boom-1\n") || strings.Contains(out.String(), "boom-3") {
		t.Errorf("output = %q, want entries outside the last 10 excluded", out.String())
	}
	if !strings.Contains(out.String(), "boom-13") {
		t.Errorf("output = %q, want the last failed entry reported", out.String())
	}
}

func TestRunHerdrDoctorJSONMatchesTextChecks(t *testing.T) {
	home := t.TempDir()
	setupValidPlugin(t, home)
	client := &fakeHerdrClient{
		ping:     herdr.PingResult{Version: "0.9.3"},
		plugins:  []herdr.PluginInfo{{PluginID: "blf", PluginRoot: herdrplugin.Dir(home)}},
		snapshot: snapshotWithTabs(),
	}
	d, out := doctorTestDeps(t, home, client, insideHerdrEnv())
	d.runCommand = func(name string, args ...string) ([]byte, error) {
		return pluginLogListJSON(2), nil
	}

	if err := runHerdrDoctor(d, true); err != nil {
		t.Fatalf("runHerdrDoctor() = %v, want nil; output:\n%s", err, out)
	}

	var decoded struct {
		OK     bool `json:"ok"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
		Tabs []struct {
			TabID  string `json:"tab_id"`
			Action string `json:"action"`
		} `json:"tabs"`
		FailedHooks []struct {
			Event string `json:"event"`
		} `json:"failed_hooks"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%q) = %v, want valid JSON report", out.String(), err)
	}

	if !decoded.OK {
		t.Errorf("decoded.OK = false, want true (no failing checks)")
	}
	if len(decoded.Checks) == 0 {
		t.Error("decoded.Checks is empty, want the same checks as the text report")
	}
	if len(decoded.Tabs) != 2 {
		t.Errorf("len(decoded.Tabs) = %d, want 2", len(decoded.Tabs))
	}
	if len(decoded.FailedHooks) != 1 {
		t.Errorf("len(decoded.FailedHooks) = %d, want 1 (one failed entry out of 2)", len(decoded.FailedHooks))
	}
}
