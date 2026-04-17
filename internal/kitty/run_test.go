package kitty

import (
	"os"
	"strings"
	"testing"
)

func TestListOSWindowsCommand(t *testing.T) {
	out := &strings.Builder{}
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[
				{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
			]`), nil
		},
		Stdout: out,
		Stderr: &strings.Builder{},
	}

	err := ListOSWindowsCommand(d)
	if err != nil {
		t.Fatalf("ListOSWindowsCommand returned error: %v", err)
	}
	if got := out.String(); got != "\x1b[1;34m7: shell\x1b[m\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestGotoOSWindowWithExplicitID(t *testing.T) {
	var commands []string
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ ls":
				return []byte(`[
					{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
				]`), nil
			case name == "kitten" && strings.Join(args, " ") == "@ focus-tab --match id:70":
				return []byte{}, nil
			}
			t.Fatalf("unexpected command: %s %v", name, args)
			return nil, nil
		},
		Stdout: &strings.Builder{},
		Stderr: &strings.Builder{},
	}

	err := GotoOSWindow([]string{"7"}, d)
	if err != nil {
		t.Fatalf("GotoOSWindow returned error: %v", err)
	}
	if strings.Join(commands, "\n") != "kitty @ ls\nkitten @ focus-tab --match id:70" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestNewSessionLaunchesOverlay(t *testing.T) {
	var commands []string
	d := Deps{
		ExecutablePath: func() (string, error) { return "/tmp/blf", nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		Stdin:  strings.NewReader(""),
		Stdout: &strings.Builder{},
		Stderr: &strings.Builder{},
	}

	if err := NewSession(nil, d); err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	want := "kitty @ launch --type=overlay --copy-env --cwd=current -- /tmp/blf kitty new-session --overlay"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestNewSessionOverlayCreatesAndSwitches(t *testing.T) {
	var commands []string
	out := &strings.Builder{}
	d := Deps{
		Stdin:       strings.NewReader("proj\n"),
		Stdout:      out,
		Stderr:      &strings.Builder{},
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		FileExists:  func(string) (bool, error) { return false, nil },
		Getwd:       func() (string, error) { return "/work", nil },
		WriteFile:   func(string, []byte, os.FileMode) error { return nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
	}

	if err := NewSession([]string{"--overlay"}, d); err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if got := out.String(); got != "Session name: " {
		t.Fatalf("prompt = %q", got)
	}
	want := "kitten @ action goto_session /Users/test/.local/share/kitty/sessions/proj.kitty-session"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestSessionsCommandLaunchesOverlay(t *testing.T) {
	var commands []string
	d := Deps{
		ExecutablePath: func() (string, error) { return "/tmp/blf", nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		Stdin:  strings.NewReader(""),
		Stdout: &strings.Builder{},
		Stderr: &strings.Builder{},
	}

	if err := SessionsCommand(nil, d); err != nil {
		t.Fatalf("SessionsCommand returned error: %v", err)
	}

	want := "kitty @ launch --type=overlay --copy-env --cwd=current -- /tmp/blf kitty sessions --overlay"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestSessionsOverlayShowsErrorWhenNoActiveSessions(t *testing.T) {
	var commands []string
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		ReadDir:     func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		Stdout: &strings.Builder{},
		Stderr: &strings.Builder{},
	}

	if err := SessionsCommand([]string{"--overlay"}, d); err != nil {
		t.Fatalf("SessionsCommand returned error: %v", err)
	}

	want := `kitten @ action show_error "blf kitty sessions" "No active kitty sessions"`
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}
