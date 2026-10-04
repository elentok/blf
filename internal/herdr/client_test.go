package herdr

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startFakeServer listens on a temp Unix socket and runs handle once per
// connection with the decoded request line. handle writes the raw reply
// line (including its trailing newline) itself, or does nothing to leave
// the client hanging until its own timeout.
func startFakeServer(t *testing.T, handle func(conn net.Conn, req map[string]any)) string {
	t.Helper()

	// A short, fixed-name dir: t.TempDir() embeds the (long) test name, and
	// that plus "herdr.sock" can exceed the Unix socket path length limit.
	dir, err := os.MkdirTemp("", "herdr")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	socketPath := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil {
					return
				}
				var req map[string]any
				if err := json.Unmarshal([]byte(line), &req); err != nil {
					return
				}
				handle(conn, req)
			}()
		}
	}()

	return socketPath
}

func reply(t *testing.T, conn net.Conn, req map[string]any, result map[string]any) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"id": req["id"], "result": result})
	if err != nil {
		t.Fatalf("marshal reply: %v", err)
	}
	conn.Write(append(data, '\n'))
}

func replyAPIError(t *testing.T, conn net.Conn, req map[string]any, code, message string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"id":    req["id"],
		"error": map[string]any{"code": code, "message": message},
	})
	if err != nil {
		t.Fatalf("marshal error reply: %v", err)
	}
	conn.Write(append(data, '\n'))
}

func TestPing(t *testing.T) {
	var gotMethod string
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		gotMethod = req["method"].(string)
		if req["id"] == nil || req["id"] == "" {
			t.Errorf("request had no id: %v", req)
		}
		reply(t, conn, req, map[string]any{"type": "pong", "version": "0.9.3", "protocol": 22})
	})

	result, err := New(socketPath).Ping()
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if gotMethod != "ping" {
		t.Errorf("method = %q, want ping", gotMethod)
	}
	if result != (PingResult{Version: "0.9.3", Protocol: 22}) {
		t.Errorf("Ping() = %+v", result)
	}
}

func TestPaneLayoutSendsExplicitID(t *testing.T) {
	var gotParams map[string]any
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		gotParams = req["params"].(map[string]any)
		reply(t, conn, req, map[string]any{
			"type": "layout_info",
			"layout": map[string]any{
				"workspace_id":    "w1",
				"tab_id":          "w1:t1",
				"focused_pane_id": "w1:p2",
				"zoomed":          false,
			},
		})
	})

	layout, err := New(socketPath).PaneLayout("w1:p1")
	if err != nil {
		t.Fatalf("PaneLayout: %v", err)
	}
	if gotParams["pane_id"] != "w1:p1" {
		t.Errorf("params.pane_id = %v, want w1:p1", gotParams["pane_id"])
	}
	want := LayoutInfo{WorkspaceID: "w1", TabID: "w1:t1", FocusedPaneID: "w1:p2", Zoomed: false}
	if layout != want {
		t.Errorf("PaneLayout() = %+v, want %+v", layout, want)
	}
}

func TestPaneGetOptionalFieldsEmptyWhenOmitted(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type": "pane_info",
			"pane": map[string]any{
				"pane_id":      "w1:p1",
				"terminal_id":  "term1",
				"workspace_id": "w1",
				"tab_id":       "w1:t1",
				"focused":      true,
				"agent_status": "idle",
				"revision":     1,
				// cwd, agent, label, title, terminal_title, etc. intentionally omitted.
			},
		})
	})

	pane, err := New(socketPath).PaneGet("w1:p1")
	if err != nil {
		t.Fatalf("PaneGet: %v", err)
	}
	want := PaneInfo{
		PaneID: "w1:p1", TerminalID: "term1", WorkspaceID: "w1", TabID: "w1:t1",
		Focused: true, AgentStatus: "idle", Revision: 1,
	}
	if pane != want {
		t.Errorf("PaneGet() = %+v, want %+v", pane, want)
	}
}

func TestPaneGetOptionalFieldsPresent(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type": "pane_info",
			"pane": map[string]any{
				"pane_id":                 "w1:p1",
				"terminal_id":             "term1",
				"workspace_id":            "w1",
				"tab_id":                  "w1:t1",
				"focused":                 false,
				"agent_status":            "working",
				"revision":                3,
				"cwd":                     "/tmp/a",
				"foreground_cwd":          "/tmp/b",
				"agent":                   "claude",
				"label":                   "custom",
				"title":                   "raw title",
				"terminal_title":          "raw title",
				"terminal_title_stripped": "title",
				"display_agent":           "Claude",
				"restore_error":           "boom",
			},
		})
	})

	pane, err := New(socketPath).PaneGet("w1:p1")
	if err != nil {
		t.Fatalf("PaneGet: %v", err)
	}
	if pane.Cwd != "/tmp/a" || pane.ForegroundCwd != "/tmp/b" || pane.Agent != "claude" ||
		pane.Label != "custom" || pane.Title != "raw title" || pane.TerminalTitle != "raw title" ||
		pane.TerminalTitleStripped != "title" || pane.DisplayAgent != "Claude" || pane.RestoreError != "boom" {
		t.Errorf("PaneGet() = %+v", pane)
	}
}

