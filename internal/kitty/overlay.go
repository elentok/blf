package kitty

import (
	"errors"
	"fmt"
)

func LaunchOverlay(subcommand string, d Deps) error {
	if d.RunCommand == nil {
		return errors.New("kitty command runner is not configured")
	}
	if d.ExecutablePath == nil {
		return errors.New("executable path resolver is not configured")
	}

	exe, err := d.ExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	if _, err := d.RunCommand(
		"kitty", "@", "launch",
		"--match", "state:focused",
		"--source-window", "state:focused",
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

func ShowError(d Deps, title, body string) error {
	if d.RunCommand == nil {
		return errors.New("kitty command runner is not configured")
	}

	arg := fmt.Sprintf("%q %q", title, body)
	if _, err := d.RunCommand("kitten", "@", "action", "show_error", arg); err != nil {
		return fmt.Errorf("show kitty error: %w", err)
	}
	return nil
}
