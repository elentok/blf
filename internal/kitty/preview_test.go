package kitty

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderSessionPreview(t *testing.T) {
	d := Deps{
		RunCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
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
		"Empty session: proj\n\n",
		"No live tabs, saved session:",
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
	if strings.Contains(got, "Path: ") {
		t.Fatalf("preview contains unexpected Path line:\n%s", got)
	}
	if !strings.Contains(got, "Empty session: proj\n\n") {
		t.Fatalf("preview is missing blank line below header:\n%s", got)
	}
}

func TestRenderSessionPreviewShowsLiveTabsWhenActive(t *testing.T) {
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls --match-tab session:^proj$" {
				t.Fatalf("command = %s %v", name, args)
			}
			return []byte(`[{"id":1,"tabs":[{"id":10,"title":"shell"},{"id":11,"title":"docs"}]}]`), nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path != "/tmp/proj.kitty-session" {
				t.Fatalf("path = %q", path)
			}
			return []byte(strings.Join([]string{
				`new_tab "saved shell"`,
				`cd "/work tree"`,
				`launch`,
				`new_tab "saved docs"`,
				`launch lazygit`,
				"",
			}, "\n")), nil
		},
	}

	got, err := RenderSessionPreview("/tmp/proj.kitty-session", d)
	if err != nil {
		t.Fatalf("RenderSessionPreview returned error: %v", err)
	}

	for _, want := range []string{
		"Live session: proj",
		"Tab 1: shell",
		"|- cd: /work tree",
		"`- window 1: shell",
		"Tab 2: docs",
		"`- window 1: lazygit",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"No live tabs, saved session:",
		"OS window 1",
		"Path: ",
		"Live tabs:",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("preview contains unexpected %q:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "proj\x1b[m\n\n") {
		t.Fatalf("preview is missing blank line below header:\n%s", got)
	}
}

func TestRenderSessionPreviewRequiresReadFileWhenInactive(t *testing.T) {
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

func TestSessionChoicesCommand(t *testing.T) {
	got, err := sessionChoicesCommand(Deps{
		ExecutablePath: func() (string, error) { return "/tmp/blf binary", nil },
	})
	if err != nil {
		t.Fatalf("sessionChoicesCommand returned error: %v", err)
	}
	if got != "'/tmp/blf binary' kitty __list-session-choices" {
		t.Fatalf("command = %q", got)
	}
}

func TestSessionDeleteCommand(t *testing.T) {
	got, err := sessionDeleteCommand(Deps{
		ExecutablePath: func() (string, error) { return "/tmp/blf binary", nil },
	})
	if err != nil {
		t.Fatalf("sessionDeleteCommand returned error: %v", err)
	}
	if got != "'/tmp/blf binary' kitty __delete-session-file {1}" {
		t.Fatalf("command = %q", got)
	}
}

func TestSessionEditCommand(t *testing.T) {
	got, err := sessionEditCommand(Deps{
		ExecutablePath: func() (string, error) { return "/tmp/blf binary", nil },
	})
	if err != nil {
		t.Fatalf("sessionEditCommand returned error: %v", err)
	}
	if got != "'/tmp/blf binary' kitty __edit-session-file {1}" {
		t.Fatalf("command = %q", got)
	}
}

func TestSessionPreviewWindowLayout(t *testing.T) {
	if sessionPreviewWindow != "right,60%,wrap,<70(down,50%,wrap)" {
		t.Fatalf("layout = %q", sessionPreviewWindow)
	}
}

func TestFZFNavigationBind(t *testing.T) {
	if fzfNavigationBind != "ctrl-j:down,ctrl-k:up" {
		t.Fatalf("bind = %q", fzfNavigationBind)
	}
}

func TestSessionFooter(t *testing.T) {
	if sessionFooter != "ctrl-d: delete, ctrl-o: edit" {
		t.Fatalf("footer = %q", sessionFooter)
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
