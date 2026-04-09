package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"
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
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
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
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
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
		runTargets: func([]string) error { return nil },
		fileExists: func(string) (bool, error) { return false, nil },
		readFile:   func(string) ([]byte, error) { return nil, nil },
		stdout:     &strings.Builder{},
		stderr:     &strings.Builder{},
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
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
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
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
		stdout:       &strings.Builder{},
		stderr:       &strings.Builder{},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped boom error, got %v", err)
	}
}

func TestExecuteRoutesTmuxTargets(t *testing.T) {
	var got []string
	d := deps{
		openURL:      func(string) error { return nil },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		runTargets: func(args []string) error {
			got = append([]string{}, args...)
			return nil
		},
		fileExists: func(string) (bool, error) { return false, nil },
		readFile:   func(string) ([]byte, error) { return nil, nil },
		stdout:     &strings.Builder{},
		stderr:     &strings.Builder{},
	}

	err := execute([]string{"tmux-targets", "--popup", "--target", "%1"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if strings.Join(got, " ") != "--popup --target %1" {
		t.Fatalf("tmux-targets called with %v", got)
	}
}

func TestExecuteRoutesVersion(t *testing.T) {
	orig := version
	version = "v9.9.9"
	t.Cleanup(func() { version = orig })

	out := &strings.Builder{}
	d := deps{
		openURL:      func(string) error { return nil },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
		stdout:       out,
		stderr:       &strings.Builder{},
	}

	err := execute([]string{"version"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if out.String() != "blf v9.9.9\n" {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestExecuteRoutesQueryStringAliases(t *testing.T) {
	for _, command := range []string{"querystring", "qs"} {
		t.Run(command, func(t *testing.T) {
			out := &strings.Builder{}
			d := deps{
				openURL:      func(string) error { return nil },
				copyText:     func(string) error { return nil },
				runTmuxLinks: func(string) error { return nil },
				runTargets:   func([]string) error { return nil },
				fileExists:   func(string) (bool, error) { return false, nil },
				readFile:     func(string) ([]byte, error) { return nil, nil },
				stdout:       out,
				stderr:       &strings.Builder{},
				stdin:        strings.NewReader(""),
			}

			err := execute([]string{command, "a=1", "a"}, d)
			if err != nil {
				t.Fatalf("execute returned error: %v", err)
			}
			if out.String() != "1\n" {
				t.Fatalf("querystring output = %q", out.String())
			}
		})
	}
}

func TestExecuteRoutesCal(t *testing.T) {
	out := &strings.Builder{}
	d := deps{
		openURL:      func(string) error { return nil },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
		stdout:       out,
		stderr:       &strings.Builder{},
		stdin:        strings.NewReader(""),
		now: func() time.Time {
			return time.Date(2026, time.April, 8, 12, 0, 0, 0, time.Local)
		},
	}

	err := execute([]string{"cal", "2026-04-08"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "         Apr 2026          ") {
		t.Fatalf("cal output = %q", out.String())
	}
}

func TestExecuteRoutesSum(t *testing.T) {
	out := &strings.Builder{}
	d := deps{
		openURL:      func(string) error { return nil },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return false, nil },
		readFile:     func(string) ([]byte, error) { return nil, nil },
		stdout:       out,
		stderr:       &strings.Builder{},
		stdin:        strings.NewReader("1 apple\n2 banana\n"),
		now:          time.Now,
	}

	err := execute([]string{"sum"}, d)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if out.String() != "= 3\n" {
		t.Fatalf("sum output = %q", out.String())
	}
}

func TestExecuteRoutesNPMScripts(t *testing.T) {
	out := &strings.Builder{}
	d := deps{
		openURL:      func(string) error { return nil },
		copyText:     func(string) error { return nil },
		runTmuxLinks: func(string) error { return nil },
		runTargets:   func([]string) error { return nil },
		fileExists:   func(string) (bool, error) { return true, nil },
		readFile: func(string) ([]byte, error) {
			return []byte(`{"scripts":{"dev":"vite"}}`), nil
		},
		stdout: out,
		stderr: &strings.Builder{},
	}

	if err := execute([]string{"npm-scripts"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if out.String() != "\x1b[32mdev\x1b[0m  - vite\n" {
		t.Fatalf("npm-scripts output = %q", out.String())
	}
}
