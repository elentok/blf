package kitty

import (
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

func TestFormatSessionFile(t *testing.T) {
	got := formatSessionFile("proj", "/tmp/work tree")
	want := "new_tab \"proj\"\ncd \"/tmp/work tree\"\nlaunch\n"
	if got != want {
		t.Fatalf("session file = %q, want %q", got, want)
	}
}

func TestPromptSessionNameRejectsSeparators(t *testing.T) {
	_, err := promptSessionName(strings.NewReader("a/b\n"), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "cannot contain path separators") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateSessionFile(t *testing.T) {
	var (
		gotDir     string
		gotPath    string
		gotPerm    os.FileMode
		gotContent string
	)
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		MkdirAll: func(path string, perm os.FileMode) error {
			gotDir = path
			if perm != 0o755 {
				t.Fatalf("mkdir perm = %v", perm)
			}
			return nil
		},
		FileExists: func(path string) (bool, error) {
			if path != "/Users/test/.local/share/kitty/sessions/proj.kitty-session" {
				t.Fatalf("exists path = %q", path)
			}
			return false, nil
		},
		Getwd: func() (string, error) { return "/work tree", nil },
		WriteFile: func(path string, data []byte, perm os.FileMode) error {
			gotPath = path
			gotPerm = perm
			gotContent = string(data)
			return nil
		},
	}

	got, err := createSessionFile("proj", d)
	if err != nil {
		t.Fatalf("createSessionFile returned error: %v", err)
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

func TestListActiveSessions(t *testing.T) {
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		ReadDir: func(path string) ([]os.DirEntry, error) {
			if path != "/Users/test/.local/share/kitty/sessions" {
				t.Fatalf("ReadDir path = %q", path)
			}
			return []os.DirEntry{
				fakeDirEntry{name: "beta.kitty-session"},
				fakeDirEntry{name: "alpha.kitty-session"},
				fakeDirEntry{name: "notes.txt"},
				fakeDirEntry{name: "nested", isDir: true},
			}, nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
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

	got, err := ListActiveSessions(d)
	if err != nil {
		t.Fatalf("ListActiveSessions returned error: %v", err)
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

func TestSessionHelpers(t *testing.T) {
	if !isSessionFilename("proj.kitty_session") {
		t.Fatal("expected kitty session filename to be recognized")
	}
	if trimSessionExtension("proj.session") != "proj" {
		t.Fatalf("trimmed = %q", trimSessionExtension("proj.session"))
	}
	if got := formatSessionChoices([]Session{{Path: filepath.Join("/tmp", "proj.kitty-session"), Name: "proj", TabCount: 1}}); got != "/tmp/proj.kitty-session\tproj\t1 tab\n" {
		t.Fatalf("choices = %q", got)
	}
}
