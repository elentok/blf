package kitty

import (
	"fmt"
	"io"
	"strconv"
)

const (
	ListOSWindowsCmd = "list-os-windows"
	GotoOSWindowCmd  = "goto-os-window"
	NewSessionCmd    = "new-session"
	SessionsCmd      = "sessions"
)

func ListOSWindowsCommand(d Deps) error {
	windows, err := ListOSWindows(d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, FormatOSWindows(windows))
	return err
}

func GotoOSWindow(args []string, d Deps) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: blf kitty goto-os-window [id]")
	}

	windows, err := ListOSWindows(d)
	if err != nil {
		return err
	}

	id := ""
	if len(args) == 1 {
		id = args[0]
	} else {
		otherWindows := filterInactiveOSWindows(windows)
		if len(otherWindows) == 0 {
			return ShowError(d, "blf kitty", "No other kitty windows")
		}

		selectedID, err := pickOSWindow(otherWindows, d)
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

	tabID, err := activeTabIDForOSWindow(windows, id)
	if err != nil {
		return err
	}

	if _, err := d.RunCommand("kitten", "@", "focus-tab", "--match", "id:"+tabID); err != nil {
		return fmt.Errorf("focus kitty os window %s: %w", id, err)
	}

	return nil
}

func NewSession(args []string, d Deps) error {
	switch {
	case len(args) == 0:
		return LaunchOverlay(NewSessionCmd, d)
	case len(args) == 1 && args[0] == "--overlay":
		return runNewSessionOverlay(d)
	default:
		return fmt.Errorf("usage: blf kitty new-session")
	}
}

func SessionsCommand(args []string, d Deps) error {
	switch {
	case len(args) == 0:
		return LaunchOverlay(SessionsCmd, d)
	case len(args) == 1 && args[0] == "--overlay":
		return runSessionsOverlay(d)
	default:
		return fmt.Errorf("usage: blf kitty sessions")
	}
}

func runNewSessionOverlay(d Deps) error {
	name, err := promptSessionName(d.Stdin, d.Stdout)
	if err != nil {
		return err
	}

	path, err := createSessionFile(name, d)
	if err != nil {
		return err
	}

	return gotoSession(path, d)
}

func runSessionsOverlay(d Deps) error {
	sessions, err := ListActiveSessions(d)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return ShowError(d, "blf kitty sessions", "No active kitty sessions")
	}

	path, err := pickSession(sessions, d)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}

	return gotoSession(path, d)
}
