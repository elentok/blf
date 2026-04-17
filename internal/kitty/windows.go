package kitty

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	activeWindowStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	lastFocusedWindowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

func ListOSWindows(d Deps) ([]OSWindow, error) {
	if d.RunCommand == nil {
		return nil, errors.New("kitty command runner is not configured")
	}

	output, err := d.RunCommand("kitty", "@", "ls")
	if err != nil {
		return nil, fmt.Errorf("run `kitty @ ls`: %w", err)
	}

	windows, err := ParseOSWindows(output)
	if err != nil {
		return nil, fmt.Errorf("parse `kitty @ ls`: %w", err)
	}

	return windows, nil
}

func ParseOSWindows(data []byte) ([]OSWindow, error) {
	var rawWindows []rawOSWindow
	if err := json.Unmarshal(data, &rawWindows); err != nil {
		return nil, err
	}

	windows := make([]OSWindow, 0, len(rawWindows))
	for _, raw := range rawWindows {
		rawTabs := raw.Tabs
		if len(rawTabs) == 0 {
			rawTabs = raw.TabsAlt
		}

		tabs := make([]Tab, 0, len(rawTabs))
		for _, rawTab := range rawTabs {
			tabs = append(tabs, Tab{
				ID:        rawTab.ID,
				IsActive:  rawTab.IsActive,
				IsFocused: rawTab.IsFocused,
				Title:     rawTab.Title,
			})
		}

		windows = append(windows, OSWindow{
			ID:          raw.ID,
			IsActive:    raw.IsActive,
			LastFocused: raw.LastFocused,
			Tabs:        tabs,
		})
	}

	return windows, nil
}

func FormatOSWindows(windows []OSWindow) string {
	var b strings.Builder
	for _, window := range windows {
		line := fmt.Sprintf("%d: %s", window.ID, joinTabTitles(window.Tabs))
		switch {
		case window.IsActive:
			line = activeWindowStyle.Render(line)
		case window.LastFocused:
			line = lastFocusedWindowStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func joinTabTitles(tabs []Tab) string {
	if len(tabs) == 0 {
		return ""
	}

	titles := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		titles = append(titles, tab.Title)
	}
	return strings.Join(titles, ", ")
}

func filterInactiveOSWindows(windows []OSWindow) []OSWindow {
	filtered := make([]OSWindow, 0, len(windows))
	for _, window := range windows {
		if window.IsActive {
			continue
		}
		filtered = append(filtered, window)
	}
	return filtered
}

func activeTabIDForOSWindow(windows []OSWindow, osWindowID string) (string, error) {
	for _, window := range windows {
		if strconv.Itoa(window.ID) != osWindowID {
			continue
		}

		for _, tab := range window.Tabs {
			if tab.IsFocused {
				return strconv.Itoa(tab.ID), nil
			}
		}

		for _, tab := range window.Tabs {
			if tab.IsActive {
				return strconv.Itoa(tab.ID), nil
			}
		}

		if len(window.Tabs) == 0 {
			return "", fmt.Errorf("kitty os window %s has no tabs", osWindowID)
		}

		return strconv.Itoa(window.Tabs[0].ID), nil
	}

	return "", fmt.Errorf("kitty os window %s not found", osWindowID)
}

func matchSessionExpr(path string) string {
	return "session:^" + regexp.QuoteMeta(path) + "$"
}
