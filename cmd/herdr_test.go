package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elentok/blf/internal/herdr"
)

type fakeHerdrClient struct {
	layout        herdr.LayoutInfo
	layoutErr     error
	pane          herdr.PaneInfo
	paneErr       error
	tab           herdr.TabInfo
	tabErr        error
	renameErr     error
	renameErrFor  map[string]error
	snapshot      herdr.SnapshotResult
	snapshotErr   error
	layoutCalls   []string
	paneCalls     []string
	snapshotCalls int
	renamed       []struct{ tabID, label string }

	workspaces    []herdr.WorkspaceInfo
	listErr       error
	focusedID     string
	focusErr      error
	createLabel   string
	createCwd     string
	createErr     error
	createdResult herdr.WorkspaceInfo
}

func (f *fakeHerdrClient) Ping() (herdr.PingResult, error) { return herdr.PingResult{}, nil }

func (f *fakeHerdrClient) PaneLayout(paneID string) (herdr.LayoutInfo, error) {
	f.layoutCalls = append(f.layoutCalls, paneID)
	return f.layout, f.layoutErr
}

func (f *fakeHerdrClient) PaneGet(paneID string) (herdr.PaneInfo, error) {
	f.paneCalls = append(f.paneCalls, paneID)
	return f.pane, f.paneErr
}

func (f *fakeHerdrClient) TabGet(tabID string) (herdr.TabInfo, error) { return f.tab, f.tabErr }

func (f *fakeHerdrClient) TabRename(tabID, label string) error {
	f.renamed = append(f.renamed, struct{ tabID, label string }{tabID, label})
	if f.renameErrFor != nil {
		if err, ok := f.renameErrFor[tabID]; ok {
			return err
		}
	}
	return f.renameErr
}

func (f *fakeHerdrClient) SessionSnapshot() (herdr.SnapshotResult, error) {
	f.snapshotCalls++
	return f.snapshot, f.snapshotErr
}

func (f *fakeHerdrClient) WorkspaceList() ([]herdr.WorkspaceInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.workspaces, nil
}

func (f *fakeHerdrClient) WorkspaceFocus(workspaceID string) error {
	if f.focusErr != nil {
		return f.focusErr
	}
	f.focusedID = workspaceID
	return nil
}

func (f *fakeHerdrClient) WorkspaceCreate(label, cwd string) (herdr.WorkspaceInfo, error) {
	if f.createErr != nil {
		return herdr.WorkspaceInfo{}, f.createErr
	}
	f.createLabel = label
	f.createCwd = cwd
	return f.createdResult, nil
}

func newHerdrSyncDeps(client *fakeHerdrClient, env map[string]string) deps {
	return newHerdrSyncDepsWithStderr(client, env, &bytes.Buffer{})
}

func newHerdrSyncDepsWithStderr(client *fakeHerdrClient, env map[string]string, stderr *bytes.Buffer) deps {
	return deps{
		stderr: stderr,
		lookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		userHomeDir:    func() (string, error) { return "/home/david", nil },
		newHerdrClient: func(string) herdr.Client { return client },
		herdrLabel: func(pane herdr.PaneInfo, _ string) string {
			if pane.Label != "" {
				return pane.Label
			}
			return "computed-label"
		},
		fileExists: func(string) (bool, error) { return false, nil },
		now:        func() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) },
		appendFile: func(string, []byte) error { return nil },
	}
}

// newHerdrSyncDepsWithLog is newHerdrSyncDeps with debug logging turned on:
// fileExists reports the marker present and every append is captured in log.
func newHerdrSyncDepsWithLog(client *fakeHerdrClient, env map[string]string, log *[]string) deps {
	d := newHerdrSyncDeps(client, env)
	d.fileExists = func(string) (bool, error) { return true, nil }
	d.appendFile = func(_ string, data []byte) error {
		*log = append(*log, string(data))
		return nil
	}
	return d
}

func baseEnv() map[string]string {
	return map[string]string{
		"HERDR_ENV":         "1",
		"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
		"HERDR_TAB_ID":      "tab-1",
		"HERDR_PANE_ID":     "pane-event",
	}
}

func TestRunHerdrSyncTabTitleRenamesWhenLabelDiffers(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrSyncDeps(client, baseEnv())

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if len(client.renamed) != 1 || client.renamed[0] != (struct{ tabID, label string }{"tab-1", "computed-label"}) {
		t.Errorf("renamed = %+v, want one rename of tab-1 to computed-label", client.renamed)
	}
}

