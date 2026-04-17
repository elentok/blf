package kitty

import (
	"bufio"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
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

func RenderSessionPreview(path string, d Deps) (string, error) {
	if d.ReadFile == nil {
		return "", errors.New("read file helper is not configured")
	}

	content, err := d.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read kitty session file %q: %w", path, err)
	}

	preview := parseSessionPreview(path, string(content))
	return formatSessionPreview(preview), nil
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

func formatSessionPreview(preview sessionPreview) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\n", preview.Name)
	fmt.Fprintf(&b, "Path: %s\n", preview.Path)

	if len(preview.Tabs) == 0 {
		b.WriteString("\n(empty session file)\n")
		return b.String()
	}

	for i, tab := range preview.Tabs {
		b.WriteString("\n")
		fmt.Fprintf(&b, "Tab %d: %s\n", i+1, tab.Name)
		if tab.Cwd != "" {
			fmt.Fprintf(&b, "|- cd: %s\n", tab.Cwd)
		}
		if tab.Layout != "" {
			fmt.Fprintf(&b, "|- layout: %s\n", tab.Layout)
		}
		if len(tab.Windows) == 0 {
			b.WriteString("`- windows: none\n")
			continue
		}
		for windowIndex, window := range tab.Windows {
			prefix := "|-"
			if windowIndex == len(tab.Windows)-1 {
				prefix = "`-"
			}
			fmt.Fprintf(&b, "%s window %d: %s\n", prefix, windowIndex+1, window)
		}
	}

	return b.String()
}
