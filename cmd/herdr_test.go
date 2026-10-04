package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/herdr"
)

type fakeHerdrClient struct {
	layout      herdr.LayoutInfo
	layoutErr   error
	pane        herdr.PaneInfo
	paneErr     error
	tab         herdr.TabInfo
	tabErr      error
	renameErr   error
	layoutCalls []string
	paneCalls   []string
	renamed     []struct{ tabID, label string }

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
	return f.renameErr
}

func (f *fakeHerdrClient) SessionSnapshot() (herdr.SnapshotResult, error) {
	return herdr.SnapshotResult{}, nil
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
	return deps{
		lookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		userHomeDir:    func() (string, error) { return "/home/david", nil },
		newHerdrClient: func(string) herdr.Client { return client },
		herdrLabel:     func(herdr.PaneInfo, string) string { return "computed-label" },
	}
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

func TestRunHerdrSyncTabTitleNoTabIDIsNoOp(t *testing.T) {
	client := &fakeHerdrClient{}
	env := map[string]string{
		"HERDR_ENV":         "1",
		"HERDR_SOCKET_PATH": "/tmp/herdr.sock",
	}
	d := newHerdrSyncDeps(client, env)

	if err := runHerdrSyncTabTitle(d); err != nil {
		t.Fatalf("runHerdrSyncTabTitle returned error: %v", err)
	}
	if len(client.layoutCalls) != 0 {
		t.Errorf("layoutCalls = %v, want no herdr calls when HERDR_TAB_ID is unset", client.layoutCalls)
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
