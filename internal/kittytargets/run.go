package kittytargets

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/elentok/blf/internal/targets"
)

var (
	errNoTargets = errors.New("no targets found in current kitty window")

	lookPath       = exec.LookPath
	executablePath = os.Executable
	runCmd         = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
	runPopupUI           = targets.RunPopupUI
	stderr     io.Writer = os.Stderr
)

func Execute(args []string) error {
	var err error
	if len(args) > 0 && args[0] == "--overlay" {
		err = runOverlayMode(args[1:])
	} else {
		err = runTopLevel()
	}

	if err == nil {
		return nil
	}
	notifyErr := notifyFailure(err)
	if errors.Is(err, errNoTargets) {
		if notifyErr != nil {
			return notifyErr
		}
		return nil
	}
	if notifyErr != nil {
		return fmt.Errorf("%v (also failed to notify in kitty: %w)", err, notifyErr)
	}
	return err
}

func runTopLevel() error {
	windowID := strings.TrimSpace(os.Getenv("KITTY_WINDOW_ID"))
	if windowID == "" {
		return errors.New("kitty targets must run inside kitty")
	}
	if _, err := lookPath("kitty"); err != nil {
		return errors.New("kitty binary not found in PATH")
	}
	if _, err := lookPath("kitten"); err != nil {
		return errors.New("kitten binary not found in PATH")
	}
	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	lines, err := captureViewport(windowID)
	if err != nil {
		return err
	}
	if len(targets.DetectTargets(lines)) == 0 {
		return errNoTargets
	}

	if err := runCmd(
		"kitty", "@", "launch",
		"--type=overlay",
		"--copy-env",
		"--",
		exe, "kitty", "targets", "--overlay", "--target", windowID,
	); err != nil {
		return fmt.Errorf("open kitty targets overlay: %w", err)
	}

	return nil
}

func runOverlayMode(args []string) error {
	targetWindow, err := parseOverlayArgs(args)
	if err != nil {
		return err
	}

	if _, err := lookPath("kitty"); err != nil {
		return errors.New("kitty binary not found in PATH")
	}
	if _, err := lookPath("kitten"); err != nil {
		return errors.New("kitten binary not found in PATH")
	}

	lines, err := captureViewport(targetWindow)
	if err != nil {
		return err
	}

	detected := targets.DetectTargets(lines)
	if len(detected) == 0 {
		return errNoTargets
	}

	notify := func(string) {}
	if err := runPopupUI(lines, detected, "Kitty Targets", notify, func(command string) error {
		return runResumeCommandInWindow(targetWindow, command)
	}); err != nil {
		return err
	}

	return nil
}

func parseOverlayArgs(args []string) (string, error) {
	if len(args) != 2 || args[0] != "--target" {
		return "", errors.New("usage: blf kitty targets --overlay --target <window-id>")
	}
	if strings.TrimSpace(args[1]) == "" {
		return "", errors.New("missing overlay target window id")
	}
	return args[1], nil
}

func captureViewport(windowID string) ([]string, error) {
	out, err := outputCmd("kitty", "@", "get-text", "--extent", "screen", "--match", "id:"+windowID)
	if err != nil {
		return nil, fmt.Errorf("capture kitty window viewport: %w", err)
	}
	return targets.NormalizeViewportText(string(out)), nil
}

func notifyFailure(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(err.Error())
	const prefix = "blf kitty targets: "
	if !strings.HasPrefix(msg, prefix) {
		msg = prefix + msg
	}
	if stderr != nil {
		_, _ = fmt.Fprintln(stderr, msg)
	}
	if notifyErr := showKittyError("blf kitty targets", err.Error()); notifyErr != nil {
		return fmt.Errorf("notify kitty targets failure: %w", notifyErr)
	}
	return nil
}

func showKittyError(title, body string) error {
	arg := fmt.Sprintf("%q %q", title, body)
	if err := runCmd("kitten", "@", "action", "show_error", arg); err != nil {
		return fmt.Errorf("show kitty error: %w", err)
	}
	return nil
}

func runResumeCommandInWindow(windowID, command string) error {
	if err := runCmd("kitty", "@", "send-text", "--match", "id:"+windowID, "--", command+"\r"); err != nil {
		return fmt.Errorf("send resume command to kitty window: %w", err)
	}
	return nil
}
