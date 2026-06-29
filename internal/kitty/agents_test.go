package kitty

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDetectAgentName(t *testing.T) {
	tests := []struct {
		name     string
		window   Window
		wantName string
		wantOK   bool
	}{
		{
			name:     "last reported cmdline is the agent",
			window:   Window{LastReportedCmdline: "claude"},
			wantName: "claude",
			wantOK:   true,
		},
		{
			name: "path containing agent name is not a match",
			window: Window{
				LastReportedCmdline: "fish",
				ForegroundProcesses: []ForegroundProcess{
					{Cmdline: []string{"cat", "/private/tmp/claude-501/notes.txt"}},
				},
			},
			wantOK: false,
		},
		{
			name: "agent behind a shell wrapper is matched",
			window: Window{
				ForegroundProcesses: []ForegroundProcess{
					{Cmdline: []string{"/bin/sh", "/usr/bin/command", "claude"}},
				},
			},
			wantName: "claude",
			wantOK:   true,
		},
		{
			name: "foreground process basename is matched",
			window: Window{
				LastReportedCmdline: "fish",
				ForegroundProcesses: []ForegroundProcess{
					{Cmdline: []string{"/opt/homebrew/bin/codex"}},
				},
			},
			wantName: "codex",
			wantOK:   true,
		},
		{
			name:     "cursor-agent hyphenated name is matched",
			window:   Window{LastReportedCmdline: "/usr/local/bin/cursor-agent"},
			wantName: "cursor-agent",
			wantOK:   true,
		},
		{
			name:   "plain shell is not an agent",
			window: Window{LastReportedCmdline: "fish", ForegroundProcesses: []ForegroundProcess{{Cmdline: []string{"fish"}}}},
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, ok := detectAgentName(tc.window)
			if ok != tc.wantOK {
				t.Fatalf("detectAgentName ok = %v, want %v", ok, tc.wantOK)
			}
			if name != tc.wantName {
				t.Fatalf("detectAgentName name = %q, want %q", name, tc.wantName)
			}
		})
	}
}

