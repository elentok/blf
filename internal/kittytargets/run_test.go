package kittytargets

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/targets"
)

func TestExecuteTopLevelFallsBackToCurrentWindowOutsideOverlay(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "17")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }

	var matches []string
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "kitty" || len(args) != 6 || strings.Join(args[:5], " ") != "@ get-text --extent screen --match" {
			t.Fatalf("unexpected output command: %s %v", name, args)
		}
		matches = append(matches, args[5])
		if args[5] == "state:overlay_parent" {
			return nil, errors.New("not in overlay")
		}
		return []byte("visit https://example.com"), nil
	}

	origRunPopupUI := runPopupUI
	t.Cleanup(func() { runPopupUI = origRunPopupUI })
	var gotLines []string
	runPopupUI = func(lines []string, tgts []targets.Target, title string, notify func(string), runResumeCmd func(string) error) error {
		gotLines = append([]string{}, lines...)
		return nil
	}

	if err := Execute(nil); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("expected two capture attempts, got %v", matches)
	}
	if matches[0] != "state:overlay_parent" || matches[1] != "id:17" {
		t.Fatalf("capture matches = %v", matches)
	}
	if len(gotLines) != 1 || gotLines[0] != "visit https://example.com" {
		t.Fatalf("lines = %#v", gotLines)
	}
}

func TestExecuteTopLevelNoTargetsDoesNotOpenOverlay(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "17")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	origStderr := stderr
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
		stderr = origStderr
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "kitty" || strings.Join(args, " ") != "@ get-text --extent screen --match state:overlay_parent" {
			t.Fatalf("unexpected output command: %s %v", name, args)
		}
		return []byte("nothing here"), nil
	}

	var calls [][]string
	var errBuf bytes.Buffer
	stderr = &errBuf
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	if err := Execute(nil); err != nil {
		t.Fatalf("expected nil for no targets case, got %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected one notification call, got %d", len(calls))
	}
	if got := strings.Join(calls[0], " "); got != `kitten @ action show_error "blf kitty targets" "no targets found in current kitty window"` {
		t.Fatalf("notification call = %q", got)
	}
	if got := errBuf.String(); got != "blf kitty targets: no targets found in current kitty window\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestExecuteTopLevelUsesOverlayParentWhenAvailable(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "24")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunPopupUI := runPopupUI
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runPopupUI = origRunPopupUI
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }

	var matches []string
	outputCmd = func(name string, args ...string) ([]byte, error) {
		matches = append(matches, args[5])
		if args[5] != "state:overlay_parent" {
			t.Fatalf("unexpected match %q", args[5])
		}
		return []byte("visit https://example.com"), nil
	}
	runPopupUI = func(lines []string, tgts []targets.Target, title string, notify func(string), runResumeCmd func(string) error) error {
		return nil
	}

	if err := Execute(nil); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected overlay-parent probe and capture, got %v", matches)
	}
	for _, match := range matches {
		if match != "state:overlay_parent" {
			t.Fatalf("matches = %v", matches)
		}
	}
}

func TestExecuteOverlayNoTargetsIsNil(t *testing.T) {
	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "kitty" || strings.Join(args, " ") != "@ get-text --extent screen --match id:17" {
			t.Fatalf("unexpected output command: %s %v", name, args)
		}
		return []byte("nothing here"), nil
	}

	var calls [][]string
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	if err := Execute([]string{"--target", "17"}); err != nil {
		t.Fatalf("expected nil for no targets case, got %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected one notification call, got %d", len(calls))
	}
	if got := strings.Join(calls[0], " "); got != `kitten @ action show_error "blf kitty targets" "no targets found in current kitty window"` {
		t.Fatalf("notification call = %q", got)
	}
}

func TestExecuteOverlayNoTargetsReturnsErrorWhenNotificationFails(t *testing.T) {
	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return []byte("nothing here"), nil
	}
	runCmd = func(name string, args ...string) error {
		return errors.New("show_error failed")
	}

	err := Execute([]string{"--target", "17"})
	if err == nil || !strings.Contains(err.Error(), "notify kitty targets failure") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecuteOverlayRunsPopupUI(t *testing.T) {
	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunPopupUI := runPopupUI
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runPopupUI = origRunPopupUI
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "kitty" || strings.Join(args, " ") != "@ get-text --extent screen --match id:17" {
			t.Fatalf("unexpected output command: %s %v", name, args)
		}
		return []byte("visit https://example.com"), nil
	}

	var gotLines []string
	var gotTargets []targets.Target
	var gotTitle string
	runPopupUI = func(lines []string, tgts []targets.Target, title string, notify func(string), runResumeCmd func(string) error) error {
		gotLines = append([]string{}, lines...)
		gotTargets = append([]targets.Target(nil), tgts...)
		gotTitle = title
		return nil
	}

	if err := Execute([]string{"--target", "17"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if gotTitle != "Kitty Targets" {
		t.Fatalf("title = %q", gotTitle)
	}
	if len(gotLines) != 1 || gotLines[0] != "visit https://example.com" {
		t.Fatalf("lines = %#v", gotLines)
	}
	if len(gotTargets) != 1 || gotTargets[0].Text != "https://example.com" {
		t.Fatalf("targets = %#v", gotTargets)
	}
}

func TestParseOverlayArgs(t *testing.T) {
	match, err := resolveTargetMatch([]string{"--target", "17"})
	if err != nil {
		t.Fatalf("resolveTargetMatch returned error: %v", err)
	}
	if match != "id:17" {
		t.Fatalf("match = %q", match)
	}

	_, err = resolveTargetMatch([]string{"--target"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestNotifyFailureDoesNotDuplicateBody(t *testing.T) {
	origRunCmd := runCmd
	origStderr := stderr
	t.Cleanup(func() { runCmd = origRunCmd })
	t.Cleanup(func() { stderr = origStderr })

	var got string
	var errBuf bytes.Buffer
	stderr = &errBuf
	runCmd = func(name string, args ...string) error {
		if len(args) >= 4 {
			got = args[3]
		}
		return nil
	}

	err := notifyFailure(errors.New("blf kitty targets: selected target is not openable"))
	if err != nil {
		t.Fatalf("notifyFailure returned error: %v", err)
	}

	if got != `"blf kitty targets" "blf kitty targets: selected target is not openable"` {
		t.Fatalf("unexpected show_error arg: %q", got)
	}
	if got := errBuf.String(); got != "blf kitty targets: selected target is not openable\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestNotifyFailureReturnsErrorWhenShowErrorFails(t *testing.T) {
	origRunCmd := runCmd
	t.Cleanup(func() { runCmd = origRunCmd })

	runCmd = func(name string, args ...string) error {
		return errors.New("boom")
	}

	err := notifyFailure(errors.New("selected target is not openable"))
	if err == nil || !strings.Contains(err.Error(), "notify kitty targets failure") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunResumeCommandInWindowSendsCommand(t *testing.T) {
	origRunCmd := runCmd
	t.Cleanup(func() { runCmd = origRunCmd })

	var calls [][]string
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	if err := runResumeCommandInWindow("state:overlay_parent", "codex resume abc123"); err != nil {
		t.Fatalf("runResumeCommandInWindow returned error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected one kitty call, got %d", len(calls))
	}
	if got := strings.Join(calls[0], " "); got != "kitty @ send-text --match state:overlay_parent -- codex resume abc123\r" {
		t.Fatalf("call = %q", got)
	}
}
