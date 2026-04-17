package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.isDir }
func (f fakeDirEntry) Type() fs.FileMode          { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestParseKittyOSWindowsSupportsTabsKeyVariants(t *testing.T) {
	t.Run("tabs", func(t *testing.T) {
		windows, err := parseKittyOSWindows([]byte(`[
			{"id":1,"is_active":true,"last_focused":false,"tabs":[
				{"id":10,"is_active":true,"is_focused":true,"title":"shell"},
				{"id":11,"is_active":false,"is_focused":false,"title":"logs"}
			]}
		]`))
		if err != nil {
			t.Fatalf("parseKittyOSWindows returned error: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("window count = %d", len(windows))
		}
		if got := joinKittyTabTitles(windows[0].Tabs); got != "shell, logs" {
			t.Fatalf("tab titles = %q", got)
		}
		if windows[0].Tabs[0].ID != 10 || !windows[0].Tabs[0].IsFocused {
			t.Fatalf("first tab = %+v", windows[0].Tabs[0])
		}
	})

	t.Run("tabs colon", func(t *testing.T) {
		windows, err := parseKittyOSWindows([]byte(`[
			{"id":2,"is_active":false,"last_focused":true,"tabs:":[{"id":20,"is_active":true,"is_focused":false,"title":"editor"}]}
		]`))
		if err != nil {
			t.Fatalf("parseKittyOSWindows returned error: %v", err)
		}
		if len(windows) != 1 {
			t.Fatalf("window count = %d", len(windows))
		}
		if got := joinKittyTabTitles(windows[0].Tabs); got != "editor" {
			t.Fatalf("tab titles = %q", got)
		}
	})
}

func TestFormatKittyOSWindowsStylesRows(t *testing.T) {
	got := formatKittyOSWindows([]kittyOSWindow{
		{ID: 1, IsActive: true, Tabs: []kittyTab{{Title: "shell"}, {Title: "logs"}}},
		{ID: 2, LastFocused: true, Tabs: []kittyTab{{Title: "editor"}}},
		{ID: 3, Tabs: []kittyTab{{Title: "plain"}}},
	})

	want := "" +
		"\x1b[1;34m1: shell, logs\x1b[m\n" +
		"\x1b[38;5;214m2: editor\x1b[m\n" +
		"3: plain\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFilterInactiveKittyOSWindowsRemovesActiveWindow(t *testing.T) {
	got := filterInactiveKittyOSWindows([]kittyOSWindow{
		{ID: 1, IsActive: true, Tabs: []kittyTab{{Title: "active"}}},
		{ID: 2, LastFocused: true, Tabs: []kittyTab{{Title: "other"}}},
		{ID: 3, Tabs: []kittyTab{{Title: "plain"}}},
	})

	if len(got) != 2 {
		t.Fatalf("window count = %d", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("filtered windows = %+v", got)
	}
}

func TestParseKittyOSWindowIDStripsANSI(t *testing.T) {
	got, err := parseKittyOSWindowID("\x1b[1;34m12: shell, logs\x1b[0m")
	if err != nil {
		t.Fatalf("parseKittyOSWindowID returned error: %v", err)
	}
	if got != "12" {
		t.Fatalf("id = %q", got)
	}
}

func TestParseKittyOSWindowIDRejectsInvalidSelection(t *testing.T) {
	_, err := parseKittyOSWindowID("not a selection")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFormatKittySessionFile(t *testing.T) {
	got := formatKittySessionFile("proj", "/tmp/work tree")
	want := "new_tab \"proj\"\ncd \"/tmp/work tree\"\nlaunch\n"
	if got != want {
		t.Fatalf("session file = %q, want %q", got, want)
	}
}

func TestPromptKittySessionNameRejectsSeparators(t *testing.T) {
	_, err := promptKittySessionName(strings.NewReader("a/b\n"), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "cannot contain path separators") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateKittySessionFile(t *testing.T) {
	var (
		gotDir     string
		gotPath    string
		gotPerm    os.FileMode
		gotContent string
	)
	d := deps{
		userHomeDir: func() (string, error) { return "/Users/test", nil },
		mkdirAll: func(path string, perm os.FileMode) error {
			gotDir = path
			if perm != 0o755 {
				t.Fatalf("mkdir perm = %v", perm)
			}
			return nil
		},
		fileExists: func(path string) (bool, error) {
			if path != "/Users/test/.local/share/kitty/sessions/proj.kitty-session" {
				t.Fatalf("exists path = %q", path)
			}
			return false, nil
		},
		getwd: func() (string, error) { return "/work tree", nil },
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			gotPath = path
			gotPerm = perm
			gotContent = string(data)
			return nil
		},
	}

	got, err := createKittySessionFile("proj", d)
	if err != nil {
		t.Fatalf("createKittySessionFile returned error: %v", err)
	}

	if got != "/Users/test/.local/share/kitty/sessions/proj.kitty-session" {
		t.Fatalf("path = %q", got)
	}
	if gotDir != "/Users/test/.local/share/kitty/sessions" {
		t.Fatalf("mkdir path = %q", gotDir)
	}
	if gotPath != got {
		t.Fatalf("write path = %q", gotPath)
	}
	if gotPerm != 0o644 {
		t.Fatalf("write perm = %v", gotPerm)
	}
	wantContent := "new_tab \"proj\"\ncd \"/work tree\"\nlaunch\n"
	if gotContent != wantContent {
		t.Fatalf("content = %q, want %q", gotContent, wantContent)
	}
}

func TestListKittyActiveSessions(t *testing.T) {
	d := deps{
		userHomeDir: func() (string, error) { return "/Users/test", nil },
		readDir: func(path string) ([]os.DirEntry, error) {
			if path != "/Users/test/.local/share/kitty/sessions" {
				t.Fatalf("readDir path = %q", path)
			}
			return []os.DirEntry{
				fakeDirEntry{name: "beta.kitty-session"},
				fakeDirEntry{name: "alpha.kitty-session"},
				fakeDirEntry{name: "notes.txt"},
				fakeDirEntry{name: "nested", isDir: true},
			}, nil
		},
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || len(args) != 4 || args[0] != "@" || args[1] != "ls" || args[2] != "--match-tab" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			switch args[3] {
			case `session:^/Users/test/\.local/share/kitty/sessions/alpha\.kitty-session$`:
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"one"},{"id":11,"title":"two"}]}]`), nil
			case `session:^/Users/test/\.local/share/kitty/sessions/beta\.kitty-session$`:
				return []byte(`[]`), nil
			default:
				t.Fatalf("unexpected match expr: %q", args[3])
				return nil, nil
			}
		},
	}

	got, err := listKittyActiveSessions(d)
	if err != nil {
		t.Fatalf("listKittyActiveSessions returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("sessions = %#v", got)
	}
	if got[0].Name != "alpha" || got[0].TabCount != 2 {
		t.Fatalf("session = %#v", got[0])
	}
	if got[0].Path != "/Users/test/.local/share/kitty/sessions/alpha.kitty-session" {
		t.Fatalf("path = %q", got[0].Path)
	}
}

func TestParseKittySessionSelection(t *testing.T) {
	got, err := parseKittySessionSelection("/tmp/proj.kitty-session\tproj\t2 tabs")
	if err != nil {
		t.Fatalf("parseKittySessionSelection returned error: %v", err)
	}
	if got != "/tmp/proj.kitty-session" {
		t.Fatalf("path = %q", got)
	}
}

func TestParseKittySessionSelectionRejectsInvalidLine(t *testing.T) {
	_, err := parseKittySessionSelection("proj only")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunKittyListOSWindows(t *testing.T) {
	out := &strings.Builder{}
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[
				{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
			]`), nil
		},
		stdout: out,
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"list-os-windows"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if got := out.String(); got != "\x1b[1;34m7: shell\x1b[m\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestRunKittyGotoOSWindowWithExplicitID(t *testing.T) {
	var commands []string
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
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
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"goto-os-window", "7"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(commands, "\n") != "kitty @ ls\nkitten @ focus-tab --match id:70" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestRunKittyTargetsRoutesToDependency(t *testing.T) {
	var got []string
	d := deps{
		runKittyTargets: func(args []string) error {
			got = append([]string{}, args...)
			return nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"targets", "--overlay", "--target", "17"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(got, " ") != "--overlay --target 17" {
		t.Fatalf("kitty targets called with %v", got)
	}
}

func TestRunKittyNewSessionLaunchesOverlay(t *testing.T) {
	var commands []string
	d := deps{
		executablePath: func() (string, error) { return "/tmp/blf", nil },
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		stdin:  strings.NewReader(""),
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	if err := runKitty([]string{"new-session"}, d); err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}

	want := "kitty @ launch --type=overlay --copy-env --cwd=current -- /tmp/blf kitty new-session --overlay"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestRunKittyNewSessionOverlayCreatesAndSwitches(t *testing.T) {
	var commands []string
	out := &strings.Builder{}
	d := deps{
		stdin:       strings.NewReader("proj\n"),
		stdout:      out,
		stderr:      &strings.Builder{},
		userHomeDir: func() (string, error) { return "/Users/test", nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		fileExists:  func(string) (bool, error) { return false, nil },
		getwd:       func() (string, error) { return "/work", nil },
		writeFile:   func(string, []byte, os.FileMode) error { return nil },
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
	}

	if err := runKitty([]string{"new-session", "--overlay"}, d); err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}

	if got := out.String(); got != "Session name: " {
		t.Fatalf("prompt = %q", got)
	}
	want := "kitten @ action goto_session /Users/test/.local/share/kitty/sessions/proj.kitty-session"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestRunKittySessionsLaunchesOverlay(t *testing.T) {
	var commands []string
	d := deps{
		executablePath: func() (string, error) { return "/tmp/blf", nil },
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		stdin:  strings.NewReader(""),
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	if err := runKitty([]string{"sessions"}, d); err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}

	want := "kitty @ launch --type=overlay --copy-env --cwd=current -- /tmp/blf kitty sessions --overlay"
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestRunKittySessionsOverlayShowsErrorWhenNoActiveSessions(t *testing.T) {
	var commands []string
	d := deps{
		userHomeDir: func() (string, error) { return "/Users/test", nil },
		readDir:     func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist },
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return []byte{}, nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	if err := runKitty([]string{"sessions", "--overlay"}, d); err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}

	want := `kitten @ action show_error "blf kitty sessions" "No active kitty sessions"`
	if strings.Join(commands, "\n") != want {
		t.Fatalf("commands = %v", commands)
	}
}

func TestRunKittyGotoOSWindowRejectsInvalidID(t *testing.T) {
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name == "kitty" && strings.Join(args, " ") == "@ ls" {
				return []byte(`[]`), nil
			}
			t.Fatalf("unexpected command: %s %v", name, args)
			return nil, nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"goto-os-window", "abc"}, d)
	if err == nil || !strings.Contains(err.Error(), `invalid kitty os window id "abc"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestRunKittyGotoOSWindowShowsErrorWhenNoOtherWindows(t *testing.T) {
	var commands []string
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ ls":
				return []byte(`[
					{"id":7,"is_active":true,"last_focused":false,"tabs":[{"id":70,"is_active":true,"is_focused":true,"title":"shell"}]}
				]`), nil
			case name == "kitten" && strings.Join(args, " ") == `@ action show_error "blf kitty" "No other kitty windows"`:
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"goto-os-window"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(commands, "\n") != "kitty @ ls\nkitten @ action show_error \"blf kitty\" \"No other kitty windows\"" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestActiveKittyTabIDForOSWindowPrefersFocusedThenActive(t *testing.T) {
	windows := []kittyOSWindow{
		{
			ID: 5,
			Tabs: []kittyTab{
				{ID: 50, IsActive: true, Title: "active"},
				{ID: 51, IsFocused: true, Title: "focused"},
			},
		},
	}

	got, err := activeKittyTabIDForOSWindow(windows, "5")
	if err != nil {
		t.Fatalf("activeKittyTabIDForOSWindow returned error: %v", err)
	}
	if got != "51" {
		t.Fatalf("tab id = %q", got)
	}
}

func TestActiveKittyTabIDForOSWindowErrorsWhenMissing(t *testing.T) {
	_, err := activeKittyTabIDForOSWindow([]kittyOSWindow{}, "9")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestListKittyOSWindowsWrapsCommandErrors(t *testing.T) {
	d := deps{
		runCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"list-os-windows"}, d)
	if err == nil || !strings.Contains(err.Error(), "run `kitty @ ls`") {
		t.Fatalf("error = %v", err)
	}
}

func TestKittySessionHelpers(t *testing.T) {
	if !isKittySessionFilename("proj.kitty_session") {
		t.Fatal("expected kitty session filename to be recognized")
	}
	if trimKittySessionExtension("proj.session") != "proj" {
		t.Fatalf("trimmed = %q", trimKittySessionExtension("proj.session"))
	}
	if got := formatKittySessionChoices([]kittySession{{Path: filepath.Join("/tmp", "proj.kitty-session"), Name: "proj", TabCount: 1}}); got != "/tmp/proj.kitty-session\tproj\t1 tab\n" {
		t.Fatalf("choices = %q", got)
	}
}