func TestStatusForAgent(t *testing.T) {
	tests := []struct {
		name     string
		agent    string
		title    string
		userVars map[string]string
		want     Status
	}{
		// Title-only fallback (no user var) — today's behavior.
		{name: "braille spinner means working", agent: "claude", title: "⠉ Reviewing draw_tab", want: StatusWorking},
		{name: "no spinner means idle", agent: "claude", title: "Reviewing draw_tab", want: StatusIdle},
		{name: "non-braille decoration is idle", agent: "claude", title: "✳ Ready", want: StatusIdle},
		{name: "opencode is always idle even with spinner", agent: "opencode", title: "⠉ Working", want: StatusIdle},
		{name: "empty title is idle", agent: "codex", title: "", want: StatusIdle},

		// AGENT_STATE user var is authoritative when recognized.
		{name: "waiting var wins over idle title", agent: "claude", title: "Reviewing draw_tab", userVars: map[string]string{"AGENT_STATE": "waiting"}, want: StatusWaiting},
		{name: "waiting var wins over spinner title", agent: "claude", title: "⠉ Reviewing", userVars: map[string]string{"AGENT_STATE": "waiting"}, want: StatusWaiting},
		{name: "working var wins over idle title", agent: "claude", title: "Reviewing", userVars: map[string]string{"AGENT_STATE": "working"}, want: StatusWorking},
		{name: "idle var wins over spinner title", agent: "claude", title: "⠉ Reviewing", userVars: map[string]string{"AGENT_STATE": "idle"}, want: StatusIdle},
		{name: "waiting var wins for opencode", agent: "opencode", title: "", userVars: map[string]string{"AGENT_STATE": "waiting"}, want: StatusWaiting},

		// Unknown / empty var falls back to the title signal.
		{name: "unknown var falls back to title", agent: "claude", title: "⠉ Reviewing", userVars: map[string]string{"AGENT_STATE": "bogus"}, want: StatusWorking},
		{name: "empty var falls back to title", agent: "claude", title: "⠉ Reviewing", userVars: map[string]string{"AGENT_STATE": ""}, want: StatusWorking},
		{name: "unrelated vars fall back to title", agent: "claude", title: "Reviewing", userVars: map[string]string{"OTHER": "waiting"}, want: StatusIdle},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusForAgent(tc.agent, tc.title, tc.userVars); got != tc.want {
				t.Fatalf("statusForAgent = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCleanTitle(t *testing.T) {
	if got := cleanTitle("⠉ Reviewing draw_tab"); got != "Reviewing draw_tab" {
		t.Fatalf("cleanTitle = %q", got)
	}
	if got := cleanTitle("Reviewing draw_tab"); got != "Reviewing draw_tab" {
		t.Fatalf("cleanTitle = %q", got)
	}
	if got := cleanTitle("✳ Reviewing draw_tab"); got != "Reviewing draw_tab" {
		t.Fatalf("cleanTitle = %q", got)
	}
}

// agentsLS builds a `kitty @ ls` JSON payload with the given windows in a
// single OS window / tab.
func agentsLS(windows ...rawWindow) []byte {
	payload := []rawOSWindow{{
		ID:       1,
		IsActive: true,
		Tabs: []rawTab{{
			ID:       10,
			IsActive: true,
			Windows:  windows,
		}},
	}}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return data
}

func depsWithLS(t *testing.T, data []byte, env map[string]string) Deps {
	t.Helper()
	return Deps{
		LookupEnv: func(key string) (string, bool) {
			v, ok := env[key]
			return v, ok
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return data, nil
		},
	}
}

func TestListAgentsDropsCurrentWindowAndSorts(t *testing.T) {
	data := agentsLS(
		rawWindow{ID: 100, LastReportedCmdline: "claude", Title: "idle one", Cwd: "/home/me/proj-a", LastFocusedAt: 50},
		rawWindow{ID: 101, LastReportedCmdline: "codex", Title: "⠉ busy", Cwd: "/home/me/proj-b", LastFocusedAt: 10},
		rawWindow{ID: 102, LastReportedCmdline: "claude", Title: "idle two", Cwd: "/home/me/proj-c", LastFocusedAt: 99},
		rawWindow{ID: 103, LastReportedCmdline: "fish", Title: "shell", Cwd: "/home/me/proj-d", LastFocusedAt: 100},
		rawWindow{ID: 999, LastReportedCmdline: "claude", Title: "current", Cwd: "/home/me/proj-e", LastFocusedAt: 1000},
	)

	d := depsWithLS(t, data, map[string]string{"KITTY_WINDOW_ID": "999"})

	agents, err := ListAgents(d)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}

	var gotIDs []int
	for _, a := range agents {
		gotIDs = append(gotIDs, a.ID)
	}
	// working first (101), then idle by recency (102 has 99 > 100's 50). The
	// plain shell (103) is excluded and the current window (999) is dropped.
	want := []int{101, 102, 100}
	if len(gotIDs) != len(want) {
		t.Fatalf("agent ids = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("agent ids = %v, want %v", gotIDs, want)
		}
	}

	if agents[0].Status != StatusWorking {
		t.Fatalf("first agent status = %q, want working", agents[0].Status)
	}
	if agents[0].Dir != "proj-b" {
		t.Fatalf("first agent dir = %q, want proj-b", agents[0].Dir)
	}
	if agents[0].Title != "busy" {
		t.Fatalf("first agent title = %q, want busy (spinner stripped)", agents[0].Title)
	}
}

func TestListAgentsSortsWaitingFirst(t *testing.T) {
	uv := func(state string) map[string]string { return map[string]string{"AGENT_STATE": state} }
	data := agentsLS(
		rawWindow{ID: 100, LastReportedCmdline: "claude", Title: "idle old", UserVars: uv("idle"), LastFocusedAt: 10},
		rawWindow{ID: 101, LastReportedCmdline: "claude", Title: "working", UserVars: uv("working"), LastFocusedAt: 20},
		rawWindow{ID: 102, LastReportedCmdline: "claude", Title: "waiting old", UserVars: uv("waiting"), LastFocusedAt: 30},
		rawWindow{ID: 103, LastReportedCmdline: "claude", Title: "idle new", UserVars: uv("idle"), LastFocusedAt: 99},
		rawWindow{ID: 104, LastReportedCmdline: "claude", Title: "waiting new", UserVars: uv("waiting"), LastFocusedAt: 40},
	)
	d := depsWithLS(t, data, map[string]string{})

	agents, err := ListAgents(d)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}

	var gotIDs []int
	for _, a := range agents {
		gotIDs = append(gotIDs, a.ID)
	}
	// waiting (by recency: 104>102) -> working (101) -> idle (by recency: 103>100).
	want := []int{104, 102, 101, 103, 100}
	if len(gotIDs) != len(want) {
		t.Fatalf("agent ids = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("agent ids = %v, want %v", gotIDs, want)
		}
	}
}

func TestListAgentsKeepsCurrentWindowWhenEnvUnset(t *testing.T) {
	data := agentsLS(
		rawWindow{ID: 100, LastReportedCmdline: "claude", Title: "one", Cwd: "/home/me/a"},
	)
	d := depsWithLS(t, data, map[string]string{})

	agents, err := ListAgents(d)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != 100 {
		t.Fatalf("agents = %#v, want single agent 100", agents)
	}
}

func TestStatusGlyphDiffersByStatus(t *testing.T) {
	working := ansiPattern.ReplaceAllString(statusGlyph(StatusWorking), "")
	waiting := ansiPattern.ReplaceAllString(statusGlyph(StatusWaiting), "")
	idle := ansiPattern.ReplaceAllString(statusGlyph(StatusIdle), "")

	if working != workingGlyph {
		t.Fatalf("working glyph = %q, want %q", working, workingGlyph)
	}
	if waiting != waitingGlyph {
		t.Fatalf("waiting glyph = %q, want %q", waiting, waitingGlyph)
	}
	if idle != idleGlyph {
		t.Fatalf("idle glyph = %q, want %q", idle, idleGlyph)
	}
	if working == waiting || working == idle || waiting == idle {
		t.Fatalf("status glyphs are not distinct: working=%q waiting=%q idle=%q", working, waiting, idle)
	}
}

func TestListAgentsCommandJSONContract(t *testing.T) {
	data := agentsLS(
		rawWindow{ID: 100, LastReportedCmdline: "claude", Title: "⠉ Reviewing", Cwd: "/home/me/proj-a", SessionName: "work", LastFocusedAt: 5},
	)
	d := depsWithLS(t, data, map[string]string{})
	var out bytes.Buffer
	d.Stdout = &out

	if err := ListAgentsCommand(true, d); err != nil {
		t.Fatalf("ListAgentsCommand: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(got))
	}

	obj := got[0]
	wantKeys := []string{"id", "agent", "status", "dir", "title", "session"}
	for _, key := range wantKeys {
		if _, ok := obj[key]; !ok {
			t.Fatalf("JSON object missing key %q: %#v", key, obj)
		}
	}
	if _, ok := obj["last_focused_at"]; ok {
		t.Fatalf("JSON should not expose last_focused_at: %#v", obj)
	}
	if obj["agent"] != "claude" {
		t.Fatalf("agent = %v, want claude", obj["agent"])
	}
	if obj["status"] != "working" {
		t.Fatalf("status = %v, want working", obj["status"])
	}
	if obj["dir"] != "proj-a" {
		t.Fatalf("dir = %v, want proj-a", obj["dir"])
	}
	if obj["title"] != "Reviewing" {
		t.Fatalf("title = %v, want Reviewing (spinner stripped)", obj["title"])
	}
	if obj["session"] != "work" {
		t.Fatalf("session = %v, want work", obj["session"])
	}
}

func TestListAgentsCommandJSONExposesWaiting(t *testing.T) {
	data := agentsLS(
		rawWindow{ID: 100, LastReportedCmdline: "claude", Title: "Reviewing", UserVars: map[string]string{"AGENT_STATE": "waiting"}, LastFocusedAt: 5},
	)
	d := depsWithLS(t, data, map[string]string{})
	var out bytes.Buffer
	d.Stdout = &out

	if err := ListAgentsCommand(true, d); err != nil {
		t.Fatalf("ListAgentsCommand: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(got))
	}
	if got[0]["status"] != "waiting" {
		t.Fatalf("status = %v, want waiting", got[0]["status"])
	}
}

func TestRenderAgentPreviewUsesGetText(t *testing.T) {
	var gotArgs string
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			gotArgs = name + " " + strings.Join(args, " ")
			return []byte("screen contents"), nil
		},
	}

	out, err := RenderAgentPreview("42", d)
	if err != nil {
		t.Fatalf("RenderAgentPreview: %v", err)
	}
	if out != "screen contents" {
		t.Fatalf("preview = %q", out)
	}
	if gotArgs != "kitty @ get-text --match id:42 --extent screen" {
		t.Fatalf("command = %q", gotArgs)
	}
}

func TestListAgentsCommandJSONEmptyIsArray(t *testing.T) {
	data := agentsLS()
	d := depsWithLS(t, data, map[string]string{})
	var out bytes.Buffer
	d.Stdout = &out

	if err := ListAgentsCommand(true, d); err != nil {
		t.Fatalf("ListAgentsCommand: %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty JSON = %q, want []", out.String())
	}
}
