// Package editfile opens a file in the user's $EDITOR, blocking until it exits.
package editfile

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Deps are the injectable dependencies for Open.
type Deps struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	LookupEnv func(string) (string, bool)
}

// Open opens path in $EDITOR, blocking until the editor exits.
func Open(path string, d Deps) error {
	if d.LookupEnv == nil {
		return errors.New("environment lookup helper is not configured")
	}

	editor, ok := d.LookupEnv("EDITOR")
	if !ok || strings.TrimSpace(editor) == "" {
		return errors.New("EDITOR environment variable is not set")
	}

	cmd := exec.Command("sh", "-lc", editor+` "$1"`, "sh", path)
	cmd.Stdin = d.Stdin
	cmd.Stdout = d.Stdout
	cmd.Stderr = d.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("edit %q: %w", path, err)
	}

	return nil
}
