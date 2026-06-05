package tmuxtargets

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/elentok/blf/internal/targets"
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

func Execute(popup bool, target string) error {
	var err error
	if popup {
		err = runPopupMode(target)
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
		"-T", "Select a target",
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

func runPopupMode(targetPane string) error {
	if _, err := lookPath("tmux"); err != nil {
		return errors.New("tmux binary not found in PATH")
	}

	lines, err := captureViewport(targetPane)
	if err != nil {
		return err
	}

	detected := targets.DetectTargets(lines)
	if len(detected) == 0 {
		return errNoTargets
	}

	lines, detected = targets.CondenseViewport(lines, detected, 1)

	notify := func(string) {}
	if err := targets.RunPopupUI(lines, detected, "Tmux Targets", notify, func(command string) error {
		return runResumeCommandInPane(targetPane, command)
	}); err != nil {
		return err
	}

	return nil
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
	return targets.NormalizeViewportText(string(out)), nil
}

func notifyFailure(err error) {
	tmuxutil.DisplayToolError(runCmd, "tmux-targets", err)
}

func notifyInfo(msg string) {
	tmuxutil.DisplayToolMessage(runCmd, "tmux-targets", msg)
}

func runResumeCommandInPane(paneID, command string) error {
	if err := runCmd("tmux", "send-keys", "-t", paneID, "-l", command); err != nil {
		return fmt.Errorf("send resume command to pane: %w", err)
	}
	if err := runCmd("tmux", "send-keys", "-t", paneID, "Enter"); err != nil {
		return fmt.Errorf("submit resume command in pane: %w", err)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
