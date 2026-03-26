package tmuxtargets

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/elentok/blf/internal/tmuxutil"
)

var (
	errNoTargets = errors.New("no targets found in current viewport")

	lookPath = exec.LookPath
	runCmd   = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

func Execute(args []string) error {
	var err error
	if len(args) > 0 && args[0] == "--popup" {
		err = runPopupMode(args[1:])
	} else {
		err = runTopLevel()
	}

	if err == nil {
		return nil
	}
	notifyFailure(err)
	if errors.Is(err, errNoTargets) {
		return nil
	}
	return err
}

func runTopLevel() error {
	if os.Getenv("TMUX") == "" {
		return errors.New("tmux-targets must run inside tmux")
	}
	if _, err := lookPath("tmux"); err != nil {
		return errors.New("tmux binary not found in PATH")
	}

	paneID, err := currentPaneID()
	if err != nil {
		return err
	}

	cmdStr := fmt.Sprintf("blf tmux-targets --popup --target %s", shellQuote(paneID))
	if err := runCmd(
		"tmux", "display-popup",
		"-t", paneID,
		"-T", "Select a target | y: yank | enter/o: open | /: search | q: quit",
		"-x", "C",
		"-y", "C",
		"-w", "80%",
		"-h", "80%",
		"-E", cmdStr,
	); err != nil {
		return fmt.Errorf("open targets popup: %w", err)
	}

	return nil
}

func runPopupMode(args []string) error {
	targetPane, err := parsePopupArgs(args)
	if err != nil {
		return err
	}

	if _, err := lookPath("tmux"); err != nil {
		return errors.New("tmux binary not found in PATH")
	}

	lines, err := captureViewport(targetPane)
	if err != nil {
		return err
	}

	targets := detectTargets(lines)
	if len(targets) == 0 {
		return errNoTargets
	}

	lines, targets = condenseViewport(lines, targets, 1)

	notify := func(msg string) {
		notifyInfo(msg)
	}
	if err := runPopupUI(lines, targets, notify); err != nil {
		return err
	}

	return nil
}

func parsePopupArgs(args []string) (string, error) {
	if len(args) != 2 || args[0] != "--target" {
		return "", errors.New("usage: blf tmux-targets --popup --target <pane-id>")
	}
	if strings.TrimSpace(args[1]) == "" {
		return "", errors.New("missing popup target pane id")
	}
	return args[1], nil
}

func currentPaneID() (string, error) {
	if paneID := strings.TrimSpace(os.Getenv("TMUX_PANE")); paneID != "" {
		return paneID, nil
	}

	out, err := outputCmd("tmux", "display-message", "-p", "#{pane_id}")
	if err != nil {
		return "", fmt.Errorf("read pane id: %w", err)
	}

	paneID := strings.TrimSpace(string(out))
	if paneID == "" {
		return "", errors.New("could not determine current pane id")
	}
	return paneID, nil
}

func captureViewport(paneID string) ([]string, error) {
	out, err := outputCmd("tmux", "capture-pane", "-p", "-t", paneID)
	if err != nil {
		return nil, fmt.Errorf("capture pane viewport: %w", err)
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}

func notifyFailure(err error) {
	tmuxutil.DisplayToolError(runCmd, "tmux-targets", err)
}

func notifyInfo(msg string) {
	tmuxutil.DisplayToolMessage(runCmd, "tmux-targets", msg)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
