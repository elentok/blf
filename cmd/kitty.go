package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	kittyListOSWindows   = "list-os-windows"
	kittyGotoOSWindowCmd = "goto-os-window"
	kittyNewSessionCmd   = "new-session"
	kittySessionsCmd     = "sessions"
)

var (
	kittyActiveWindowStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	kittyLastFocusedWindowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	kittyANSIPattern            = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	kittySessionExtensions      = []string{".kitty-session", ".kitty_session", ".session"}
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

type kittySession struct {
	Name     string
	Path     string
	TabCount int
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
		return fmt.Errorf("usage: blf kitty <list-os-windows|goto-os-window|targets|new-session|sessions> [id]")
	}

	switch args[0] {
	case kittyListOSWindows:
		return runKittyListOSWindows(d)
	case kittyGotoOSWindowCmd:
		return runKittyGotoOSWindow(args[1:], d)
	case kittyNewSessionCmd:
		return runKittyNewSession(args[1:], d)
	case kittySessionsCmd:
		return runKittySessions(args[1:], d)
	case "targets":
		if d.runKittyTargets == nil {
			return fmt.Errorf("kitty targets runner is not configured")
		}
		return d.runKittyTargets(args[1:])
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

func runKittyNewSession(args []string, d deps) error {
	switch {
	case len(args) == 0:
		return launchKittyOverlay(kittyNewSessionCmd, d)
	case len(args) == 1 && args[0] == "--overlay":
		return runKittyNewSessionOverlay(d)
	default:
		return fmt.Errorf("usage: blf kitty new-session")
	}
}

func runKittySessions(args []string, d deps) error {
	switch {
	case len(args) == 0:
		return launchKittyOverlay(kittySessionsCmd, d)
	case len(args) == 1 && args[0] == "--overlay":
		return runKittySessionsOverlay(d)
	default:
		return fmt.Errorf("usage: blf kitty sessions")
	}
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

func launchKittyOverlay(subcommand string, d deps) error {
	if d.runCommand == nil {
		return errors.New("kitty command runner is not configured")
	}
	if d.executablePath == nil {
		return errors.New("executable path resolver is not configured")
	}

	exe, err := d.executablePath()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	if _, err := d.runCommand(
		"kitty", "@", "launch",
		"--type=overlay",
		"--copy-env",
		"--cwd=current",
		"--",
		exe, "kitty", subcommand, "--overlay",
	); err != nil {
		return fmt.Errorf("open kitty %s overlay: %w", subcommand, err)
	}

	return nil
}

func runKittyNewSessionOverlay(d deps) error {
	name, err := promptKittySessionName(d.stdin, d.stdout)
	if err != nil {
		return err
	}

	path, err := createKittySessionFile(name, d)
	if err != nil {
		return err
	}

	return gotoKittySession(path, d)
}

func runKittySessionsOverlay(d deps) error {
	sessions, err := listKittyActiveSessions(d)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return showKittyError(d, "blf kitty sessions", "No active kitty sessions")
	}

	path, err := pickKittySession(sessions, d)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}

	return gotoKittySession(path, d)
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

func promptKittySessionName(r io.Reader, w io.Writer) (string, error) {
	if _, err := io.WriteString(w, "Session name: "); err != nil {
		return "", err
	}

	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read kitty session name: %w", err)
	}

	name := strings.TrimSpace(line)
	if err := validateKittySessionName(name); err != nil {
		return "", err
	}

	return name, nil
}

func validateKittySessionName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("kitty session name is required")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("kitty session name %q cannot contain path separators", name)
	}
	return nil
}

func createKittySessionFile(name string, d deps) (string, error) {
	if d.userHomeDir == nil {
		return "", errors.New("home directory resolver is not configured")
	}
	if d.mkdirAll == nil {
		return "", errors.New("mkdir helper is not configured")
	}
	if d.writeFile == nil {
		return "", errors.New("write file helper is not configured")
	}
	if d.fileExists == nil {
		return "", errors.New("file exists helper is not configured")
	}
	if d.getwd == nil {
		return "", errors.New("working directory resolver is not configured")
	}

	sessionDir, err := kittySessionsDir(d)
	if err != nil {
		return "", err
	}
	if err := d.mkdirAll(sessionDir, 0o755); err != nil {
		return "", fmt.Errorf("create kitty sessions directory: %w", err)
	}

	path := filepath.Join(sessionDir, name+".kitty-session")
	exists, err := d.fileExists(path)
	if err != nil {
		return "", fmt.Errorf("check kitty session file: %w", err)
	}
	if exists {
		return "", fmt.Errorf("kitty session %q already exists", path)
	}

	cwd, err := d.getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}

	if err := d.writeFile(path, []byte(formatKittySessionFile(name, cwd)), 0o644); err != nil {
		return "", fmt.Errorf("write kitty session file: %w", err)
	}

	return path, nil
}

