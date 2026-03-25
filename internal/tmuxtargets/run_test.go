package tmuxtargets

import (
	"errors"
	"strings"
	"testing"
)

func TestExecuteTopLevelOpensPopup(t *testing.T) {
	t.Setenv("TMUX", "1")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(string) (string, error) { return "/usr/bin/tmux", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "display-message" {
			return []byte("%1 120 40 12 7"), nil
		}
		return nil, errors.New("unexpected output command")
	}

	var calls [][]string
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	err := Execute(nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one run command call, got %d", len(calls))
	}
	if calls[0][1] != "display-popup" {
		t.Fatalf("expected display-popup, got %#v", calls[0])
	}
	joined := strings.Join(calls[0], " ")
	for _, snippet := range []string{"-x 12", "-y 7", "-w 120", "-h 40", "-B"} {
		if !strings.Contains(joined, snippet) {
			t.Fatalf("expected %q in popup args: %s", snippet, joined)
		}
	}
}

func TestExecutePopupNoTargetsIsNil(t *testing.T) {
	t.Setenv("TMUX", "1")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(string) (string, error) { return "/usr/bin/tmux", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "capture-pane" {
			return []byte("nothing here"), nil
		}
		return nil, errors.New("unexpected output command")
	}

	var messages int
	runCmd = func(name string, args ...string) error {
		if len(args) > 0 && args[0] == "display-message" {
			messages++
		}
		return nil
	}

	err := Execute([]string{"--popup", "--target", "%1"})
	if err != nil {
		t.Fatalf("expected nil for no targets case, got %v", err)
	}
	if messages != 1 {
		t.Fatalf("expected one display-message call, got %d", messages)
	}
}

func TestParsePopupArgs(t *testing.T) {
	pane, err := parsePopupArgs([]string{"--target", "%1"})
	if err != nil {
		t.Fatalf("parsePopupArgs returned error: %v", err)
	}
	if pane != "%1" {
		t.Fatalf("pane = %q", pane)
	}

	_, err = parsePopupArgs([]string{"--target"})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("expected usage error, got %v", err)
	}
}
