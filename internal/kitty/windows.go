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
	kittyLSTabStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba9")).Background(lipgloss.Color("#32283b"))
	kittyLSWindowStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2b0")).Background(lipgloss.Color("#33313b"))
	kittyLSKeyStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa"))
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
			windows := make([]Window, 0, len(rawTab.Windows))
			for _, rawWindow := range rawTab.Windows {
				foregroundProcesses := make([]ForegroundProcess, 0, len(rawWindow.ForegroundProcesses))
				for _, rawProcess := range rawWindow.ForegroundProcesses {
					foregroundProcesses = append(foregroundProcesses, ForegroundProcess{
						PID:     rawProcess.PID,
						Cmdline: rawProcess.Cmdline,
						Cwd:     rawProcess.Cwd,
					})
				}

				windows = append(windows, Window{
					ID:                        rawWindow.ID,
					IsActive:                  rawWindow.IsActive,
					Cmdline:                   rawWindow.Cmdline,
					ForegroundProcesses:       foregroundProcesses,
					Title:                     rawWindow.Title,
					SessionName:               rawWindow.SessionName,
					LastReportedCmdline:       rawWindow.LastReportedCmdline,
					HasActivitySinceLastFocus: rawWindow.HasActivitySinceLastFocus,
					LastFocusedAt:             rawWindow.LastFocusedAt,
					Cwd:                       rawWindow.Cwd,
				})
			}

			tabs = append(tabs, Tab{
				ID:          rawTab.ID,
				IsActive:    rawTab.IsActive,
				IsFocused:   rawTab.IsFocused,
				Title:       rawTab.Title,
				SessionName: rawTab.SessionName,
				Windows:     windows,
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

func FormatKittyLS(windows []OSWindow) string {
	var b strings.Builder
	for _, osWindow := range windows {
		fmt.Fprintf(&b, "- OS Window %d%s\n", osWindow.ID, activeSuffix(osWindow.IsActive))
		for _, tab := range osWindow.Tabs {
			b.WriteString(kittyLSTabStyle.Render(fmt.Sprintf("  - Tab %d%s: %s", tab.ID, activeSuffix(tab.IsActive), tab.Title)))
			b.WriteByte('\n')
			for _, window := range tab.Windows {
				b.WriteString(kittyLSWindowStyle.Render(fmt.Sprintf(
					"    - Window %d%s%s%s: %s",
					window.ID,
					activeSuffix(window.IsActive),
					activitySuffix(window.HasActivitySinceLastFocus),
					sessionSuffix(window.SessionName),
					window.Title,
				)))
				b.WriteByte('\n')
				b.WriteString(formatKeyValueLine("      ", "cmdline", formatCommandLine(window.Cmdline)))
				b.WriteString(formatKeyValueLine("      ", "last_reported_cmdline", window.LastReportedCmdline))
				b.WriteString(formatKeyValueLine("      ", "Foreground processes", ""))
				for _, proc := range window.ForegroundProcesses {
					fmt.Fprintf(&b, "        - Proc %d:\n", proc.PID)
					b.WriteString(formatKeyValueLine("          ", "cmdline", formatForegroundProcessCommandLine(proc.Cmdline)))
					b.WriteString(formatKeyValueLine("          ", "cwd", proc.Cwd))
				}
			}
		}
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

func activeSuffix(active bool) string {
	if !active {
		return ""
	}
	return " (active)"
}

func activitySuffix(active bool) string {
	if !active {
		return ""
	}
	return " *"
}

func sessionSuffix(sessionName string) string {
	if strings.TrimSpace(sessionName) == "" {
		return ""
	}
	return fmt.Sprintf(" [%s]", sessionName)
}

func formatCommandLine(cmdline []string) string {
	if len(cmdline) == 0 {
		return "[]"
	}
	return strings.Join(cmdline, " ")
}

func formatForegroundProcessCommandLine(cmdline []string) string {
	if len(cmdline) == 0 {
		return "[]"
	}
	if len(cmdline) == 1 {
		return cmdline[0]
	}
	return fmt.Sprintf("%s... (%d more lines)", cmdline[0], len(cmdline)-1)
}

func formatKeyValueLine(indent, key, value string) string {
	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(kittyLSKeyStyle.Render("- " + key + ":"))
	if value != "" {
		b.WriteByte(' ')
		b.WriteString(value)
	}
	b.WriteByte('\n')
	return b.String()
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
