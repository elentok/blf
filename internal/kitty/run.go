package kitty

import (
	"fmt"
	"io"
	"strconv"
)

const (
	ListOSWindowsCmd      = "list-os-windows"
	GotoOSWindowCmd       = "goto-os-window"
	NewSessionCmd         = "new-session"
	SessionsCmd           = "sessions"
	DeleteSessionCmd      = "delete-session"
	PreviewSessionCmd     = "__preview-session"
	ListSessionChoicesCmd = "__list-session-choices"
	DeleteSessionFileCmd  = "__delete-session-file"
	EditSessionFileCmd    = "__edit-session-file"
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

func DeleteSession(args []string, d Deps) error {
	switch {
	case len(args) == 0:
		return LaunchOverlay(DeleteSessionCmd, d)
	case len(args) == 1 && args[0] == "--overlay":
		return runDeleteSessionOverlay(d)
	default:
		return fmt.Errorf("usage: blf kitty delete-session")
	}
}

func PreviewSession(args []string, d Deps) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: blf kitty %s <path>", PreviewSessionCmd)
	}

	preview, err := RenderSessionPreview(args[0], d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, preview)
	return err
}

func ListSessionChoices(args []string, d Deps) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: blf kitty %s", ListSessionChoicesCmd)
	}

	sessions, err := ListSessions(d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, formatSessionChoices(sessions))
	return err
}

func DeleteSessionFile(args []string, d Deps) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: blf kitty %s <path>", DeleteSessionFileCmd)
	}

	return deleteSessionFile(args[0], d)
}

func EditSessionFile(args []string, d Deps) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: blf kitty %s <path>", EditSessionFileCmd)
	}

	return editSessionFile(args[0], d)
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
	sessions, err := ListSessions(d)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return ShowError(d, "blf kitty sessions", "No kitty sessions")
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

func runDeleteSessionOverlay(d Deps) error {
	sessions, err := ListSessions(d)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return ShowError(d, "blf kitty delete-session", "No kitty sessions")
	}

	path, err := pickSession(sessions, d)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}

	return deleteSessionFile(path, d)
}