func TestRunHerdrSyncTabTitleNoRenameWhenLabelEqual(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "computed-label"},
	}
	d := newHerdrSyncDeps(client, baseEnv())

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if len(client.renamed) != 0 {
		t.Errorf("renamed = %+v, want no rename", client.renamed)
	}
}

func TestRunHerdrSyncTabTitleUsesActivePaneNotEventPane(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrSyncDeps(client, baseEnv())

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if len(client.layoutCalls) != 1 || client.layoutCalls[0] != "pane-event" {
		t.Errorf("layoutCalls = %v, want pane.layout called with the event pane", client.layoutCalls)
	}
	if len(client.paneCalls) != 1 || client.paneCalls[0] != "pane-active" {
		t.Errorf("paneCalls = %v, want pane.get called with the tab's active pane, not the event pane", client.paneCalls)
	}
}

func TestRunHerdrSyncTabTitleSilentOutsideHerdr(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "HERDR_ENV unset",
			env: map[string]string{
				"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
				"HERDR_TAB_ID":      "tab-1",
			},
		},
		{
			name: "HERDR_SOCKET_PATH unset",
			env: map[string]string{
				"HERDR_ENV":    "1",
				"HERDR_TAB_ID": "tab-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeHerdrClient{}
			d := newHerdrSyncDeps(client, tt.env)

			if err := runHerdrSyncTabTitle(d); err != nil {
				t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
			}
			if len(client.layoutCalls) != 0 {
				t.Errorf("layoutCalls = %v, want no herdr calls outside herdr", client.layoutCalls)
			}
		})
	}
}

func noTabIDEnv() map[string]string {
	return map[string]string{
		"HERDR_ENV":         "1",
		"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
	}
}

func TestRunHerdrSyncTabTitleNoTabIDSyncsEveryTabFromSnapshot(t *testing.T) {
	client := &fakeHerdrClient{
		snapshot: herdr.SnapshotResult{
			Tabs: []herdr.TabInfo{
				{TabID: "tab-1", Label: "old-label"},
				{TabID: "tab-2", Label: "same-label"},
			},
			Panes: []herdr.PaneInfo{
				{PaneID: "pane-1", Label: "new-label"},
				{PaneID: "pane-2", Label: "same-label"},
			},
			Layouts: []herdr.LayoutInfo{
				{TabID: "tab-1", FocusedPaneID: "pane-1"},
				{TabID: "tab-2", FocusedPaneID: "pane-2"},
			},
		},
	}
	d := newHerdrSyncDeps(client, noTabIDEnv())

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if client.snapshotCalls != 1 {
		t.Errorf("snapshotCalls = %d, want 1", client.snapshotCalls)
	}
	if len(client.renamed) != 1 || client.renamed[0] != (struct{ tabID, label string }{"tab-1", "new-label"}) {
		t.Errorf("renamed = %+v, want one rename of tab-1 to new-label", client.renamed)
	}
}

func TestRunHerdrSyncTabTitleOneFailedTabDoesNotStopOthers(t *testing.T) {
	stderr := &bytes.Buffer{}
	client := &fakeHerdrClient{
		snapshot: herdr.SnapshotResult{
			Tabs: []herdr.TabInfo{
				{TabID: "tab-1", Label: "old-label"},
				{TabID: "tab-2", Label: "old-label"},
			},
			Panes: []herdr.PaneInfo{
				{PaneID: "pane-1", Label: "new-label"},
				{PaneID: "pane-2", Label: "new-label"},
			},
			Layouts: []herdr.LayoutInfo{
				{TabID: "tab-1", FocusedPaneID: "pane-1"},
				{TabID: "tab-2", FocusedPaneID: "pane-2"},
			},
		},
		renameErrFor: map[string]error{"tab-1": errors.New("rename failed")},
	}
	d := newHerdrSyncDepsWithStderr(client, noTabIDEnv(), stderr)

	err := runHerdrSyncTabTitle(d)
	if err == nil {
		t.Fatal("runHerdrSyncTabTitle returned nil error, want an error (exit 1)")
	}

	if len(client.renamed) != 2 {
		t.Errorf("renamed = %+v, want both tab-1 and tab-2 attempted", client.renamed)
	}

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("stderr = %q, want exactly one line for the one failed tab", stderr.String())
	}
}

