package kitty

import (
	"errors"
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

func TestLSCommand(t *testing.T) {
	out := &strings.Builder{}
	d := Deps{
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[
				{"id":7,"is_active":true,"tabs":[
					{"id":70,"is_active":true,"title":"shell","windows":[
						{"id":700,"is_active":true,"title":"editor","session_name":"proj","cmdline":["nvim","main.go"],"last_reported_cmdline":"nvim main.go","foreground_processes":[{"pid":123,"cmdline":["go","test"],"cwd":"/work"}]}
					]}
				]}
			]`), nil
		},
		Stdout: out,
		Stderr: &strings.Builder{},
	}

	err := LSCommand(d)
	if err != nil {
		t.Fatalf("LSCommand returned error: %v", err)
	}
	if got := out.String(); got != ""+
		"- OS Window 7 (active)\n"+
		"\x1b[38;2;243;139;169;48;2;50;40;59m  - Tab 70 (active): shell\x1b[m\n"+
		"\x1b[38;2;249;226;176;48;2;51;49;59m    - Window 700 (active) [proj]: editor\x1b[m\n"+
		"      \x1b[38;2;137;180;250m- cmdline:\x1b[m nvim main.go\n"+
		"      \x1b[38;2;137;180;250m- last_reported_cmdline:\x1b[m nvim main.go\n"+
		"      \x1b[38;2;137;180;250m- Foreground processes:\x1b[m\n"+
		"        - Proc 123:\n"+
		"          \x1b[38;2;137;180;250m- cmdline:\x1b[m go... (1 more lines)\n"+
		"          \x1b[38;2;137;180;250m- cwd:\x1b[m /work\n" {
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

	err := GotoOSWindow("7", d)
	if err != nil {
		t.Fatalf("GotoOSWindow returned error: %v", err)
	}
	if strings.Join(commands, "\n") != "kitty @ ls\nkitten @ focus-tab --match id:70" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestNewSessionCreatesAndSwitches(t *testing.T) {
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

	if err := NewSession(d); err != nil {
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

func TestNewSessionSwitchesToExistingActiveSession(t *testing.T) {
	var (
		commands     []string
		writeInvoked bool
	)
	d := Deps{
		Stdin:       strings.NewReader("proj\n"),
		Stdout:      &strings.Builder{},
		Stderr:      &strings.Builder{},
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		FileExists:  func(string) (bool, error) { return true, nil },
		Getwd:       func() (string, error) { return "/work", nil },
		WriteFile: func(string, []byte, os.FileMode) error {
			writeInvoked = true
			return nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			switch strings.Join(append([]string{name}, args...), " ") {
			case `kitty @ ls --match-tab session:^proj$`:
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"shell"}]}]`), nil
			case "kitten @ action goto_session /Users/test/.local/share/kitty/sessions/proj.kitty-session":
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
	}

	if err := NewSession(d); err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if writeInvoked {
		t.Fatal("expected existing active session to skip file rewrite")
	}
	if strings.Join(commands, "\n") != "kitty @ ls --match-tab session:^proj$\nkitten @ action goto_session /Users/test/.local/share/kitty/sessions/proj.kitty-session" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestNewSessionRecreatesExistingZeroTabSession(t *testing.T) {
	var (
		commands     []string
		writeInvoked bool
	)
	d := Deps{
		Stdin:       strings.NewReader("proj\n"),
		Stdout:      &strings.Builder{},
		Stderr:      &strings.Builder{},
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		FileExists:  func(string) (bool, error) { return true, nil },
		Getwd:       func() (string, error) { return "/work", nil },
		WriteFile: func(string, []byte, os.FileMode) error {
			writeInvoked = true
			return nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			switch strings.Join(append([]string{name}, args...), " ") {
			case `kitty @ ls --match-tab session:^proj$`:
				return nil, errors.New("exit status 1")
			case "kitten @ action goto_session /Users/test/.local/share/kitty/sessions/proj.kitty-session":
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
	}

	if err := NewSession(d); err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if !writeInvoked {
		t.Fatal("expected zero-tab session to be rewritten")
	}
	if commands[len(commands)-1] != "kitten @ action goto_session /Users/test/.local/share/kitty/sessions/proj.kitty-session" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestDeleteSessionLaunchesOverlay(t *testing.T) {
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

	if err := DeleteSession(false, d); err != nil {
		t.Fatalf("DeleteSession returned error: %v", err)
	}

	want := "kitty @ launch --match state:focused --source-window state:focused --type=overlay --copy-env --cwd=current -- /tmp/blf kitty delete-session --overlay"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestSessionsCommandShowsErrorWhenNoSessions(t *testing.T) {
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

	if err := SessionsCommand(d); err != nil {
		t.Fatalf("SessionsCommand returned error: %v", err)
	}

	want := `kitten @ action show_error "blf kitty sessions" "No kitty sessions"`
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestDeleteSessionOverlayShowsErrorWhenNoSessions(t *testing.T) {
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

	if err := DeleteSession(true, d); err != nil {
		t.Fatalf("DeleteSession returned error: %v", err)
	}

	want := `kitten @ action show_error "blf kitty delete-session" "No kitty sessions"`
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestListSessionChoicesWritesChoices(t *testing.T) {
	out := &strings.Builder{}
	d := Deps{
		Stdout:      out,
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		ReadDir: func(string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "alpha.kitty-session"},
				fakeDirEntry{name: "beta.kitty-session"},
			}, nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[
				{"id":1,"is_active":true,"tabs":[
					{"id":10,"is_focused":true,"title":"active","session_name":"beta","windows":[{"last_focused_at":200}]},
					{"id":11,"title":"other","session_name":"alpha","windows":[{"last_focused_at":100}]}
				]}
			]`), nil
		},
	}

	if err := ListSessionChoices(d); err != nil {
		t.Fatalf("ListSessionChoices returned error: %v", err)
	}

	want := "/Users/test/.local/share/kitty/sessions/alpha.kitty-session\t\x1b[1;97malpha\x1b[m\n"
	if out.String() != want {
		t.Fatalf("output = %q", out.String())
	}
}

func TestEditSessionFileUsesEditor(t *testing.T) {
	d := Deps{
		Stdin:       strings.NewReader(""),
		Stdout:      &strings.Builder{},
		Stderr:      &strings.Builder{},
		LookupEnv:   func(key string) (string, bool) { return "true", key == "EDITOR" },
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
	}

	if err := EditSessionFile("/Users/test/.local/share/kitty/sessions/proj.kitty-session", d); err != nil {
		t.Fatalf("EditSessionFile returned error: %v", err)
	}
}

func TestEditSessionFileRequiresEditor(t *testing.T) {
	d := Deps{
		LookupEnv:   func(string) (string, bool) { return "", false },
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
	}

	err := EditSessionFile("/Users/test/.local/share/kitty/sessions/proj.kitty-session", d)
	if err == nil || !strings.Contains(err.Error(), "EDITOR environment variable is not set") {
		t.Fatalf("error = %v", err)
	}
}
