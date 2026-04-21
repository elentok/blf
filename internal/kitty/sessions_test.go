package kitty

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

func TestFormatSessionFile(t *testing.T) {
	got := formatSessionFile("proj", "/tmp/work tree")
	want := "new_tab\ncd \"/tmp/work tree\"\nlaunch\n"
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
	wantContent := "new_tab\ncd \"/work tree\"\nlaunch\n"
	if gotContent != wantContent {
		t.Fatalf("content = %q, want %q", gotContent, wantContent)
	}
}

func TestListSessions(t *testing.T) {
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
	}

	got, err := ListSessions(d)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("sessions = %#v", got)
	}
	if got[0].Name != "alpha" || got[0].TabCount != 0 {
		t.Fatalf("first session = %#v", got[0])
	}
	if got[0].Path != "/Users/test/.local/share/kitty/sessions/alpha.kitty-session" {
		t.Fatalf("path = %q", got[0].Path)
	}
	if got[1].Name != "beta" || got[1].TabCount != 0 {
		t.Fatalf("second session = %#v", got[1])
	}
}

func TestListActiveSessionsFiltersZeroTabSessions(t *testing.T) {
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		ReadDir: func(path string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				fakeDirEntry{name: "beta.kitty-session"},
				fakeDirEntry{name: "alpha.kitty-session"},
			}, nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			switch args[3] {
			case `session:^alpha$`:
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"one"}]}]`), nil
			case `session:^beta$`:
				return nil, errors.New("exit status 1")
			default:
				t.Fatalf("unexpected args: %v", args)
			}
			return nil, nil
		},
	}

	got, err := ListActiveSessions(d)
	if err != nil {
		t.Fatalf("ListActiveSessions returned error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "alpha" {
		t.Fatalf("sessions = %#v", got)
	}
}

func TestCreateSessionFileReturnsExistingActiveSessionPath(t *testing.T) {
	var writeInvoked bool
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		FileExists:  func(string) (bool, error) { return true, nil },
		Getwd:       func() (string, error) { return "/work tree", nil },
		WriteFile: func(string, []byte, os.FileMode) error {
			writeInvoked = true
			return nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			switch args[3] {
			case `session:^proj$`:
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"one"}]}]`), nil
			default:
				t.Fatalf("unexpected args: %v", args)
				return nil, nil
			}
		},
	}

	got, err := createSessionFile("proj", d)
	if err != nil {
		t.Fatalf("createSessionFile returned error: %v", err)
	}
	if got != "/Users/test/.local/share/kitty/sessions/proj.kitty-session" {
		t.Fatalf("path = %q", got)
	}
	if writeInvoked {
		t.Fatal("expected active session file to be reused without rewriting")
	}
}

func TestDeleteSessionFile(t *testing.T) {
	var removed string
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		RunCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
		RemoveFile: func(path string) error {
			removed = path
			return nil
		},
	}

	err := deleteSessionFile("/Users/test/.local/share/kitty/sessions/proj.kitty-session", d)
	if err != nil {
		t.Fatalf("deleteSessionFile returned error: %v", err)
	}
	if removed != "/Users/test/.local/share/kitty/sessions/proj.kitty-session" {
		t.Fatalf("removed = %q", removed)
	}
}

func TestDeleteSessionFileRejectsOutsidePath(t *testing.T) {
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		RunCommand:  func(string, ...string) ([]byte, error) { return nil, nil },
		RemoveFile:  func(string) error { return nil },
	}

	err := deleteSessionFile("/tmp/proj.kitty-session", d)
	if err == nil || !strings.Contains(err.Error(), "invalid kitty session path") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeleteSessionFileIgnoresMissingFile(t *testing.T) {
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		RunCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
		RemoveFile: func(string) error {
			return os.ErrNotExist
		},
	}

	if err := deleteSessionFile("/Users/test/.local/share/kitty/sessions/proj.kitty-session", d); err != nil {
		t.Fatalf("deleteSessionFile returned error: %v", err)
	}
}

func TestDeleteSessionFileRejectsActiveSession(t *testing.T) {
	var removed bool
	d := Deps{
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		RunCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls --match-tab session:^proj$" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[{"id":1,"tabs":[{"id":10,"title":"one"}]}]`), nil
		},
		RemoveFile: func(string) error {
			removed = true
			return nil
		},
	}

	err := deleteSessionFile("/Users/test/.local/share/kitty/sessions/proj.kitty-session", d)
	if err == nil || !strings.Contains(err.Error(), "cannot delete active kitty session") {
		t.Fatalf("error = %v", err)
	}
	if removed {
		t.Fatal("expected active session to skip file removal")
	}
}

func TestSessionHelpers(t *testing.T) {
	if !isSessionFilename("proj.kitty_session") {
		t.Fatal("expected kitty session filename to be recognized")
	}
	if trimSessionExtension("proj.session") != "proj" {
		t.Fatalf("trimmed = %q", trimSessionExtension("proj.session"))
	}
	session := Session{Path: filepath.Join("/tmp", "proj.kitty-session"), Name: "proj", TabCount: 1}
	if got := formatSessionLabel(session); got != "\x1b[1;97mproj\x1b[m" {
		t.Fatalf("label = %q", got)
	}
	if got := formatSessionChoices([]Session{session}); got != "/tmp/proj.kitty-session\t\x1b[1;97mproj\x1b[m\n" {
		t.Fatalf("choices = %q", got)
	}
	if got := sessionStem("/tmp/proj.kitty-session"); got != "proj" {
		t.Fatalf("stem = %q", got)
	}
	if got := sessionMatchExpr("/tmp/proj.kitty-session"); got != `session:^proj$` {
		t.Fatalf("expr = %q", got)
	}
	if got := tabSessionName(Tab{Windows: []Window{{SessionName: "proj"}}}); got != "proj" {
		t.Fatalf("tab session name = %q", got)
	}
}

