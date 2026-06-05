package kitty

import (
	"errors"
	"fmt"
	"strings"

	"github.com/elentok/blf/internal/targets"
)

var (
	errNoKittyTargets = errors.New("no targets found in current kitty window")

	runTargetsPopupUI = targets.RunPopupUI
)

func Targets(overlay bool, target string, d Deps) error {
	err := runTargets(target, d)
	if err == nil {
		return nil
	}

	notifyErr := notifyTargetsFailure(err, d)
	if errors.Is(err, errNoKittyTargets) {
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

func runTargets(target string, d Deps) error {
	if err := requireKittyBinaries(d); err != nil {
		return err
	}

	targetMatch, err := resolveTargetMatch(target, d)
	if err != nil {
		return err
	}

	lines, err := captureTargetViewport(targetMatch, d)
	if err != nil {
		return err
	}

	detected := targets.DetectTargets(lines)
	if len(detected) == 0 {
		return errNoKittyTargets
	}

	notify := func(string) {}
	if err := runTargetsPopupUI(lines, detected, "Kitty Targets", notify, func(command string) error {
		return runResumeCommandInWindow(targetMatch, command, d)
	}); err != nil {
		return err
	}

	return nil
}

func requireKittyBinaries(d Deps) error {
	if d.LookPath == nil {
		return nil
	}
	if _, err := d.LookPath("kitty"); err != nil {
		return errors.New("kitty binary not found in PATH")
	}
	if _, err := d.LookPath("kitten"); err != nil {
		return errors.New("kitten binary not found in PATH")
	}
	return nil
}

func resolveTargetMatch(target string, d Deps) (string, error) {
	if target != "" {
		return parseExplicitTargetMatch(target)
	}
	return resolveImplicitTargetMatch(d)
}

func resolveImplicitTargetMatch(d Deps) (string, error) {
	if d.LookupEnv == nil {
		return "", errors.New("kitty targets must run inside kitty")
	}

	windowID, ok := d.LookupEnv("KITTY_WINDOW_ID")
	windowID = strings.TrimSpace(windowID)
	if !ok || windowID == "" {
		return "", errors.New("kitty targets must run inside kitty")
	}

	if _, err := captureTargetViewport("state:overlay_parent", d); err == nil {
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

func captureTargetViewport(targetMatch string, d Deps) ([]string, error) {
	if d.RunCommand == nil {
		return nil, errors.New("kitty command runner is not configured")
	}

	out, err := d.RunCommand("kitty", "@", "get-text", "--extent", "screen", "--match", targetMatch)
	if err != nil {
		return nil, fmt.Errorf("capture kitty window viewport: %w", err)
	}
	return targets.NormalizeViewportText(string(out)), nil
}

func notifyTargetsFailure(err error, d Deps) error {
	if err == nil {
		return nil
	}

	msg := strings.TrimSpace(err.Error())
	const prefix = "blf kitty targets: "
	if !strings.HasPrefix(msg, prefix) {
		msg = prefix + msg
	}
	if d.Stderr != nil {
		_, _ = fmt.Fprintln(d.Stderr, msg)
	}
	if notifyErr := ShowError(d, "blf kitty targets", err.Error()); notifyErr != nil {
		return fmt.Errorf("notify kitty targets failure: %w", notifyErr)
	}
	return nil
}

func runResumeCommandInWindow(targetMatch, command string, d Deps) error {
	if d.RunCommand == nil {
		return errors.New("kitty command runner is not configured")
	}
	if _, err := d.RunCommand("kitty", "@", "send-text", "--match", targetMatch, "--", command+"\r"); err != nil {
		return fmt.Errorf("send resume command to kitty window: %w", err)
	}
	return nil
}
