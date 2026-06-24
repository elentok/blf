package kitty

import (
	"fmt"
	"io"
	"strconv"
)

const (
	LSCmd                 = "ls"
	ListOSWindowsCmd      = "list-os-windows"
	GotoOSWindowCmd       = "goto-os-window"
	TargetsCmd            = "targets"
	ListAgentsCmd         = "list-agents"
	NewSessionCmd         = "new-session"
	SessionsCmd           = "sessions"
	DeleteSessionCmd      = "delete-session"
	PreviewSessionCmd     = "__preview-session"
	ListSessionChoicesCmd = "__list-session-choices"
	DeleteSessionFileCmd  = "__delete-session-file"
	EditSessionFileCmd    = "__edit-session-file"
)

func LSCommand(d Deps) error {
	windows, err := ListOSWindows(d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, FormatKittyLS(windows))
	return err
}

func ListOSWindowsCommand(d Deps) error {
	windows, err := ListOSWindows(d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, FormatOSWindows(windows))
	return err
}

func GotoOSWindow(id string, d Deps) error {
	windows, err := ListOSWindows(d)
	if err != nil {
		return err
	}

	if id == "" {
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

func NewSession(d Deps) error {
	return runNewSessionPrompt(d)
}

func SessionsCommand(d Deps) error {
	return runSessionsPicker(d)
}

func DeleteSession(overlay bool, d Deps) error {
	if overlay {
		return runDeleteSessionOverlay(d)
	}
	return LaunchOverlay(DeleteSessionCmd, d)
}

func PreviewSession(path string, d Deps) error {
	preview, err := RenderSessionPreview(path, d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, preview)
	return err
}

func ListSessionChoices(d Deps) error {
	sessions, err := ListSessionsForPicker(d)
	if err != nil {
		return err
	}

	_, err = io.WriteString(d.Stdout, formatSessionChoices(sessions))
	return err
}

func DeleteSessionFile(path string, d Deps) error {
	return deleteSessionFile(path, d)
}

func EditSessionFile(path string, d Deps) error {
	return editSessionFile(path, d)
}

func runNewSessionPrompt(d Deps) error {
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

func runSessionsPicker(d Deps) error {
	sessions, err := ListSessionsForPicker(d)
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
	sessions, err := ListSessionsForPicker(d)
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