func TestSessionsWithLiveSessionMetadataMarksLiveAndActiveSessions(t *testing.T) {
	sessions := []Session{
		{Name: "alpha", Path: "/tmp/alpha.kitty-session"},
		{Name: "beta", Path: "/tmp/beta.kitty-session"},
		{Name: "gamma", Path: "/tmp/gamma.kitty-session"},
		{Name: "delta", Path: "/tmp/delta.kitty-session"},
	}

	windows := []OSWindow{
		{
			ID:       1,
			IsActive: true,
			Tabs: []Tab{
				{Title: "active", IsFocused: true, Windows: []Window{{LastFocusedAt: 300.1, SessionName: "beta"}}},
				{Title: "alpha", Windows: []Window{{LastFocusedAt: 250.2, SessionName: "alpha"}}},
			},
		},
		{
			ID: 2,
			Tabs: []Tab{
				{Title: "gamma", Windows: []Window{{LastFocusedAt: 150.4, SessionName: "gamma"}, {LastFocusedAt: 275.6, SessionName: "gamma"}}},
			},
		},
	}

	got := sessionsWithLiveSessionMetadata(sessions, windows)
	if len(got) != 4 {
		t.Fatalf("session count = %d, sessions = %#v", len(got), got)
	}
	if got[0].Name != "alpha" || got[0].TabCount != 1 || got[0].LastFocusedAt != 250.2 || got[0].IsActive {
		t.Fatalf("alpha session = %#v", got[0])
	}
	if got[1].Name != "beta" || got[1].TabCount != 1 || got[1].LastFocusedAt != 300.1 || !got[1].IsActive {
		t.Fatalf("beta session = %#v", got[1])
	}
	if got[2].Name != "gamma" || got[2].TabCount != 1 || got[2].LastFocusedAt != 275.6 || got[2].IsActive {
		t.Fatalf("gamma session = %#v", got[2])
	}
	if got[3].Name != "delta" || got[3].TabCount != 0 {
		t.Fatalf("delta session = %#v", got[3])
	}
}

func TestFilterAndSortSessionsForPickerHidesActiveSessionAndSortsLiveFirst(t *testing.T) {
	sessions := []Session{
		{Name: "alpha", Path: "/tmp/alpha.kitty-session", TabCount: 1, LastFocusedAt: 250.2},
		{Name: "beta", Path: "/tmp/beta.kitty-session", TabCount: 1, LastFocusedAt: 300.1, IsActive: true},
		{Name: "gamma", Path: "/tmp/gamma.kitty-session", TabCount: 1, LastFocusedAt: 275.6},
		{Name: "delta", Path: "/tmp/delta.kitty-session"},
	}

	got := filterAndSortSessionsForPicker(sessions)
	if len(got) != 3 {
		t.Fatalf("session count = %d, sessions = %#v", len(got), got)
	}
	if got[0].Name != "gamma" || got[0].TabCount != 1 || got[0].LastFocusedAt != 275.6 {
		t.Fatalf("first session = %#v", got[0])
	}
	if got[1].Name != "alpha" || got[1].TabCount != 1 || got[1].LastFocusedAt != 250.2 {
		t.Fatalf("second session = %#v", got[1])
	}
	if got[2].Name != "delta" || got[2].TabCount != 0 {
		t.Fatalf("third session = %#v", got[2])
	}
}

func TestApplyLiveSessionMetadataHidesActiveSessionFromActiveWindowFallback(t *testing.T) {
	sessions := []Session{
		{Name: "alpha", Path: "/tmp/alpha.kitty-session"},
		{Name: "beta", Path: "/tmp/beta.kitty-session"},
	}

	windows := []OSWindow{
		{
			ID:       1,
			IsActive: true,
			Tabs: []Tab{
				{Title: "shell", Windows: []Window{{LastFocusedAt: 300, SessionName: "beta"}}},
			},
		},
		{
			ID: 2,
			Tabs: []Tab{
				{Title: "editor", Windows: []Window{{LastFocusedAt: 200, SessionName: "alpha"}}},
			},
		},
	}

	got := filterAndSortSessionsForPicker(sessionsWithLiveSessionMetadata(sessions, windows))
	if len(got) != 1 {
		t.Fatalf("session count = %d, sessions = %#v", len(got), got)
	}
	if got[0].Name != "alpha" {
		t.Fatalf("remaining session = %#v", got[0])
	}
}

func TestFormatSessionLabelDimsEmptySessions(t *testing.T) {
	live := formatSessionLabel(Session{Name: "live", TabCount: 1})
	if live != "\x1b[1;97mlive\x1b[m" {
		t.Fatalf("live label = %q", live)
	}

	empty := formatSessionLabel(Session{Name: "empty"})
	if !strings.Contains(empty, "empty") || !strings.Contains(empty, "[2;90m") {
		t.Fatalf("empty label = %q", empty)
	}
}