func TestTabGet(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type": "tab_info",
			"tab": map[string]any{
				"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "3",
				"focused": true, "pane_count": 2, "agent_status": "idle",
			},
		})
	})

	tab, err := New(socketPath).TabGet("w1:t1")
	if err != nil {
		t.Fatalf("TabGet: %v", err)
	}
	want := TabInfo{TabID: "w1:t1", WorkspaceID: "w1", Number: 1, Label: "3", Focused: true, PaneCount: 2, AgentStatus: "idle"}
	if tab != want {
		t.Errorf("TabGet() = %+v, want %+v", tab, want)
	}
}

func TestTabRenameSendsLabelAndID(t *testing.T) {
	var gotParams map[string]any
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		gotParams = req["params"].(map[string]any)
		reply(t, conn, req, map[string]any{
			"type": "tab_info",
			"tab": map[string]any{
				"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "new label",
				"focused": true, "pane_count": 1, "agent_status": "idle",
			},
		})
	})

	if err := New(socketPath).TabRename("w1:t1", "new label"); err != nil {
		t.Fatalf("TabRename: %v", err)
	}
	if gotParams["tab_id"] != "w1:t1" || gotParams["label"] != "new label" {
		t.Errorf("params = %v", gotParams)
	}
}

func TestSessionSnapshot(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type": "session_snapshot",
			"snapshot": map[string]any{
				"version": "0.9.3", "protocol": 22,
				"workspaces": []map[string]any{
					{"workspace_id": "w1", "number": 1, "label": "work", "focused": true, "pane_count": 1, "tab_count": 1, "active_tab_id": "w1:t1", "agent_status": "idle"},
				},
				"tabs": []map[string]any{
					{"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "1", "focused": true, "pane_count": 1, "agent_status": "idle"},
				},
				"panes": []map[string]any{
					{"pane_id": "w1:p1", "terminal_id": "term1", "workspace_id": "w1", "tab_id": "w1:t1", "focused": true, "agent_status": "idle", "revision": 1},
				},
				"layouts": []map[string]any{
					{"workspace_id": "w1", "tab_id": "w1:t1", "focused_pane_id": "w1:p1", "zoomed": false},
				},
			},
		})
	})

	snapshot, err := New(socketPath).SessionSnapshot()
	if err != nil {
		t.Fatalf("SessionSnapshot: %v", err)
	}
	if snapshot.Version != "0.9.3" || snapshot.Protocol != 22 {
		t.Errorf("version/protocol = %q/%d", snapshot.Version, snapshot.Protocol)
	}
	if len(snapshot.Workspaces) != 1 || len(snapshot.Tabs) != 1 || len(snapshot.Panes) != 1 || len(snapshot.Layouts) != 1 {
		t.Errorf("snapshot = %+v", snapshot)
	}
}

func TestWorkspaceList(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type": "workspace_list",
			"workspaces": []map[string]any{
				{"workspace_id": "w1", "number": 1, "label": "work", "focused": true, "active_tab_id": "w1:t1"},
				{"workspace_id": "w2", "number": 2, "label": "notes", "focused": false, "active_tab_id": "w2:t1"},
			},
		})
	})

	workspaces, err := New(socketPath).WorkspaceList()
	if err != nil {
		t.Fatalf("WorkspaceList: %v", err)
	}
	want := []WorkspaceInfo{
		{WorkspaceID: "w1", Number: 1, Label: "work", Focused: true, ActiveTabID: "w1:t1"},
		{WorkspaceID: "w2", Number: 2, Label: "notes", Focused: false, ActiveTabID: "w2:t1"},
	}
	if len(workspaces) != len(want) || workspaces[0] != want[0] || workspaces[1] != want[1] {
		t.Errorf("WorkspaceList() = %+v, want %+v", workspaces, want)
	}
}

