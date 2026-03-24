package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestExecuteRoutesOpen(t *testing.T) {
	var got string
	d := deps{
		openURL: func(s string) error {
			got = s
			return nil
		},
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		stdout:       &strings.Builder{},
		stderr:       &strings.Builder{},
	}

	err := execute([]string{"open", "https://example.com"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("open called with %q", got)
	}
}

func TestExecuteRoutesCopyWithSpaces(t *testing.T) {
	var got string
	d := deps{
		openURL: func(string) error { return nil },
		copyText: func(s string) error {
			got = s
			return nil
		},
		runTmuxLinks: func(string) error { return nil },
		stdout:       &strings.Builder{},
		stderr:       &strings.Builder{},
	}

	err := execute([]string{"copy", "hello", "world"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("copy called with %q", got)
	}
}

func TestExecuteRoutesTmuxLinks(t *testing.T) {
	var got string
	d := deps{
		openURL:  func(string) error { return nil },
		copyText: func(string) error { return nil },
		runTmuxLinks: func(mode string) error {
			got = mode
			return nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := execute([]string{"tmux-links", "copy"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got != "copy" {
		t.Fatalf("tmux-links called with %q", got)
	}
}

func TestExecuteInvalidCommand(t *testing.T) {
	err := execute([]string{"wat"}, deps{
		openURL:      func(string) error { return nil },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		stdout:       &strings.Builder{},
		stderr:       &strings.Builder{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecutePropagatesActionError(t *testing.T) {
	boom := errors.New("boom")
	err := execute([]string{"open", "https://example.com"}, deps{
		openURL:      func(string) error { return boom },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		stdout:       &strings.Builder{},
		stderr:       &strings.Builder{},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped boom error, got %v", err)
	}
}
