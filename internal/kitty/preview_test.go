package kitty

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderSessionPreview(t *testing.T) {
	d := Deps{
		ReadFile: func(path string) ([]byte, error) {
			if path != "/tmp/proj.kitty-session" {
				t.Fatalf("path = %q", path)
			}
			return []byte(strings.Join([]string{
				`new_tab "proj"`,
				`cd "/work tree"`,
				`launch`,
				`launch lazygit`,
				`new_tab docs`,
				`layout splits`,
				`launch --type=os-window vim README.md`,
				"",
			}, "\n")), nil
		},
	}

	got, err := RenderSessionPreview("/tmp/proj.kitty-session", d)
	if err != nil {
		t.Fatalf("RenderSessionPreview returned error: %v", err)
	}

	for _, want := range []string{
		"Session: proj",
		"Path: /tmp/proj.kitty-session",
		"Tab 1: proj",
		"|- cd: /work tree",
		"|- window 1: shell",
		"`- window 2: lazygit",
		"Tab 2: docs",
		"|- layout: splits",
		"`- window 1: --type=os-window vim README.md",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview missing %q:\n%s", want, got)
		}
	}
}

func TestRenderSessionPreviewRequiresReadFile(t *testing.T) {
	_, err := RenderSessionPreview("/tmp/proj.kitty-session", Deps{})
	if err == nil || !strings.Contains(err.Error(), "read file helper") {
		t.Fatalf("error = %v", err)
	}
}

func TestPreviewSessionCommand(t *testing.T) {
	got, err := sessionPreviewCommand(Deps{
		ExecutablePath: func() (string, error) { return "/tmp/blf binary", nil },
	})
	if err != nil {
		t.Fatalf("sessionPreviewCommand returned error: %v", err)
	}
	if got != "'/tmp/blf binary' kitty __preview-session {1}" {
		t.Fatalf("command = %q", got)
	}
}

func TestSessionPreviewWindowLayout(t *testing.T) {
	if sessionPreviewWindow != "right,60%,wrap,<70(down,50%,wrap)" {
		t.Fatalf("layout = %q", sessionPreviewWindow)
	}
}

func TestPreviewSessionCommandPropagatesExecutableError(t *testing.T) {
	_, err := sessionPreviewCommand(Deps{
		ExecutablePath: func() (string, error) { return "", errors.New("boom") },
	})
	if err == nil || !strings.Contains(err.Error(), "resolve executable path") {
		t.Fatalf("error = %v", err)
	}
}
