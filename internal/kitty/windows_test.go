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
				{"id":10,"is_active":true,"is_focused":true,"title":"shell"},
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