func TestWorkspaceFocusSendsWorkspaceID(t *testing.T) {
	var gotMethod string
	var gotParams map[string]any
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		gotMethod = req["method"].(string)
		gotParams = req["params"].(map[string]any)
		reply(t, conn, req, map[string]any{"type": "ok"})
	})

	if err := New(socketPath).WorkspaceFocus("w1"); err != nil {
		t.Fatalf("WorkspaceFocus: %v", err)
	}
	if gotMethod != "workspace.focus" {
		t.Errorf("method = %q, want workspace.focus", gotMethod)
	}
	if gotParams["workspace_id"] != "w1" {
		t.Errorf("params.workspace_id = %v, want w1", gotParams["workspace_id"])
	}
}

func TestWorkspaceCreateSendsFocusTrue(t *testing.T) {
	var gotParams map[string]any
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		gotParams = req["params"].(map[string]any)
		reply(t, conn, req, map[string]any{
			"type": "workspace_info",
			"workspace": map[string]any{
				"workspace_id": "w3", "number": 3, "label": "scratch", "focused": true, "active_tab_id": "w3:t1",
			},
		})
	})

	ws, err := New(socketPath).WorkspaceCreate("scratch", "/tmp/scratch")
	if err != nil {
		t.Fatalf("WorkspaceCreate: %v", err)
	}
	if gotParams["label"] != "scratch" || gotParams["cwd"] != "/tmp/scratch" || gotParams["focus"] != true {
		t.Errorf("params = %v", gotParams)
	}
	want := WorkspaceInfo{WorkspaceID: "w3", Number: 3, Label: "scratch", Focused: true, ActiveTabID: "w3:t1"}
	if ws != want {
		t.Errorf("WorkspaceCreate() = %+v, want %+v", ws, want)
	}
}

func TestWorkspaceCreateOmitsCwdWhenEmpty(t *testing.T) {
	var gotParams map[string]any
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		gotParams = req["params"].(map[string]any)
		reply(t, conn, req, map[string]any{
			"type": "workspace_info",
			"workspace": map[string]any{
				"workspace_id": "w4", "number": 4, "label": "scratch2", "focused": true, "active_tab_id": "w4:t1",
			},
		})
	})

	if _, err := New(socketPath).WorkspaceCreate("scratch2", ""); err != nil {
		t.Fatalf("WorkspaceCreate: %v", err)
	}
	if _, ok := gotParams["cwd"]; ok {
		t.Errorf("params = %v, want no cwd key", gotParams)
	}
}

func TestPluginList(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type": "plugin_list",
			"plugins": []map[string]any{
				{"plugin_id": "blf", "plugin_root": "/home/david/.local/share/blf/herdr-plugin"},
			},
		})
	})

	plugins, err := New(socketPath).PluginList("blf")
	if err != nil {
		t.Fatalf("PluginList: %v", err)
	}
	want := []PluginInfo{{PluginID: "blf", PluginRoot: "/home/david/.local/share/blf/herdr-plugin"}}
	if len(plugins) != 1 || plugins[0] != want[0] {
		t.Errorf("PluginList() = %+v, want %+v", plugins, want)
	}
}

func TestPluginListEmptyMeansNotLinked(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		reply(t, conn, req, map[string]any{
			"type":    "plugin_list",
			"plugins": []map[string]any{},
		})
	})

	plugins, err := New(socketPath).PluginList("blf")
	if err != nil {
		t.Fatalf("PluginList: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("PluginList() = %+v, want empty", plugins)
	}
}

func TestMissingRequiredFieldIsNotUnderstood(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		// agent_status is required on pane_info but omitted here.
		reply(t, conn, req, map[string]any{
			"type": "pane_info",
			"pane": map[string]any{
				"pane_id": "w1:p1", "terminal_id": "term1", "workspace_id": "w1", "tab_id": "w1:t1",
				"focused": true, "revision": 1,
			},
		})
	})

	_, err := New(socketPath).PaneGet("w1:p1")
	wantErr := "herdr socket reply not understood (blf tested with herdr 0.9.3); run blf herdr doctor"
	if err == nil || err.Error() != wantErr {
		t.Fatalf("err = %v, want %q", err, wantErr)
	}
}

func TestAPIErrorSurfaces(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		replyAPIError(t, conn, req, "pane_not_found", "no such pane: w9:p9")
	})

	_, err := New(socketPath).PaneGet("w9:p9")
	if err == nil || !strings.Contains(err.Error(), "pane_not_found") || !strings.Contains(err.Error(), "no such pane: w9:p9") {
		t.Fatalf("err = %v", err)
	}
}

func TestServerNotRespondingFailsAfterTimeout(t *testing.T) {
	socketPath := startFakeServer(t, func(conn net.Conn, req map[string]any) {
		// Never reply; let the client's own deadline fire.
	})

	start := time.Now()
	_, err := New(socketPath).Ping()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed > 2*requestTimeout {
		t.Errorf("took %v, want roughly the %v request timeout", elapsed, requestTimeout)
	}
}