func formatKittySessionFile(name, cwd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "new_tab %s\n", strconv.Quote(name))
	fmt.Fprintf(&b, "cd %s\n", strconv.Quote(cwd))
	b.WriteString("launch\n")
	return b.String()
}

func kittySessionsDir(d deps) (string, error) {
	if d.userHomeDir == nil {
		return "", errors.New("home directory resolver is not configured")
	}

	homeDir, err := d.userHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(homeDir, ".local", "share", "kitty", "sessions"), nil
}

func gotoKittySession(path string, d deps) error {
	if d.runCommand == nil {
		return errors.New("kitty command runner is not configured")
	}
	if _, err := d.runCommand("kitten", "@", "action", "goto_session", path); err != nil {
		return fmt.Errorf("goto kitty session %q: %w", path, err)
	}
	return nil
}

func listKittyActiveSessions(d deps) ([]kittySession, error) {
	if d.readDir == nil {
		return nil, errors.New("read dir helper is not configured")
	}

	sessionDir, err := kittySessionsDir(d)
	if err != nil {
		return nil, err
	}

	entries, err := d.readDir(sessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read kitty sessions directory: %w", err)
	}

	sessions := make([]kittySession, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isKittySessionFilename(name) {
			continue
		}

		path := filepath.Join(sessionDir, name)
		tabCount, err := kittySessionTabCount(path, d)
		if err != nil {
			return nil, err
		}
		if tabCount == 0 {
			continue
		}

		sessions = append(sessions, kittySession{
			Name:     trimKittySessionExtension(name),
			Path:     path,
			TabCount: tabCount,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})

	return sessions, nil
}

func kittySessionTabCount(path string, d deps) (int, error) {
	if d.runCommand == nil {
		return 0, errors.New("kitty command runner is not configured")
	}

	match := "session:^" + regexp.QuoteMeta(path) + "$"
	output, err := d.runCommand("kitty", "@", "ls", "--match-tab", match)
	if err != nil {
		return 0, fmt.Errorf("list kitty tabs for session %q: %w", path, err)
	}

	windows, err := parseKittyOSWindows(output)
	if err != nil {
		return 0, fmt.Errorf("parse kitty tabs for session %q: %w", path, err)
	}

	return countKittyTabs(windows), nil
}

func countKittyTabs(windows []kittyOSWindow) int {
	total := 0
	for _, window := range windows {
		total += len(window.Tabs)
	}
	return total
}

func isKittySessionFilename(name string) bool {
	for _, ext := range kittySessionExtensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func trimKittySessionExtension(name string) string {
	for _, ext := range kittySessionExtensions {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext)
		}
	}
	return name
}

func formatKittySessionChoices(sessions []kittySession) string {
	var b strings.Builder
	for _, session := range sessions {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", session.Path, session.Name, formatKittyTabCount(session.TabCount))
	}
	return b.String()
}

func formatKittyTabCount(n int) string {
	if n == 1 {
		return "1 tab"
	}
	return fmt.Sprintf("%d tabs", n)
}

func pickKittySession(sessions []kittySession, d deps) (string, error) {
	cmd := exec.Command("fzf", "--delimiter", "\t", "--with-nth", "2,3")
	cmd.Stdin = strings.NewReader(formatKittySessionChoices(sessions))
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

	return parseKittySessionSelection(stdout.String())
}

func parseKittySessionSelection(line string) (string, error) {
	plain := strings.TrimSpace(line)
	if plain == "" {
		return "", nil
	}

	path, _, found := strings.Cut(plain, "\t")
	if !found {
		return "", fmt.Errorf("invalid kitty session selection %q", plain)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("invalid kitty session selection %q", plain)
	}
	return path, nil
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
