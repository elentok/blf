package tmuxtargets

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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

	paneID, width, height, left, top, err := readPaneInfo()
	if err != nil {
		return err
	}

	cmdStr := fmt.Sprintf("blf tmux-targets --popup --target %s", shellQuote(paneID))
	if err := runCmd(
		"tmux", "display-popup", "-B",
		"-x", strconv.Itoa(left),
		"-y", strconv.Itoa(top),
		"-w", strconv.Itoa(width),
		"-h", strconv.Itoa(height),
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

func readPaneInfo() (string, int, int, int, int, error) {
	out, err := outputCmd("tmux", "display-message", "-p", "#{pane_id} #{pane_width} #{pane_height} #{pane_left} #{pane_top}")
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("read pane info: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 5 {
		return "", 0, 0, 0, 0, errors.New("unexpected tmux pane info format")
	}
	w, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("parse pane width: %w", err)
	}
	h, err := strconv.Atoi(fields[2])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("parse pane height: %w", err)
	}
	x, err := strconv.Atoi(fields[3])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("parse pane left: %w", err)
	}
	y, err := strconv.Atoi(fields[4])
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("parse pane top: %w", err)
	}
	return fields[0], w, h, x, y, nil
}

func captureViewport(paneID string) ([]string, error) {
	out, err := outputCmd("tmux", "capture-pane", "-p", "-t", paneID)
	if err != nil {
		return nil, fmt.Errorf("capture pane viewport: %w", err)
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
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
