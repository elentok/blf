package tmuxlinks

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestRunMenuBuildsAndRunsTmuxMenu(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(file string) (string, error) {
		if file != "tmux" {
			t.Fatalf("lookPath called with %q, want tmux", file)
		}
		return "/usr/bin/tmux", nil
	}

	outputCmd = func(name string, args ...string) ([]byte, error) {
		if name != "tmux" {
			t.Fatalf("output command name = %q, want tmux", name)
		}
		want := []string{"capture-pane", "-pJ", "-S", "-10000"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("capture args = %#v, want %#v", args, want)
		}
		return []byte("url: https://example.com/a\nagain https://example.com/a\nother http://x.example/b."), nil
	}

	var runName string
	var runArgs []string
	runCmd = func(name string, args ...string) error {
		runName = name
		runArgs = append([]string{}, args...)
		return nil
	}

	if err := RunMenu(ModeCopy); err != nil {
		t.Fatalf("RunMenu returned error: %v", err)
	}

	if runName != "tmux" {
		t.Fatalf("run command name = %q, want tmux", runName)
	}
	if len(runArgs) < 7 {
		t.Fatalf("run args too short: %#v", runArgs)
	}
	if runArgs[0] != "display-menu" {
		t.Fatalf("expected display-menu, got %#v", runArgs)
	}
	if runArgs[1] != "-T" || runArgs[2] != "Copy URL" {
		t.Fatalf("expected Copy URL title, got %#v", runArgs)
	}
	if runArgs[3] != "-x" || runArgs[4] != "C" || runArgs[5] != "-y" || runArgs[6] != "C" {
		t.Fatalf("expected centered menu args, got %#v", runArgs[:7])
	}
}

func TestRunMenuFailsOutsideTmux(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() { lookPath = origLookPath })
	lookPath = func(file string) (string, error) { return "/usr/bin/tmux", nil }

	_ = os.Unsetenv("TMUX")

	err := RunMenu(ModeOpen)
	if err == nil {
		t.Fatal("expected error outside tmux")
	}
}

func TestRunMenuPropagatesMenuError(t *testing.T) {
	t.Setenv("TMUX", "1")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(file string) (string, error) { return "/usr/bin/tmux", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return []byte("https://example.com"), nil
	}
	boom := errors.New("boom")
	var calls [][]string
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		if len(calls) == 1 {
			return boom
		}
		return nil
	}

	err := RunMenu(ModeOpen)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 tmux calls (menu + display-message), got %d", len(calls))
	}
	if len(calls[1]) < 3 || calls[1][1] != "display-message" {
		t.Fatalf("expected second call to be display-message, got %#v", calls[1])
	}
}

func TestRunMenuNoLinksReturnsNilButShowsMessage(t *testing.T) {
	t.Setenv("TMUX", "1")

	origLookPath := lookPath
	origOutputCmd := outputCmd
	origRunCmd := runCmd
	t.Cleanup(func() {
		lookPath = origLookPath
		outputCmd = origOutputCmd
		runCmd = origRunCmd
	})

	lookPath = func(file string) (string, error) { return "/usr/bin/tmux", nil }
	outputCmd = func(name string, args ...string) ([]byte, error) {
		return []byte("no urls here"), nil
	}

	var calls [][]string
	runCmd = func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	err := RunMenu(ModeOpen)
	if err != nil {
		t.Fatalf("expected nil error for no-links case, got %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one tmux call (display-message), got %d", len(calls))
	}
	if len(calls[0]) < 5 || calls[0][1] != "display-message" || calls[0][2] != "-d" || calls[0][3] != "5000" {
		t.Fatalf("expected display-message with delay, got %#v", calls[0])
	}
}
