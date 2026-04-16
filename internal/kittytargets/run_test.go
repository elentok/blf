package kittytargets

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/targets"
)

func TestExecuteTopLevelOpensOverlay(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "17")

	origLookPath := lookPath
	origExecutablePath := executablePath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	origStderr := stderr
	t.Cleanup(func() {
		lookPath = origLookPath
		executablePath = origExecutablePath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
		stderr = origStderr
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	executablePath = func() (string, error) { return "/tmp/go-build/blf", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "kitty" || strings.Join(args, " ") != "@ get-text --extent screen --match id:17" {
			t.Fatalf("unexpected output command: %s %v", name, args)
		}
		return []byte("visit https://example.com"), nil
	}

	var calls [][]string
	var errBuf bytes.Buffer
	stderr = &errBuf
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	if err := Execute(nil); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected one run command call, got %d", len(calls))
	}

	got := strings.Join(calls[0], " ")
	for _, snippet := range []string{
		"kitty @ launch",
		"--type=overlay",
		"--copy-env",
		"/tmp/go-build/blf kitty targets --overlay --target 17",
	} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected %q in launch command: %s", snippet, got)
		}
	}
}

func TestExecuteTopLevelNoTargetsDoesNotOpenOverlay(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "17")

	origLookPath := lookPath
	origExecutablePath := executablePath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	origStderr := stderr
	t.Cleanup(func() {
		lookPath = origLookPath
		executablePath = origExecutablePath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
		stderr = origStderr
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	executablePath = func() (string, error) { return "/tmp/go-build/blf", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "kitty" || strings.Join(args, " ") != "@ get-text --extent screen --match id:17" {
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

func TestExecuteTopLevelReturnsExecutableResolutionError(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "17")

	origLookPath := lookPath
	origExecutablePath := executablePath
	origStderr := stderr
	t.Cleanup(func() {
		lookPath = origLookPath
		executablePath = origExecutablePath
		stderr = origStderr
	})

	lookPath = func(string) (string, error) { return "/usr/bin/kitty", nil }
	executablePath = func() (string, error) { return "", errors.New("boom") }

	var errBuf bytes.Buffer
	stderr = &errBuf

	err := Execute(nil)
	if err == nil || !strings.Contains(err.Error(), "resolve current executable") {
		t.Fatalf("error = %v", err)
	}
	if got := errBuf.String(); got != "blf kitty targets: resolve current executable: boom\n" {
		t.Fatalf("stderr = %q", got)
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

	if err := Execute([]string{"--overlay", "--target", "17"}); err != nil {
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

	err := Execute([]string{"--overlay", "--target", "17"})
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

	if err := Execute([]string{"--overlay", "--target", "17"}); err != nil {
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
	windowID, err := parseOverlayArgs([]string{"--target", "17"})
	if err != nil {
		t.Fatalf("parseOverlayArgs returned error: %v", err)
	}
	if windowID != "17" {
		t.Fatalf("windowID = %q", windowID)
	}

	_, err = parseOverlayArgs([]string{"--target"})
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

	if err := runResumeCommandInWindow("17", "codex resume abc123"); err != nil {
		t.Fatalf("runResumeCommandInWindow returned error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("expected one kitty call, got %d", len(calls))
	}
	if got := strings.Join(calls[0], " "); got != "kitty @ send-text --match id:17 -- codex resume abc123\r" {
		t.Fatalf("call = %q", got)
	}
}
