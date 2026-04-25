package kitty

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/targets"
)

func TestTargetsFallsBackToCurrentWindowOutsideOverlay(t *testing.T) {
	origRunTargetsPopupUI := runTargetsPopupUI
	t.Cleanup(func() { runTargetsPopupUI = origRunTargetsPopupUI })

	var matches []string
	var gotLines []string
	runTargetsPopupUI = func(lines []string, tgts []targets.Target, title string, notify func(string), runResumeCmd func(string) error) error {
		gotLines = append([]string{}, lines...)
		return nil
	}

	d := Deps{
		LookupEnv: func(key string) (string, bool) {
			if key == "KITTY_WINDOW_ID" {
				return "17", true
			}
			return "", false
		},
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || len(args) != 6 || strings.Join(args[:5], " ") != "@ get-text --extent screen --match" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			matches = append(matches, args[5])
			if args[5] == "state:overlay_parent" {
				return nil, errors.New("not in overlay")
			}
			return []byte("visit https://example.com"), nil
		},
	}

	if err := Targets(nil, d); err != nil {
		t.Fatalf("Targets returned error: %v", err)
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

func TestTargetsNoTargetsShowsError(t *testing.T) {
	var calls []string
	var errBuf bytes.Buffer

	d := Deps{
		LookupEnv: func(key string) (string, bool) {
			if key == "KITTY_WINDOW_ID" {
				return "17", true
			}
			return "", false
		},
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ get-text --extent screen --match state:overlay_parent":
				return []byte("nothing here"), nil
			case name == "kitten" && strings.Join(args, " ") == `@ action show_error "blf kitty targets" "no targets found in current kitty window"`:
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
		Stderr: &errBuf,
	}

	if err := Targets(nil, d); err != nil {
		t.Fatalf("expected nil for no-targets case, got %v", err)
	}

	if strings.Join(calls, "\n") != "kitty @ get-text --extent screen --match state:overlay_parent\nkitty @ get-text --extent screen --match state:overlay_parent\nkitten @ action show_error \"blf kitty targets\" \"no targets found in current kitty window\"" {
		t.Fatalf("calls = %v", calls)
	}
	if got := errBuf.String(); got != "blf kitty targets: no targets found in current kitty window\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestTargetsUsesOverlayParentWhenAvailable(t *testing.T) {
	origRunTargetsPopupUI := runTargetsPopupUI
	t.Cleanup(func() { runTargetsPopupUI = origRunTargetsPopupUI })

	var matches []string
	runTargetsPopupUI = func(lines []string, tgts []targets.Target, title string, notify func(string), runResumeCmd func(string) error) error {
		return nil
	}

	d := Deps{
		LookupEnv: func(key string) (string, bool) {
			if key == "KITTY_WINDOW_ID" {
				return "24", true
			}
			return "", false
		},
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			matches = append(matches, args[5])
			if args[5] != "state:overlay_parent" {
				t.Fatalf("unexpected match %q", args[5])
			}
			return []byte("visit https://example.com"), nil
		},
	}

	if err := Targets(nil, d); err != nil {
		t.Fatalf("Targets returned error: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected overlay-parent probe and capture, got %v", matches)
	}
}

func TestTargetsExplicitTargetNoTargetsIsNil(t *testing.T) {
	var calls []string

	d := Deps{
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ get-text --extent screen --match id:17":
				return []byte("nothing here"), nil
			case name == "kitten" && strings.Join(args, " ") == `@ action show_error "blf kitty targets" "no targets found in current kitty window"`:
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
	}

	if err := Targets([]string{"--target", "17"}, d); err != nil {
		t.Fatalf("expected nil for no-targets case, got %v", err)
	}
}

func TestTargetsNoTargetsReturnsErrorWhenNotificationFails(t *testing.T) {
	d := Deps{
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name == "kitty" {
				return []byte("nothing here"), nil
			}
			return nil, errors.New("show_error failed")
		},
	}

	err := Targets([]string{"--target", "17"}, d)
	if err == nil || !strings.Contains(err.Error(), "notify kitty targets failure") {
		t.Fatalf("error = %v", err)
	}
}

func TestTargetsRunsPopupUI(t *testing.T) {
	origRunTargetsPopupUI := runTargetsPopupUI
	t.Cleanup(func() { runTargetsPopupUI = origRunTargetsPopupUI })

	var gotLines []string
	var gotTargets []targets.Target
	var gotTitle string
	runTargetsPopupUI = func(lines []string, tgts []targets.Target, title string, notify func(string), runResumeCmd func(string) error) error {
		gotLines = append([]string{}, lines...)
		gotTargets = append([]targets.Target(nil), tgts...)
		gotTitle = title
		return nil
	}

	d := Deps{
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ get-text --extent screen --match id:17" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte("visit https://example.com"), nil
		},
	}

	if err := Targets([]string{"--target", "17"}, d); err != nil {
		t.Fatalf("Targets returned error: %v", err)
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

func TestResolveTargetMatch(t *testing.T) {
	match, err := resolveTargetMatch([]string{"--target", "17"}, Deps{})
	if err != nil {
		t.Fatalf("resolveTargetMatch returned error: %v", err)
	}
	if match != "id:17" {
		t.Fatalf("match = %q", match)
	}

	_, err = resolveTargetMatch([]string{"--target"}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestNotifyTargetsFailureDoesNotDuplicateBody(t *testing.T) {
	var (
		calls  []string
		errBuf bytes.Buffer
	)
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		Stderr: &errBuf,
	}

	err := notifyTargetsFailure(errors.New("blf kitty targets: selected target is not openable"), d)
	if err != nil {
		t.Fatalf("notifyTargetsFailure returned error: %v", err)
	}

	if strings.Join(calls, "\n") != `kitten @ action show_error "blf kitty targets" "blf kitty targets: selected target is not openable"` {
		t.Fatalf("calls = %v", calls)
	}
	if got := errBuf.String(); got != "blf kitty targets: selected target is not openable\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestNotifyTargetsFailureReturnsErrorWhenShowErrorFails(t *testing.T) {
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	}

	err := notifyTargetsFailure(errors.New("selected target is not openable"), d)
	if err == nil || !strings.Contains(err.Error(), "notify kitty targets failure") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunResumeCommandInWindowSendsCommand(t *testing.T) {
	var calls []string
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
	}

	if err := runResumeCommandInWindow("state:overlay_parent", "codex resume abc123", d); err != nil {
		t.Fatalf("runResumeCommandInWindow returned error: %v", err)
	}

	if strings.Join(calls, "\n") != "kitty @ send-text --match state:overlay_parent -- codex resume abc123\r" {
		t.Fatalf("calls = %v", calls)
	}
}
