package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	kittyListOSWindows   = "list-os-windows"
	kittyGotoOSWindowCmd = "goto-os-window"
)

var (
	kittyActiveWindowStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	kittyLastFocusedWindowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	kittyANSIPattern            = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

type kittyOSWindow struct {
	ID          int
	IsActive    bool
	LastFocused bool
	Tabs        []kittyTab
}

type kittyTab struct {
	ID        int
	IsActive  bool
	IsFocused bool
	Title     string
}

type kittyRawOSWindow struct {
	ID          int           `json:"id"`
	IsActive    bool          `json:"is_active"`
	LastFocused bool          `json:"last_focused"`
	Tabs        []kittyRawTab `json:"tabs"`
	TabsAlt     []kittyRawTab `json:"tabs:"`
}

type kittyRawTab struct {
	ID        int    `json:"id"`
	IsActive  bool   `json:"is_active"`
	IsFocused bool   `json:"is_focused"`
	Title     string `json:"title"`
}

func runKitty(args []string, d deps) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: blf kitty <list-os-windows|goto-os-window> [id]")
	}

	switch args[0] {
	case kittyListOSWindows:
		return runKittyListOSWindows(d)
	case kittyGotoOSWindowCmd:
		return runKittyGotoOSWindow(args[1:], d)
	default:
		return fmt.Errorf("unknown kitty command %q", args[0])
	}
}

func runKittyListOSWindows(d deps) error {
	windows, err := listKittyOSWindows(d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.stdout, formatKittyOSWindows(windows))
	return err
}

func runKittyGotoOSWindow(args []string, d deps) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: blf kitty goto-os-window [id]")
	}

	windows, err := listKittyOSWindows(d)
	if err != nil {
		return err
	}

	id := ""
	if len(args) == 1 {
		id = args[0]
	} else {
		otherWindows := filterInactiveKittyOSWindows(windows)
		if len(otherWindows) == 0 {
			return showKittyError(d, "blf kitty", "No other kitty windows")
		}

		selectedID, err := pickKittyOSWindow(otherWindows, d)
		if err != nil {
			return err
		}
		id = selectedID
		if id == "" {
			return nil
		}
	}

	if _, err := strconv.Atoi(id); err != nil {
		return fmt.Errorf("invalid kitty os window id %q", id)
	}

	tabID, err := activeKittyTabIDForOSWindow(windows, id)
	if err != nil {
		return err
	}

	if _, err := d.runCommand("kitten", "@", "focus-tab", "--match", "id:"+tabID); err != nil {
		return fmt.Errorf("focus kitty os window %s: %w", id, err)
	}

	return nil
}

func listKittyOSWindows(d deps) ([]kittyOSWindow, error) {
	if d.runCommand == nil {
		return nil, errors.New("kitty command runner is not configured")
	}

	output, err := d.runCommand("kitty", "@", "ls")
	if err != nil {
		return nil, fmt.Errorf("run `kitty @ ls`: %w", err)
	}

	windows, err := parseKittyOSWindows(output)
	if err != nil {
		return nil, fmt.Errorf("parse `kitty @ ls`: %w", err)
	}

	return windows, nil
}

func parseKittyOSWindows(data []byte) ([]kittyOSWindow, error) {
	var rawWindows []kittyRawOSWindow
	if err := json.Unmarshal(data, &rawWindows); err != nil {
		return nil, err
	}

	windows := make([]kittyOSWindow, 0, len(rawWindows))
	for _, raw := range rawWindows {
		rawTabs := raw.Tabs
		if len(rawTabs) == 0 {
			rawTabs = raw.TabsAlt
		}

		tabs := make([]kittyTab, 0, len(rawTabs))
		for _, rawTab := range rawTabs {
			tabs = append(tabs, kittyTab{
				ID:        rawTab.ID,
				IsActive:  rawTab.IsActive,
				IsFocused: rawTab.IsFocused,
				Title:     rawTab.Title,
			})
		}

		windows = append(windows, kittyOSWindow{
			ID:          raw.ID,
			IsActive:    raw.IsActive,
			LastFocused: raw.LastFocused,
			Tabs:        tabs,
		})
	}

	return windows, nil
}

func formatKittyOSWindows(windows []kittyOSWindow) string {
	var b strings.Builder
	for _, window := range windows {
		line := fmt.Sprintf("%d: %s", window.ID, joinKittyTabTitles(window.Tabs))
		switch {
		case window.IsActive:
			line = kittyActiveWindowStyle.Render(line)
		case window.LastFocused:
			line = kittyLastFocusedWindowStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func joinKittyTabTitles(tabs []kittyTab) string {
	if len(tabs) == 0 {
		return ""
	}

	titles := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		titles = append(titles, tab.Title)
	}
	return strings.Join(titles, ", ")
}

func showKittyError(d deps, title, body string) error {
	if d.runCommand == nil {
		return errors.New("kitty command runner is not configured")
	}

	arg := fmt.Sprintf("%q %q", title, body)
	if _, err := d.runCommand("kitten", "@", "action", "show_error", arg); err != nil {
		return fmt.Errorf("show kitty error: %w", err)
	}
	return nil
}

func filterInactiveKittyOSWindows(windows []kittyOSWindow) []kittyOSWindow {
	filtered := make([]kittyOSWindow, 0, len(windows))
	for _, window := range windows {
		if window.IsActive {
			continue
		}
		filtered = append(filtered, window)
	}
	return filtered
}

func activeKittyTabIDForOSWindow(windows []kittyOSWindow, osWindowID string) (string, error) {
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

func pickKittyOSWindow(windows []kittyOSWindow, d deps) (string, error) {
	cmd := exec.Command("fzf", "--ansi")
	cmd.Stdin = strings.NewReader(formatKittyOSWindows(windows))
	cmd.Stderr = d.stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return "", nil
		}
		return "", fmt.Errorf("run fzf: %w", err)
	}

	return parseKittyOSWindowID(stdout.String())
}

func parseKittyOSWindowID(line string) (string, error) {
	plain := kittyANSIPattern.ReplaceAllString(strings.TrimSpace(line), "")
	if plain == "" {
		return "", nil
	}

	id, _, found := strings.Cut(plain, ":")
	if !found {
		return "", fmt.Errorf("invalid kitty os window selection %q", plain)
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("invalid kitty os window selection %q", plain)
	}

	return id, nil
}
