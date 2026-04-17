package kitty

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
		tabCount, err := sessionTabCount(path, d)
		if err != nil {
			return "", err
		}
		if tabCount > 0 {
			return path, nil
		}
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

func ListSessions(d Deps) ([]Session, error) {
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
		sessions = append(sessions, Session{
			Name:     trimSessionExtension(name),
			Path:     path,
			TabCount: tabCount,
		})
	}

	sortSessionsByName(sessions)

	return sessions, nil
}

func ListActiveSessions(d Deps) ([]Session, error) {
	sessions, err := ListSessions(d)
	if err != nil {
		return nil, err
	}

	active := sessions[:0]
	for _, session := range sessions {
		if session.TabCount == 0 {
			continue
		}
		active = append(active, session)
	}
	return active, nil
}

func sessionTabCount(path string, d Deps) (int, error) {
	if d.RunCommand == nil {
		return 0, errors.New("kitty command runner is not configured")
	}

	output, err := d.RunCommand("kitty", "@", "ls", "--match-tab", sessionMatchExpr(path))
	if err != nil {
		if isKittyNoMatchError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("list kitty tabs for session %q: %w", path, err)
	}

	windows, err := ParseOSWindows(output)
	if err != nil {
		return 0, fmt.Errorf("parse kitty tabs for session %q: %w", path, err)
	}

	return countTabs(windows), nil
}

func isKittyNoMatchError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "exit status 1")
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
		fmt.Fprintf(&b, "%s\t%s\n", session.Path, formatSessionLabel(session))
	}
	return b.String()
}

func formatSessionLabel(session Session) string {
	return fmt.Sprintf("%s (%s)", session.Name, formatTabCount(session.TabCount))
}

func formatTabCount(n int) string {
	if n == 1 {
		return "1 tab"
	}
	return fmt.Sprintf("%d tabs", n)
}

func sortSessionsByName(sessions []Session) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Name < sessions[j].Name
	})
}

func sessionStem(path string) string {
	return trimSessionExtension(filepath.Base(path))
}

func sessionMatchExpr(path string) string {
	return matchSessionExpr(sessionStem(path))
}
