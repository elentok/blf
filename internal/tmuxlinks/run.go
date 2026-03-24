package tmuxlinks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	errNoLinks = errors.New("no http/https URLs found in the last 10000 pane lines")

	lookPath = exec.LookPath
	runCmd   = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

func RunMenu(mode string) error {
	err := runMenu(mode)
	if err == nil {
		return nil
	}

	notifyFailure(err)
	if errors.Is(err, errNoLinks) {
		return nil
	}
	return err
}

func runMenu(mode string) error {
	if mode != ModeOpen && mode != ModeCopy {
		return fmt.Errorf("invalid mode %q", mode)
	}

	if os.Getenv("TMUX") == "" {
		return errors.New("tmux-links must run inside tmux")
	}

	if _, err := lookPath("tmux"); err != nil {
		return errors.New("tmux binary not found in PATH")
	}

	text, err := capturePaneText()
	if err != nil {
		return err
	}

	urls := ExtractURLs(text)
	if len(urls) == 0 {
		return errNoLinks
	}

	args, err := BuildDisplayMenuArgs(mode, urls)
	if err != nil {
		return err
	}

	if err := runCmd("tmux", args...); err != nil {
		return fmt.Errorf("open tmux menu: %w", err)
	}

	return nil
}

func notifyFailure(err error) {
	if err == nil {
		return
	}
	if os.Getenv("TMUX") == "" {
		return
	}

	msg := "blf tmux-links: " + strings.ReplaceAll(err.Error(), "\n", " ")
	_ = runCmd("tmux", "display-message", "-d", "5000", msg)
}

func capturePaneText() (string, error) {
	// -p prints pane contents, -J joins soft wraps (keeps wrapped URLs whole),
	// and -S -10000 captures enough scrollback to be useful.
	out, err := outputCmd("tmux", "capture-pane", "-pJ", "-S", "-10000")
	if err != nil {
		return "", fmt.Errorf("capture tmux pane: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
