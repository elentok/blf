package kitty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const DoctorCmd = "doctor"

func Doctor(args []string, d Deps) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: blf kitty doctor")
	}
	return writeDoctorReport(d)
}

func writeDoctorReport(d Deps) error {
	if d.Stdout == nil {
		return errors.New("stdout is not configured")
	}

	writeLine := func(format string, args ...any) error {
		_, err := fmt.Fprintf(d.Stdout, format+"\n", args...)
		return err
	}

	if err := writeLine("blf kitty doctor"); err != nil {
		return err
	}
	if err := writeLine(""); err != nil {
		return err
	}

	if err := writeDoctorSectionHeader(d.Stdout, "environment"); err != nil {
		return err
	}
	for _, key := range []string{"KITTY_WINDOW_ID", "KITTY_LISTEN_ON", "TERM", "TERM_PROGRAM"} {
		if err := writeLine("%s=%s", key, doctorEnvValue(key, d)); err != nil {
			return err
		}
	}

	if err := writeDoctorSectionHeader(d.Stdout, "kitty binary"); err != nil {
		return err
	}
	if out, err := runDoctorCommand(d, "kitty", "--version"); err != nil {
		if err := writeLine("kitty --version: ERROR: %v", err); err != nil {
			return err
		}
	} else if err := writeLine("kitty --version: %s", oneLine(out)); err != nil {
		return err
	}

	if err := writeDoctorSectionHeader(d.Stdout, "kitty ls"); err != nil {
		return err
	}
	if windows, err := ListOSWindows(d); err != nil {
		if err := writeLine("kitty @ ls: ERROR: %v", err); err != nil {
			return err
		}
	} else {
		if err := writeLine("os_windows=%d tabs=%d", len(windows), countTabs(windows)); err != nil {
			return err
		}
		for _, expr := range []string{"session:.", "session:~", "session:^$"} {
			count, err := doctorTabCount(expr, d)
			if err != nil {
				if err := writeLine("%s -> ERROR: %v", expr, err); err != nil {
					return err
				}
				continue
			}
			if err := writeLine("%s -> tabs=%d", expr, count); err != nil {
				return err
			}
		}
	}

	sessionDir, err := SessionsDir(d)
	if err != nil {
		return err
	}

	if err := writeDoctorSectionHeader(d.Stdout, "session dir"); err != nil {
		return err
	}
	if err := writeLine("path=%s", sessionDir); err != nil {
		return err
	}

	entries, err := readDoctorSessionEntries(sessionDir, d)
	if err != nil {
		if err := writeLine("read: ERROR: %v", err); err != nil {
			return err
		}
		return nil
	}

	if err := writeLine("session_files=%d", len(entries)); err != nil {
		return err
	}
	for _, entry := range entries {
		path := sessionDir + string(os.PathSeparator) + entry
		if err := writeLine("- %s", entry); err != nil {
			return err
		}
		if err := writeLine("  stem=%s", sessionStem(path)); err != nil {
			return err
		}
		expr := sessionMatchExpr(path)
		count, err := doctorTabCount(expr, d)
		if err != nil {
			if err := writeLine("  %s -> ERROR: %v", expr, err); err != nil {
				return err
			}
			continue
		}
		if err := writeLine("  %s -> tabs=%d", expr, count); err != nil {
			return err
		}
	}

	activeSessions, err := ListActiveSessions(d)
	if err := writeDoctorSectionHeader(d.Stdout, "active sessions"); err != nil {
		return err
	}
	if err != nil {
		return writeLine("ERROR: %v", err)
	}
	if len(activeSessions) == 0 {
		return writeLine("none")
	}
	for _, session := range activeSessions {
		if err := writeLine("- %s (%d tabs) -> %s", session.Name, session.TabCount, session.Path); err != nil {
			return err
		}
	}

	return nil
}

func writeDoctorSectionHeader(w io.Writer, name string) error {
	_, err := fmt.Fprintf(w, "[%s]\n", name)
	return err
}

func doctorEnvValue(key string, d Deps) string {
	if d.LookupEnv == nil {
		return "<unavailable>"
	}
	if v, ok := d.LookupEnv(key); ok {
		return v
	}
	return "<unset>"
}

func runDoctorCommand(d Deps, name string, args ...string) (string, error) {
	if d.RunCommand == nil {
		return "", errors.New("kitty command runner is not configured")
	}
	out, err := d.RunCommand(name, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func oneLine(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

func doctorTabCount(expr string, d Deps) (int, error) {
	if d.RunCommand == nil {
		return 0, errors.New("kitty command runner is not configured")
	}
	out, err := d.RunCommand("kitty", "@", "ls", "--match-tab", expr)
	if err != nil {
		if isKittyNoMatchError(err) {
			return 0, nil
		}
		return 0, err
	}
	windows, err := ParseOSWindows(out)
	if err != nil {
		return 0, err
	}
	return countTabs(windows), nil
}

func readDoctorSessionEntries(sessionDir string, d Deps) ([]string, error) {
	if d.ReadDir == nil {
		return nil, errors.New("read dir helper is not configured")
	}
	entries, err := d.ReadDir(sessionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isSessionFilename(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sortStrings(names)
	return names, nil
}

func sortStrings(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
