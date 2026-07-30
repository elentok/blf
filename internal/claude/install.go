package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// StatusLineCommand is the command installed into Claude Code's statusLine
// settings.
const StatusLineCommand = "blf claude statusline"

// InstallDeps are the filesystem dependencies needed to install the
// statusLine entry into Claude Code's global settings.
type InstallDeps struct {
	Stdout      io.Writer
	UserHomeDir func() (string, error)
	ReadFile    func(string) ([]byte, error)
	WriteFile   func(string, []byte, os.FileMode) error
	MkdirAll    func(string, os.FileMode) error
}

// InstallStatusLine idempotently sets ~/.claude/settings.json's "statusLine"
// entry to run StatusLineCommand.
func InstallStatusLine(d InstallDeps) error {
	home, err := d.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	path := filepath.Join(home, ".claude", "settings.json")

	settings, err := readSettings(path, d)
	if err != nil {
		return err
	}

	settings["statusLine"] = map[string]any{
		"type":    "command",
		"command": StatusLineCommand,
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	dir := filepath.Dir(path)
	if err := d.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := d.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	fmt.Fprintf(d.Stdout, "Installed statusLine into %s\n", path)
	return nil
}

// readSettings parses the settings file, treating a missing or empty file as
// an empty object.
func readSettings(path string, d InstallDeps) (map[string]any, error) {
	data, err := d.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, nil
}
