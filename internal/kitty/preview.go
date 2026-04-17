package kitty

import (
	"bufio"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

type sessionPreview struct {
	Name string
	Path string
	Tabs []sessionPreviewTab
}

type sessionPreviewTab struct {
	Name    string
	Cwd     string
	Layout  string
	Windows []string
}

var (
	liveTabsStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	savedTabsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	tabLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

func RenderSessionPreview(path string, d Deps) (string, error) {
	savedPreview, savedErr := readSavedSessionPreview(path, d)
	if savedErr != nil && !errors.Is(savedErr, errNoSavedSessionPreview) {
		return "", savedErr
	}

	livePreview, active, err := renderLiveSessionPreview(path, d)
	if err != nil {
		return "", err
	}
	if active {
		return formatLiveSessionPreview(path, livePreview, savedPreview), nil
	}

	if errors.Is(savedErr, errNoSavedSessionPreview) {
		return "", errors.New("read file helper is not configured")
	}
	return formatSavedSessionPreview(savedPreview), nil
}

func renderLiveSessionPreview(path string, d Deps) ([]OSWindow, bool, error) {
	if d.RunCommand == nil {
		return nil, false, nil
	}

	output, err := d.RunCommand("kitty", "@", "ls", "--match-tab", sessionMatchExpr(path))
	if err != nil {
		if isKittyNoMatchError(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("list kitty tabs for session %q: %w", path, err)
	}

	windows, err := ParseOSWindows(output)
	if err != nil {
		return nil, false, fmt.Errorf("parse kitty tabs for session %q: %w", path, err)
	}
	if countTabs(windows) == 0 {
		return nil, false, nil
	}

	return windows, true, nil
}

var errNoSavedSessionPreview = errors.New("saved session preview unavailable")

func readSavedSessionPreview(path string, d Deps) (sessionPreview, error) {
	if d.ReadFile == nil {
		return sessionPreview{}, errNoSavedSessionPreview
	}

	content, err := d.ReadFile(path)
	if err != nil {
		return sessionPreview{}, fmt.Errorf("read kitty session file %q: %w", path, err)
	}

	return parseSessionPreview(path, string(content)), nil
}

func parseSessionPreview(path, content string) sessionPreview {
	preview := sessionPreview{
		Name: trimSessionExtension(filepath.Base(path)),
		Path: path,
	}

	var current *sessionPreviewTab
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		cmd, rest := splitSessionCommand(line)
		switch cmd {
		case "new_tab":
			tab := sessionPreviewTab{Name: previewValue(rest)}
			preview.Tabs = append(preview.Tabs, tab)
			current = &preview.Tabs[len(preview.Tabs)-1]
		case "cd":
			current = ensurePreviewTab(&preview, current)
			current.Cwd = previewValue(rest)
		case "layout":
			current = ensurePreviewTab(&preview, current)
			current.Layout = previewValue(rest)
		case "launch":
			current = ensurePreviewTab(&preview, current)
			current.Windows = append(current.Windows, previewLaunchLabel(rest))
		}
	}

	if len(preview.Tabs) == 0 {
		preview.Tabs = append(preview.Tabs, sessionPreviewTab{Name: preview.Name})
	}

	for i := range preview.Tabs {
		if strings.TrimSpace(preview.Tabs[i].Name) == "" {
			preview.Tabs[i].Name = fmt.Sprintf("tab %d", i+1)
		}
	}

	return preview
}

func ensurePreviewTab(preview *sessionPreview, current *sessionPreviewTab) *sessionPreviewTab {
	if current != nil {
		return current
	}
	preview.Tabs = append(preview.Tabs, sessionPreviewTab{})
	return &preview.Tabs[len(preview.Tabs)-1]
}

func splitSessionCommand(line string) (string, string) {
	cmd, rest, found := strings.Cut(line, " ")
	if !found {
		return line, ""
	}
	return cmd, strings.TrimSpace(rest)
}

func previewValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(raw); err == nil {
		return unquoted
	}
	return raw
}

func previewLaunchLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "shell"
	}
	return raw
}

func formatSavedSessionPreview(preview sessionPreview) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", preview.Name)
	b.WriteString("\n")
	fmt.Fprintf(&b, "Path: %s\n", preview.Path)
	b.WriteString("State: inactive\n")
	b.WriteString("\n")
	b.WriteString(savedTabsStyle.Render("No live tabs, saved session:"))
	b.WriteString("\n")

	if len(preview.Tabs) == 0 {
		b.WriteString("\n(empty session file)\n")
		return b.String()
	}

	for i, tab := range preview.Tabs {
		b.WriteString("\n")
		writePreviewTab(&b, i, tab.Name, tab)
	}

	return b.String()
}

func formatLiveSessionPreview(path string, windows []OSWindow, saved sessionPreview) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", sessionStem(path))
	b.WriteString("\n")
	fmt.Fprintf(&b, "Path: %s\n", path)
	b.WriteString("State: active\n")
	b.WriteString("\n")
	b.WriteString(liveTabsStyle.Render("Live tabs:"))
	b.WriteString("\n")

	liveTabs := flattenTabs(windows)
	for i, tab := range liveTabs {
		b.WriteString("\n")
		savedTab := sessionPreviewTab{}
		if i < len(saved.Tabs) {
			savedTab = saved.Tabs[i]
		}
		writePreviewTab(&b, i, tab.Title, savedTab)
	}

	return b.String()
}

func flattenTabs(windows []OSWindow) []Tab {
	var tabs []Tab
	for _, window := range windows {
		tabs = append(tabs, window.Tabs...)
	}
	return tabs
}

func writePreviewTab(b *strings.Builder, index int, title string, details sessionPreviewTab) {
	fmt.Fprintln(b, tabLineStyle.Render(fmt.Sprintf("Tab %d: %s", index+1, title)))
	if details.Cwd != "" {
		fmt.Fprintf(b, "|- cd: %s\n", details.Cwd)
	}
	if details.Layout != "" {
		fmt.Fprintf(b, "|- layout: %s\n", details.Layout)
	}
	if len(details.Windows) == 0 {
		b.WriteString("`- windows: none\n")
		return
	}
	for windowIndex, window := range details.Windows {
		prefix := "|-"
		if windowIndex == len(details.Windows)-1 {
			prefix = "`-"
		}
		fmt.Fprintf(b, "%s window %d: %s\n", prefix, windowIndex+1, window)
	}
}