func TestRunHerdrSyncTabTitleMissingLayoutIsOneStderrLineAndOtherTabsStillRun(t *testing.T) {
	stderr := &bytes.Buffer{}
	client := &fakeHerdrClient{
		snapshot: herdr.SnapshotResult{
			Tabs: []herdr.TabInfo{
				{TabID: "tab-1", Label: "old-label"},
				{TabID: "tab-2", Label: "old-label"},
			},
			Panes: []herdr.PaneInfo{
				{PaneID: "pane-2", Label: "new-label"},
			},
			Layouts: []herdr.LayoutInfo{
				{TabID: "tab-2", FocusedPaneID: "pane-2"},
			},
		},
	}
	d := newHerdrSyncDepsWithStderr(client, noTabIDEnv(), stderr)

	err := runHerdrSyncTabTitle(d)
	if err == nil {
		t.Fatal("runHerdrSyncTabTitle returned nil error, want an error (exit 1)")
	}

	if len(client.renamed) != 1 || client.renamed[0] != (struct{ tabID, label string }{"tab-2", "new-label"}) {
		t.Errorf("renamed = %+v, want tab-2 still renamed despite tab-1 missing a layout", client.renamed)
	}

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("stderr = %q, want exactly one line for the one failed tab", stderr.String())
	}
}

func TestRunHerdrSyncTabTitleSnapshotErrorReturnsOneLineError(t *testing.T) {
	client := &fakeHerdrClient{snapshotErr: errors.New("connection refused")}
	d := newHerdrSyncDeps(client, noTabIDEnv())

	err := runHerdrSyncTabTitle(d)
	if err == nil {
		t.Fatal("runHerdrSyncTabTitle returned nil error, want an error")
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error %q contains a newline, want one line", err.Error())
	}
}

func TestRunHerdrSyncTabTitleClientErrorReturnsOneLineError(t *testing.T) {
	client := &fakeHerdrClient{layoutErr: errors.New("connection refused")}
	d := newHerdrSyncDeps(client, baseEnv())

	err := runHerdrSyncTabTitle(d)
	if err == nil {
		t.Fatal("runHerdrSyncTabTitle returned nil error, want an error")
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("error %q contains a newline, want one line", err.Error())
	}
}

func TestRunHerdrSyncTabTitleLogsRenameWhenDebugOn(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	var log []string
	d := newHerdrSyncDepsWithLog(client, baseEnv(), &log)

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if len(log) != 1 {
		t.Fatalf("log = %v, want exactly one line", log)
	}
	fields := strings.Split(strings.TrimRight(log[0], "\n"), "\t")
	if len(fields) != 6 {
		t.Fatalf("fields = %v, want 6 tab-separated fields", fields)
	}
	if fields[1] != "event" || fields[2] != "tab-1" || fields[3] != "old-label" || fields[4] != "computed-label" || fields[5] != "" {
		t.Errorf("fields = %v, want [ts event tab-1 old-label computed-label \"\"]", fields)
	}
}

func TestRunHerdrSyncTabTitleLogsSkipWhenDebugOn(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "computed-label"},
	}
	var log []string
	d := newHerdrSyncDepsWithLog(client, baseEnv(), &log)

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if len(log) != 1 {
		t.Fatalf("log = %v, want exactly one line", log)
	}
	fields := strings.Split(strings.TrimRight(log[0], "\n"), "\t")
	if fields[2] != "tab-1" || fields[3] != "computed-label" || fields[4] != "computed-label" || fields[5] == "" {
		t.Errorf("fields = %v, want tab-1/computed-label/computed-label with a skip reason", fields)
	}
}

func TestRunHerdrSyncTabTitleLogsErrorWhenDebugOn(t *testing.T) {
	client := &fakeHerdrClient{layoutErr: errors.New("connection refused")}
	var log []string
	d := newHerdrSyncDepsWithLog(client, baseEnv(), &log)

	if err := runHerdrSyncTabTitle(d); err == nil {
		t.Fatal("runHerdrSyncTabTitle returned nil error, want an error")
	}

	if len(log) != 1 {
		t.Fatalf("log = %v, want exactly one line", log)
	}
	fields := strings.Split(strings.TrimRight(log[0], "\n"), "\t")
	if fields[2] != "tab-1" || fields[5] != "connection refused" {
		t.Errorf("fields = %v, want tab-1 with the connection error as the last field", fields)
	}
}

