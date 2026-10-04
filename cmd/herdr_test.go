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
