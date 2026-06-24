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
		name  string
		agent string
		title string
		want  Status
	}{
		{name: "braille spinner means working", agent: "claude", title: "⠉ Reviewing draw_tab", want: StatusWorking},
		{name: "no spinner means idle", agent: "claude", title: "Reviewing draw_tab", want: StatusIdle},
		{name: "non-braille decoration is idle", agent: "claude", title: "✳ Ready", want: StatusIdle},
		{name: "opencode is always idle even with spinner", agent: "opencode", title: "⠉ Working", want: StatusIdle},
		{name: "empty title is idle", agent: "codex", title: "", want: StatusIdle},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusForAgent(tc.agent, tc.title); got != tc.want {
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

func TestFormatAgentChoicesAndSelectionRoundTrip(t *testing.T) {
	agents := []Agent{
		{ID: 101, Name: "codex", Status: StatusWorking, Dir: "proj-b", Title: "busy"},
		{ID: 100, Name: "claude", Status: StatusIdle, Dir: "proj-a", Title: "idle"},
	}

	choices := formatAgentChoices(agents)
	lines := strings.Split(strings.TrimRight(choices, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 choice lines, got %d: %q", len(lines), choices)
	}

	// Hidden id field is the first tab-delimited column.
	id, rest, found := strings.Cut(lines[0], "\t")
	if !found || id != "101" {
		t.Fatalf("first line id field = %q (found=%v)", id, found)
	}
	plainRest := ansiPattern.ReplaceAllString(rest, "")
	if !strings.Contains(plainRest, "proj-b") || !strings.Contains(plainRest, "busy") || !strings.Contains(plainRest, "codex") {
		t.Fatalf("row missing expected fields: %q", plainRest)
	}

	// Selection round-trips back to the id.
	gotID, err := parseAgentSelection(lines[0])
	if err != nil {
		t.Fatalf("parseAgentSelection: %v", err)
	}
	if gotID != "101" {
		t.Fatalf("parseAgentSelection = %q, want 101", gotID)
	}
}

func TestStatusGlyphDiffersByStatus(t *testing.T) {
	working := ansiPattern.ReplaceAllString(statusGlyph(StatusWorking), "")
	idle := ansiPattern.ReplaceAllString(statusGlyph(StatusIdle), "")
	if working != "●" {
		t.Fatalf("working glyph = %q, want ●", working)
	}
	if idle != "○" {
		t.Fatalf("idle glyph = %q, want ○", idle)
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

func TestGotoAgentEmptyShowsError(t *testing.T) {
	var calls []string
	d := Deps{
		LookupEnv: func(string) (string, bool) { return "", false },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			if name == "kitty" && strings.Join(args, " ") == "@ ls" {
				return agentsLS(), nil
			}
			return []byte{}, nil
		},
	}

	if err := GotoAgent(d); err != nil {
		t.Fatalf("GotoAgent: %v", err)
	}

	want := `kitten @ action show_error "blf kitty goto-agent" "No agent windows"`
	found := false
	for _, c := range calls {
		if c == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected show_error call, got %v", calls)
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
