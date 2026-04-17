package kitty

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var sessionExtensions = []string{".kitty-session", ".kitty_session", ".session"}

func promptSessionName(r io.Reader, w io.Writer) (string, error) {
	if _, err := io.WriteString(w, "Session name: "); err != nil {
		return "", err
	}

	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read kitty session name: %w", err)
	}

	name := strings.TrimSpace(line)
	if err := validateSessionName(name); err != nil {
		return "", err
	}

	return name, nil
}

func validateSessionName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("kitty session name is required")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("kitty session name %q cannot contain path separators", name)
	}
	return nil
}

func createSessionFile(name string, d Deps) (string, error) {
	if d.UserHomeDir == nil {
		return "", errors.New("home directory resolver is not configured")
	}
	if d.MkdirAll == nil {
		return "", errors.New("mkdir helper is not configured")
	}
	if d.WriteFile == nil {
		return "", errors.New("write file helper is not configured")
	}
	if d.FileExists == nil {
		return "", errors.New("file exists helper is not configured")
	}
	if d.Getwd == nil {
		return "", errors.New("working directory resolver is not configured")
	}

	sessionDir, err := SessionsDir(d)
	if err != nil {
		return "", err
	}
	if err := d.MkdirAll(sessionDir, 0o755); err != nil {
		return "", fmt.Errorf("create kitty sessions directory: %w", err)
	}

	path := filepath.Join(sessionDir, name+".kitty-session")
	exists, err := d.FileExists(path)
	if err != nil {
		return "", fmt.Errorf("check kitty session file: %w", err)
	}
	if exists {
		return "", fmt.Errorf("kitty session %q already exists", path)
	}

	cwd, err := d.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}

	if err := d.WriteFile(path, []byte(formatSessionFile(name, cwd)), 0o644); err != nil {
		return "", fmt.Errorf("write kitty session file: %w", err)
	}

	return path, nil
}

func formatSessionFile(name, cwd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "new_tab %s\n", quote(name))
	fmt.Fprintf(&b, "cd %s\n", quote(cwd))
	b.WriteString("launch\n")
	return b.String()
}

func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

func SessionsDir(d Deps) (string, error) {
	if d.UserHomeDir == nil {
		return "", errors.New("home directory resolver is not configured")
	}

	homeDir, err := d.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(homeDir, ".local", "share", "kitty", "sessions"), nil
}

func gotoSession(path string, d Deps) error {
	if d.RunCommand == nil {
		return errors.New("kitty command runner is not configured")
	}
	if _, err := d.RunCommand("kitten", "@", "action", "goto_session", path); err != nil {
		return fmt.Errorf("goto kitty session %q: %w", path, err)
	}
	return nil
}

func ListActiveSessions(d Deps) ([]Session, error) {
	if d.ReadDir == nil {
		return nil, errors.New("read dir helper is not configured")
	}

	sessionDir, err := SessionsDir(d)
	if err != nil {
		return nil, err
	}

	entries, err := d.ReadDir(sessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read kitty sessions directory: %w", err)
	}

	sessions := make([]Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isSessionFilename(name) {
			continue
		}

		path := filepath.Join(sessionDir, name)
		tabCount, err := sessionTabCount(path, d)
		if err != nil {
			return nil, err
		}
		if tabCount == 0 {
			continue
		}

		sessions = append(sessions, Session{
			Name:     trimSessionExtension(name),
			Path:     path,
			TabCount: tabCount,
		})
	}

	sortSessionsByName(sessions)

	return sessions, nil
}

func sessionTabCount(path string, d Deps) (int, error) {
	if d.RunCommand == nil {
		return 0, errors.New("kitty command runner is not configured")
	}

	output, err := d.RunCommand("kitty", "@", "ls", "--match-tab", matchSessionExpr(path))
	if err != nil {
		return 0, fmt.Errorf("list kitty tabs for session %q: %w", path, err)
	}

	windows, err := ParseOSWindows(output)
	if err != nil {
		return 0, fmt.Errorf("parse kitty tabs for session %q: %w", path, err)
	}

	return countTabs(windows), nil
}

func countTabs(windows []OSWindow) int {
	total := 0
	for _, window := range windows {
		total += len(window.Tabs)
	}
	return total
}

func isSessionFilename(name string) bool {
	for _, ext := range sessionExtensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func trimSessionExtension(name string) string {
	for _, ext := range sessionExtensions {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext)
		}
	}
	return name
}

func formatSessionChoices(sessions []Session) string {
	var b strings.Builder
	for _, session := range sessions {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", session.Path, session.Name, formatTabCount(session.TabCount))
	}
	return b.String()
}

func formatTabCount(n int) string {
	if n == 1 {
		return "1 tab"
	}
	return fmt.Sprintf("%d tabs", n)
}

func sortSessionsByName(sessions []Session) {
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].Name < sessions[i].Name {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}
}
