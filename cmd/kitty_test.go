package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestParseKittyOSWindowsSupportsTabsKeyVariants(t *testing.T) {
	t.Run("tabs", func(t *testing.T) {
		windows, err := parseKittyOSWindows([]byte(`[
			{"id":1,"is_active":true,"last_focused":false,"tabs":[
				{"id":10,"is_active":true,"is_focused":true,"title":"shell"},
				{"id":11,"is_active":false,"is_focused":false,"title":"logs"}
			]}
		]`))
		if err != nil {
			t.Fatalf("parseKittyOSWindows returned error: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("window count = %d", len(windows))
		}
		if got := joinKittyTabTitles(windows[0].Tabs); got != "shell, logs" {
			t.Fatalf("tab titles = %q", got)
		}
		if windows[0].Tabs[0].ID != 10 || !windows[0].Tabs[0].IsFocused {
			t.Fatalf("first tab = %+v", windows[0].Tabs[0])
		}
	})

	t.Run("tabs colon", func(t *testing.T) {
		windows, err := parseKittyOSWindows([]byte(`[
			{"id":2,"is_active":false,"last_focused":true,"tabs:":[{"id":20,"is_active":true,"is_focused":false,"title":"editor"}]}
		]`))
		if err != nil {
			t.Fatalf("parseKittyOSWindows returned error: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("window count = %d", len(windows))
		}
		if got := joinKittyTabTitles(windows[0].Tabs); got != "editor" {
			t.Fatalf("tab titles = %q", got)
		}
	})
}

func TestFormatKittyOSWindowsStylesRows(t *testing.T) {
	got := formatKittyOSWindows([]kittyOSWindow{
		{ID: 1, IsActive: true, Tabs: []kittyTab{{Title: "shell"}, {Title: "logs"}}},
		{ID: 2, LastFocused: true, Tabs: []kittyTab{{Title: "editor"}}},
		{ID: 3, Tabs: []kittyTab{{Title: "plain"}}},
	})

	want := "" +
		"\x1b[1;34m1: shell, logs\x1b[m\n" +
		"\x1b[38;5;214m2: editor\x1b[m\n" +
		"3: plain\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFilterInactiveKittyOSWindowsRemovesActiveWindow(t *testing.T) {
	got := filterInactiveKittyOSWindows([]kittyOSWindow{
		{ID: 1, IsActive: true, Tabs: []kittyTab{{Title: "active"}}},
		{ID: 2, LastFocused: true, Tabs: []kittyTab{{Title: "other"}}},
		{ID: 3, Tabs: []kittyTab{{Title: "plain"}}},
	})

	if len(got) != 2 {
		t.Fatalf("window count = %d", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("filtered windows = %+v", got)
	}
}

func TestParseKittyOSWindowIDStripsANSI(t *testing.T) {
	got, err := parseKittyOSWindowID("\x1b[1;34m12: shell, logs\x1b[0m")
	if err != nil {
		t.Fatalf("parseKittyOSWindowID returned error: %v", err)
	}
	if got != "12" {
		t.Fatalf("id = %q", got)
	}
}

func TestParseKittyOSWindowIDRejectsInvalidSelection(t *testing.T) {
	_, err := parseKittyOSWindowID("not a selection")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunKittyListOSWindows(t *testing.T) {
	out := &strings.Builder{}
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[
				{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
			]`), nil
		},
		stdout: out,
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"list-os-windows"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if got := out.String(); got != "\x1b[1;34m7: shell\x1b[m\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRunKittyGotoOSWindowWithExplicitID(t *testing.T) {
	var commands []string
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ ls":
				return []byte(`[
					{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
				]`), nil
			case name == "kitten" && strings.Join(args, " ") == "@ focus-tab --match id:70":
				return []byte{}, nil
			}
			t.Fatalf("unexpected command: %s %v", name, args)
			return nil, nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"goto-os-window", "7"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(commands, "\n") != "kitty @ ls\nkitten @ focus-tab --match id:70" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestRunKittyTargetsRoutesToDependency(t *testing.T) {
	var got []string
	d := deps{
		runKittyTargets: func(args []string) error {
			got = append([]string{}, args...)
			return nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"targets", "--overlay", "--target", "17"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(got, " ") != "--overlay --target 17" {
		t.Fatalf("kitty targets called with %v", got)
	}
}

func TestRunKittyGotoOSWindowRejectsInvalidID(t *testing.T) {
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name == "kitty" && strings.Join(args, " ") == "@ ls" {
				return []byte(`[]`), nil
			}
			t.Fatalf("unexpected command: %s %v", name, args)
			return nil, nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"goto-os-window", "abc"}, d)
	if err == nil || !strings.Contains(err.Error(), `invalid kitty os window id "abc"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunKittyGotoOSWindowShowsErrorWhenNoOtherWindows(t *testing.T) {
	var commands []string
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ ls":
				return []byte(`[
					{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
				]`), nil
			case name == "kitten" && strings.Join(args, " ") == `@ action show_error "blf kitty" "No other kitty windows"`:
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"goto-os-window"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(commands, "\n") != "kitty @ ls\nkitten @ action show_error \"blf kitty\" \"No other kitty windows\"" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestActiveKittyTabIDForOSWindowPrefersFocusedThenActive(t *testing.T) {
	windows := []kittyOSWindow{
		{
			ID: 5,
			Tabs: []kittyTab{
				{ID: 50, IsActive: true, Title: "active"},
				{ID: 51, IsFocused: true, Title: "focused"},
			},
		},
	}

	got, err := activeKittyTabIDForOSWindow(windows, "5")
	if err != nil {
		t.Fatalf("activeKittyTabIDForOSWindow returned error: %v", err)
	}
	if got != "51" {
		t.Fatalf("tab id = %q", got)
	}
}

func TestActiveKittyTabIDForOSWindowErrorsWhenMissing(t *testing.T) {
	_, err := activeKittyTabIDForOSWindow([]kittyOSWindow{}, "9")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestListKittyOSWindowsWrapsCommandErrors(t *testing.T) {
	d := deps{
		runCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"list-os-windows"}, d)
	if err == nil || !strings.Contains(err.Error(), "run `kitty @ ls`") {
		t.Fatalf("error = %v", err)
	}
}