func TestRunHerdrSyncTabTitleNoLogWhenDebugOff(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	var log []string
	d := newHerdrSyncDeps(client, baseEnv())
	d.appendFile = func(_ string, data []byte) error {
		log = append(log, string(data))
		return nil
	}

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}

	if len(log) != 0 {
		t.Errorf("log = %v, want no lines written when debug is off", log)
	}
}

func TestRunHerdrSyncTabTitleEmptyComputedLabelSendsNoRename(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrSyncDeps(client, baseEnv())
	d.herdrLabel = func(herdr.PaneInfo, string) string { return "" }

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}
	if len(client.renamed) != 0 {
		t.Errorf("renamed = %+v, want no rename when the computed label is empty", client.renamed)
	}
}

func newHerdrTestDeps(out *strings.Builder, socketPath string, client *fakeHerdrClient, dirs map[string]bool) deps {
	return deps{
		stdout: out,
		stderr: &strings.Builder{},
		lookupEnv: func(key string) (string, bool) {
			if key == "HERDR_SOCKET_PATH" {
				if socketPath == "" {
					return "", false
				}
				return socketPath, true
			}
			return "", false
		},
		newHerdrClient: func(string) herdr.Client { return client },
		dirExists: func(path string) (bool, error) {
			return dirs[path], nil
		},
	}
}

func TestRunFindWorkspaceMatchesIgnoringCase(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{workspaces: []herdr.WorkspaceInfo{
		{WorkspaceID: "w1", Label: "Notes"},
	}}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	if err := runFindWorkspace("notes", d); err != nil {
		t.Fatalf("runFindWorkspace: %v", err)
	}
	if out.String() != "w1\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "w1\n")
	}
}

func TestRunFindWorkspaceFirstDuplicateWins(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{workspaces: []herdr.WorkspaceInfo{
		{WorkspaceID: "w1", Label: "notes"},
		{WorkspaceID: "w2", Label: "notes"},
	}}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	if err := runFindWorkspace("notes", d); err != nil {
		t.Fatalf("runFindWorkspace: %v", err)
	}
	if out.String() != "w1\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "w1\n")
	}
}

