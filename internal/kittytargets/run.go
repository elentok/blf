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

	lookPath = exec.LookPath
	runCmd   = func(name string, args ...string) error {
		return exec.Command(name, args...).Run()
	}
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
	runPopupUI           = targets.RunPopupUI
	stderr     io.Writer = os.Stderr
)

func Execute(args []string) error {
	err := run(args)
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

func run(args []string) error {
	if _, err := lookPath("kitty"); err != nil {
		return errors.New("kitty binary not found in PATH")
	}
	if _, err := lookPath("kitten"); err != nil {
		return errors.New("kitten binary not found in PATH")
	}

	targetMatch, err := resolveTargetMatch(args)
	if err != nil {
		return err
	}

	lines, err := captureViewport(targetMatch)
	if err != nil {
		return err
	}

	detected := targets.DetectTargets(lines)
	if len(detected) == 0 {
		return errNoTargets
	}

	notify := func(string) {}
	if err := runPopupUI(lines, detected, "Kitty Targets", notify, func(command string) error {
		return runResumeCommandInWindow(targetMatch, command)
	}); err != nil {
		return err
	}

	return nil
}

func resolveTargetMatch(args []string) (string, error) {
	switch len(args) {
	case 0:
		return resolveImplicitTargetMatch()
	case 1:
		if args[0] == "--overlay" {
			return resolveImplicitTargetMatch()
		}
	case 2:
		if args[0] == "--target" {
			return parseExplicitTargetMatch(args[1])
		}
	case 3:
		if args[0] == "--overlay" && args[1] == "--target" {
			return parseExplicitTargetMatch(args[2])
		}
	}

	return "", errors.New("usage: blf kitty targets [--target <window-id>]")
}

func resolveImplicitTargetMatch() (string, error) {
	windowID := strings.TrimSpace(os.Getenv("KITTY_WINDOW_ID"))
	if windowID == "" {
		return "", errors.New("kitty targets must run inside kitty")
	}

	if _, err := captureViewport("state:overlay_parent"); err == nil {
		return "state:overlay_parent", nil
	}

	return "id:" + windowID, nil
}

func parseExplicitTargetMatch(windowID string) (string, error) {
	if strings.TrimSpace(windowID) == "" {
		return "", errors.New("missing overlay target window id")
	}
	return "id:" + windowID, nil
}

func captureViewport(targetMatch string) ([]string, error) {
	out, err := outputCmd("kitty", "@", "get-text", "--extent", "screen", "--match", targetMatch)
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

func runResumeCommandInWindow(targetMatch, command string) error {
	if err := runCmd("kitty", "@", "send-text", "--match", targetMatch, "--", command+"\r"); err != nil {
		return fmt.Errorf("send resume command to kitty window: %w", err)
	}
	return nil
}
