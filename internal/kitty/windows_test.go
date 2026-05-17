package kitty

import (
	"errors"
	"strings"
	"testing"
)

func TestParseOSWindowsSupportsTabsKeyVariants(t *testing.T) {
	t.Run("tabs", func(t *testing.T) {
		windows, err := ParseOSWindows([]byte(`[
			{"id":1,"is_active":true,"last_focused":false,"tabs":[
				{"id":10,"is_active":true,"is_focused":true,"title":"shell","windows":[{"id":100,"is_active":true,"title":"editor","cmdline":["nvim","main.go"],"last_reported_cmdline":"nvim main.go","has_activity_since_last_focus":true,"cwd":"/work","foreground_processes":[{"pid":4321,"cmdline":["go","test","./..."],"cwd":"/work"}],"last_focused_at":123.5,"session_name":"proj"}]},
				{"id":11,"is_active":false,"is_focused":false,"title":"logs"}
			]}
		]`))
		if err != nil {
			t.Fatalf("ParseOSWindows returned error: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("window count = %d", len(windows))
		}
		if got := joinTabTitles(windows[0].Tabs); got != "shell, logs" {
			t.Fatalf("tab titles = %q", got)
		}
		if windows[0].Tabs[0].ID != 10 || !windows[0].Tabs[0].IsFocused {
			t.Fatalf("first tab = %+v", windows[0].Tabs[0])
		}
		if len(windows[0].Tabs[0].Windows) != 1 || windows[0].Tabs[0].Windows[0].LastFocusedAt != 123.5 {
			t.Fatalf("tab windows = %+v", windows[0].Tabs[0].Windows)
		}
		if windows[0].Tabs[0].Windows[0].SessionName != "proj" {
			t.Fatalf("window session name = %q", windows[0].Tabs[0].Windows[0].SessionName)
		}
		if windows[0].Tabs[0].Windows[0].ID != 100 || !windows[0].Tabs[0].Windows[0].IsActive {
			t.Fatalf("window = %+v", windows[0].Tabs[0].Windows[0])
		}
		if got := windows[0].Tabs[0].Windows[0].Title; got != "editor" {
			t.Fatalf("window title = %q", got)
		}
		if got := windows[0].Tabs[0].Windows[0].LastReportedCmdline; got != "nvim main.go" {
			t.Fatalf("last reported cmdline = %q", got)
		}
		if !windows[0].Tabs[0].Windows[0].HasActivitySinceLastFocus {
			t.Fatalf("expected activity flag on window: %+v", windows[0].Tabs[0].Windows[0])
		}
		if got := windows[0].Tabs[0].Windows[0].Cwd; got != "/work" {
			t.Fatalf("window cwd = %q", got)
		}
		if len(windows[0].Tabs[0].Windows[0].ForegroundProcesses) != 1 {
			t.Fatalf("foreground processes = %+v", windows[0].Tabs[0].Windows[0].ForegroundProcesses)
		}
		if got := windows[0].Tabs[0].Windows[0].ForegroundProcesses[0].PID; got != 4321 {
			t.Fatalf("foreground process pid = %d", got)
		}
	})

	t.Run("tabs colon", func(t *testing.T) {
		windows, err := ParseOSWindows([]byte(`[
			{"id":2,"is_active":false,"last_focused":true,"tabs:":[{"id":20,"is_active":true,"is_focused":false,"title":"editor"}]}
		]`))
		if err != nil {
			t.Fatalf("ParseOSWindows returned error: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("window count = %d", len(windows))
		}
		if got := joinTabTitles(windows[0].Tabs); got != "editor" {
			t.Fatalf("tab titles = %q", got)
		}
	})
}

func TestFormatOSWindowsStylesRows(t *testing.T) {
	got := FormatOSWindows([]OSWindow{
		{ID: 1, IsActive: true, Tabs: []Tab{{Title: "shell"}, {Title: "logs"}}},
		{ID: 2, LastFocused: true, Tabs: []Tab{{Title: "editor"}}},
		{ID: 3, Tabs: []Tab{{Title: "plain"}}},
	})

	want := "" +
		"\x1b[1;34m1: shell, logs\x1b[m\n" +
		"\x1b[38;5;214m2: editor\x1b[m\n" +
		"3: plain\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatKittyLSRendersTree(t *testing.T) {
	got := FormatKittyLS([]OSWindow{
		{
			ID:       1,
			IsActive: true,
			Tabs: []Tab{
				{
					ID:       10,
					IsActive: true,
					Title:    "shell",
					Windows: []Window{
						{
							ID:                        100,
							IsActive:                  true,
							Title:                     "editor",
							SessionName:               "proj",
							Cmdline:                   []string{"nvim", "main.go"},
							LastReportedCmdline:       "nvim main.go",
							HasActivitySinceLastFocus: true,
							ForegroundProcesses: []ForegroundProcess{
								{PID: 4321, Cmdline: []string{"go", "test", "./..."}, Cwd: "/work"},
							},
						},
					},
				},
			},
		},
	})

	want := "" +
		"- OS Window 1 (active)\n" +
		"\x1b[38;2;243;139;169;48;2;50;40;59m  - Tab 10 (active): shell\x1b[m\n" +
		"\x1b[38;2;249;226;176;48;2;51;49;59m    - Window 100 (active) * [proj]: editor\x1b[m\n" +
		"      \x1b[38;2;137;180;250m- cmdline:\x1b[m nvim main.go\n" +
		"      \x1b[38;2;137;180;250m- last_reported_cmdline:\x1b[m nvim main.go\n" +
		"      \x1b[38;2;137;180;250m- Foreground processes:\x1b[m\n" +
		"        - Proc 4321:\n" +
		"          \x1b[38;2;137;180;250m- cmdline:\x1b[m go... (2 more lines)\n" +
		"          \x1b[38;2;137;180;250m- cwd:\x1b[m /work\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFilterInactiveOSWindowsRemovesActiveWindow(t *testing.T) {
	got := filterInactiveOSWindows([]OSWindow{
		{ID: 1, IsActive: true, Tabs: []Tab{{Title: "active"}}},
		{ID: 2, LastFocused: true, Tabs: []Tab{{Title: "other"}}},
		{ID: 3, Tabs: []Tab{{Title: "plain"}}},
	})

	if len(got) != 2 {
		t.Fatalf("window count = %d", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("filtered windows = %+v", got)
	}
}

func TestActiveTabIDForOSWindowPrefersFocusedThenActive(t *testing.T) {
	windows := []OSWindow{
		{
			ID: 5,
			Tabs: []Tab{
				{ID: 50, IsActive: true, Title: "active"},
				{ID: 51, IsFocused: true, Title: "focused"},
			},
		},
	}

	got, err := activeTabIDForOSWindow(windows, "5")
	if err != nil {
		t.Fatalf("activeTabIDForOSWindow returned error: %v", err)
	}
	if got != "51" {
		t.Fatalf("tab id = %q", got)
	}
}

func TestActiveTabIDForOSWindowErrorsWhenMissing(t *testing.T) {
	_, err := activeTabIDForOSWindow([]OSWindow{}, "9")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestListOSWindowsWrapsCommandErrors(t *testing.T) {
	d := Deps{
		RunCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}

	_, err := ListOSWindows(d)
	if err == nil || !strings.Contains(err.Error(), "run `kitty @ ls`") {
		t.Fatalf("error = %v", err)
	}
}