func TestRunFindWorkspaceNotFound(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	err := runFindWorkspace("notes", d)
	wantErr := `no workspace labelled "notes"`
	if err == nil || err.Error() != wantErr {
		t.Fatalf("err = %v, want %q", err, wantErr)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestRunFindWorkspaceEmptyLabelIsUsageError(t *testing.T) {
	out := &strings.Builder{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", &fakeHerdrClient{}, nil)

	if err := runFindWorkspace("", d); err == nil {
		t.Fatal("expected usage error, got nil")
	}
}

func TestRunFindWorkspaceOutsideHerdr(t *testing.T) {
	out := &strings.Builder{}
	d := newHerdrTestDeps(out, "", &fakeHerdrClient{}, nil)

	err := runFindWorkspace("notes", d)
	wantErr := "not running inside herdr"
	if err == nil || err.Error() != wantErr {
		t.Fatalf("err = %v, want %q", err, wantErr)
	}
}

func TestRunGotoWorkspaceFocusesExisting(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{workspaces: []herdr.WorkspaceInfo{
		{WorkspaceID: "w1", Label: "notes"},
	}}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	if err := runGotoWorkspace("Notes", false, "", d); err != nil {
		t.Fatalf("runGotoWorkspace: %v", err)
	}
	if client.focusedID != "w1" {
		t.Errorf("focusedID = %q, want w1", client.focusedID)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestRunGotoWorkspaceNotFoundWithoutCreate(t *testing.T) {
	out := &strings.Builder{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", &fakeHerdrClient{}, nil)

	err := runGotoWorkspace("notes", false, "", d)
	wantErr := `no workspace labelled "notes"`
	if err == nil || err.Error() != wantErr {
		t.Fatalf("err = %v, want %q", err, wantErr)
	}
}

func TestRunGotoWorkspaceCreatesWhenMissing(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	if err := runGotoWorkspace("scratch", true, "", d); err != nil {
		t.Fatalf("runGotoWorkspace: %v", err)
	}
	if client.createLabel != "scratch" || client.createCwd != "" {
		t.Errorf("createLabel/Cwd = %q/%q, want scratch/empty", client.createLabel, client.createCwd)
	}
}

func TestRunGotoWorkspaceCreateHonoursCwd(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{}
	dirs := map[string]bool{"/tmp/proj": true}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, dirs)

	if err := runGotoWorkspace("scratch", true, "/tmp/proj", d); err != nil {
		t.Fatalf("runGotoWorkspace: %v", err)
	}
	if client.createCwd != "/tmp/proj" {
		t.Errorf("createCwd = %q, want /tmp/proj", client.createCwd)
	}
}

func TestRunGotoWorkspaceCreateRejectsNonExistentCwd(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	err := runGotoWorkspace("scratch", true, "/no/such/dir", d)
	if err == nil {
		t.Fatal("expected error for non-existent cwd, got nil")
	}
	if client.createLabel != "" {
		t.Errorf("WorkspaceCreate should not have been called, got label %q", client.createLabel)
	}
}

func TestRunGotoWorkspaceCwdIgnoredWhenWorkspaceExists(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{workspaces: []herdr.WorkspaceInfo{
		{WorkspaceID: "w1", Label: "notes"},
	}}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	if err := runGotoWorkspace("notes", true, "/no/such/dir", d); err != nil {
		t.Fatalf("runGotoWorkspace: %v", err)
	}
	if client.focusedID != "w1" {
		t.Errorf("focusedID = %q, want w1", client.focusedID)
	}
}

func TestRunGotoWorkspaceCwdWithoutCreateIsUsageError(t *testing.T) {
	out := &strings.Builder{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", &fakeHerdrClient{}, nil)

	if err := runGotoWorkspace("notes", false, "/tmp/proj", d); err == nil {
		t.Fatal("expected usage error, got nil")
	}
}

func TestRunGotoWorkspaceEmptyLabelIsUsageError(t *testing.T) {
	out := &strings.Builder{}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", &fakeHerdrClient{}, nil)

	if err := runGotoWorkspace("", false, "", d); err == nil {
		t.Fatal("expected usage error, got nil")
	}
}

func TestRunGotoWorkspaceOutsideHerdr(t *testing.T) {
	out := &strings.Builder{}
	d := newHerdrTestDeps(out, "", &fakeHerdrClient{}, nil)

	err := runGotoWorkspace("notes", false, "", d)
	wantErr := "not running inside herdr"
	if err == nil || err.Error() != wantErr {
		t.Fatalf("err = %v, want %q", err, wantErr)
	}
}

func TestRunGotoWorkspaceAlreadyFocusedIsNoop(t *testing.T) {
	out := &strings.Builder{}
	client := &fakeHerdrClient{workspaces: []herdr.WorkspaceInfo{
		{WorkspaceID: "w1", Label: "notes", Focused: true},
	}}
	d := newHerdrTestDeps(out, "/tmp/herdr.sock", client, nil)

	if err := runGotoWorkspace("notes", false, "", d); err != nil {
		t.Fatalf("runGotoWorkspace: %v", err)
	}
	if client.focusedID != "w1" {
		t.Errorf("focusedID = %q, want w1", client.focusedID)
	}
}

func newHerdrFishSyncDeps(client *fakeHerdrClient, env map[string]string) deps {
	d := newHerdrSyncDeps(client, env)
	d.getwd = func() (string, error) { return "/home/david/proj", nil }
	return d
}

func fishBaseEnv() map[string]string {
	return map[string]string{
		"HERDR_ENV":         "1",
		"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
		"HERDR_TAB_ID":      "tab-1",
		"HERDR_PANE_ID":     "pane-1",
	}
}

func TestRunHerdrSyncTabTitleFishCmdRenamesToComputedTitle(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-1"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrFishSyncDeps(client, fishBaseEnv())

	if err := runHerdrSyncTabTitleFish(d, "nvim foo.go"); err != nil {
		t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
	}

	if len(client.renamed) != 1 || client.renamed[0].tabID != "tab-1" {
		t.Fatalf("renamed = %+v, want one rename of tab-1", client.renamed)
	}
	if client.renamed[0].label == "" || client.renamed[0].label == "old-label" {
		t.Errorf("renamed label = %q, want the computed title", client.renamed[0].label)
	}
}

func TestRunHerdrSyncTabTitleFishPromptRenamesToDirectoryForm(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-1"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrFishSyncDeps(client, fishBaseEnv())

	if err := runHerdrSyncTabTitleFish(d, ""); err != nil {
		t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
	}

	if len(client.renamed) != 1 {
		t.Fatalf("renamed = %+v, want one rename", client.renamed)
	}
	if client.renamed[0].label != "proj" {
		t.Errorf("renamed label = %q, want the directory form %q", client.renamed[0].label, "proj")
	}
}

func TestRunHerdrSyncTabTitleFishNonActivePaneSendsNoRename(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-other"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrFishSyncDeps(client, fishBaseEnv())

	if err := runHerdrSyncTabTitleFish(d, "nvim foo.go"); err != nil {
		t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
	}

	if len(client.renamed) != 0 {
		t.Errorf("renamed = %+v, want no rename when the event pane is not the tab's active pane", client.renamed)
	}
}

func TestRunHerdrSyncTabTitleFishVimruntimeSendsNothing(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-1"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	env := fishBaseEnv()
	env["VIMRUNTIME"] = "/usr/share/nvim/runtime"
	d := newHerdrFishSyncDeps(client, env)

	if err := runHerdrSyncTabTitleFish(d, "nvim foo.go"); err != nil {
		t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
	}

	if len(client.layoutCalls) != 0 {
		t.Errorf("layoutCalls = %v, want no herdr calls when $VIMRUNTIME is set", client.layoutCalls)
	}
	if len(client.renamed) != 0 {
		t.Errorf("renamed = %+v, want no rename when $VIMRUNTIME is set", client.renamed)
	}
}

func TestRunHerdrSyncTabTitleFishMissingIdsExitsSilently(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "HERDR_TAB_ID unset",
			env: map[string]string{
				"HERDR_ENV":         "1",
				"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
				"HERDR_PANE_ID":     "pane-1",
			},
		},
		{
			name: "HERDR_PANE_ID unset",
			env: map[string]string{
				"HERDR_ENV":         "1",
				"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
				"HERDR_TAB_ID":      "tab-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeHerdrClient{}
			d := newHerdrFishSyncDeps(client, tt.env)

			if err := runHerdrSyncTabTitleFish(d, "nvim foo.go"); err != nil {
				t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
			}
			if len(client.layoutCalls) != 0 {
				t.Errorf("layoutCalls = %v, want no herdr calls when an id is missing", client.layoutCalls)
			}
		})
	}
}

func TestRunHerdrSyncTabTitleFishOutsideHerdrIsSilent(t *testing.T) {
	client := &fakeHerdrClient{}
	env := map[string]string{
		"HERDR_TAB_ID":  "tab-1",
		"HERDR_PANE_ID": "pane-1",
	}
	d := newHerdrFishSyncDeps(client, env)

	if err := runHerdrSyncTabTitleFish(d, "nvim foo.go"); err != nil {
		t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
	}
	if len(client.layoutCalls) != 0 {
		t.Errorf("layoutCalls = %v, want no herdr calls outside herdr", client.layoutCalls)
	}
}

func TestRunHerdrSyncTabTitleFishEmptyCmdBehavesLikePrompt(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-1"},
		tab:    herdr.TabInfo{Label: "proj"},
	}
	d := newHerdrFishSyncDeps(client, fishBaseEnv())

	if err := runHerdrSyncTabTitleFish(d, ""); err != nil {
		t.Fatalf("runHerdrSyncTabTitleFish returned error: %v", err)
	}

	if len(client.renamed) != 0 {
		t.Errorf("renamed = %+v, want no rename when the label already matches the directory form", client.renamed)
	}
}

func TestNewHerdrSyncTabTitleCmdRejectsCmdAndPromptTogether(t *testing.T) {
	client := &fakeHerdrClient{}
	d := newHerdrFishSyncDeps(client, fishBaseEnv())

	cmd := newHerdrSyncTabTitleCmd(d)
	cmd.SetArgs([]string{"--cmd", "nvim foo.go", "--prompt"})
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want a usage error for --cmd with --prompt")
	}
	if len(client.layoutCalls) != 0 {
		t.Errorf("layoutCalls = %v, want no herdr calls on a usage error", client.layoutCalls)
	}
}

func TestNewHerdrSyncTabTitleCmdDispatchesToEventModeByDefault(t *testing.T) {
	client := &fakeHerdrClient{
		layout: herdr.LayoutInfo{FocusedPaneID: "pane-active"},
		tab:    herdr.TabInfo{Label: "old-label"},
	}
	d := newHerdrSyncDeps(client, baseEnv())

	cmd := newHerdrSyncTabTitleCmd(d)
	cmd.SetArgs([]string{})
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(client.renamed) != 1 || client.renamed[0].label != "computed-label" {
		t.Errorf("renamed = %+v, want the event-mode label from d.herdrLabel", client.renamed)
	}
}
