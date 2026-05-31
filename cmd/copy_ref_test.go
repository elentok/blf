package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- buildCopyRefCommand (pure, OS-parameterized) ---

func TestBuildCopyRefCommandDarwinSingle(t *testing.T) {
	name, args, err := buildCopyRefCommand("darwin", []string{"/a/x.png"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if name != "osascript" {
		t.Fatalf("name = %q, want osascript", name)
	}
	want := `set the clipboard to POSIX file "/a/x.png"`
	if len(args) != 2 || args[0] != "-e" || args[1] != want {
		t.Fatalf("args = %#v, want [-e %q]", args, want)
	}
}

func TestBuildCopyRefCommandDarwinMultiple(t *testing.T) {
	_, args, err := buildCopyRefCommand("darwin", []string{"/a/x.png", "/a/y.png"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := `set the clipboard to {POSIX file "/a/x.png", POSIX file "/a/y.png"}`
	if args[1] != want {
		t.Fatalf("script = %q, want %q", args[1], want)
	}
}

func TestBuildCopyRefCommandDarwinEscaping(t *testing.T) {
	_, args, err := buildCopyRefCommand("darwin", []string{`/tmp/a "b" \c.png`})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := `set the clipboard to POSIX file "/tmp/a \"b\" \\c.png"`
	if args[1] != want {
		t.Fatalf("script = %q, want %q", args[1], want)
	}
}

func TestBuildCopyRefCommandLinuxSingle(t *testing.T) {
	name, args, err := buildCopyRefCommand("linux", []string{"/a/x.png"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if name != "wl-copy" {
		t.Fatalf("name = %q, want wl-copy", name)
	}
	want := []string{"--type", "text/uri-list", "file:///a/x.png"}
	if len(args) != 3 || args[0] != want[0] || args[1] != want[1] || args[2] != want[2] {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestBuildCopyRefCommandLinuxMultiple(t *testing.T) {
	_, args, err := buildCopyRefCommand("linux", []string{"/a/x.png", "/a/y.png"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := "file:///a/x.png\nfile:///a/y.png"
	if args[2] != want {
		t.Fatalf("payload = %q, want %q", args[2], want)
	}
}

func TestBuildCopyRefCommandLinuxEncoding(t *testing.T) {
	_, args, err := buildCopyRefCommand("linux", []string{"/a/b c.png"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	want := "file:///a/b%20c.png"
	if args[2] != want {
		t.Fatalf("uri = %q, want %q", args[2], want)
	}
}

func TestBuildCopyRefCommandUnsupportedOS(t *testing.T) {
	_, _, err := buildCopyRefCommand("windows", []string{"/a/x.png"})
	if err == nil {
		t.Fatal("expected error for unsupported OS")
	}
	if !strings.Contains(err.Error(), "unsupported on this OS (windows)") {
		t.Fatalf("error = %q, want it to mention unsupported OS", err.Error())
	}
}

// --- expandTilde ---

func TestExpandTilde(t *testing.T) {
	home := func() (string, error) { return "/home/test", nil }
	cases := []struct {
		in   string
		want string
	}{
		{"~", "/home/test"},
		{"~/sub/file", "/home/test/sub/file"},
		{"~user/x", "~user/x"},
		{"/a/~/b", "/a/~/b"},
		{"rel/path", "rel/path"},
		{"/abs/path", "/abs/path"},
	}
	for _, c := range cases {
		got, err := expandTilde(c.in, home)
		if err != nil {
			t.Fatalf("expandTilde(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("expandTilde(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- copyRefForOS (orchestrator) ---

func TestCopyRefForOSNoArgs(t *testing.T) {
	called := false
	d := deps{
		stdout:      &strings.Builder{},
		userHomeDir: func() (string, error) { return "/home/test", nil },
		lookPath:    func(string) (string, error) { return "", nil },
		runCommand: func(string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
	}
	if err := copyRefForOS(nil, d, "darwin"); err == nil {
		t.Fatal("expected usage error for no args")
	}
	if called {
		t.Fatal("runCommand must not be called when there are no args")
	}
}

func TestCopyRefForOSMissingFile(t *testing.T) {
	called := false
	d := deps{
		stdout:      &strings.Builder{},
		userHomeDir: func() (string, error) { return "/home/test", nil },
		lookPath:    func(string) (string, error) { return "", nil },
		runCommand: func(string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
	}
	missing := filepath.Join(t.TempDir(), "nope.png")
	err := copyRefForOS([]string{missing}, d, "darwin")
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("error = %v, want file not found", err)
	}
	if called {
		t.Fatal("runCommand must not be called when a file is missing")
	}
}

func TestCopyRefForOSAtomicOneMissing(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "a.png")
	if err := os.WriteFile(exists, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "b.png")

	called := false
	d := deps{
		stdout:      &strings.Builder{},
		userHomeDir: func() (string, error) { return "/home/test", nil },
		lookPath:    func(string) (string, error) { return "", nil },
		runCommand: func(string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
	}
	if err := copyRefForOS([]string{exists, missing}, d, "darwin"); err == nil {
		t.Fatal("expected error when one of several files is missing")
	}
	if called {
		t.Fatal("runCommand must not be called: validation is all-or-nothing")
	}
}

func TestCopyRefForOSLinuxToolMissing(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.png")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	called := false
	d := deps{
		stdout:      &strings.Builder{},
		userHomeDir: func() (string, error) { return "/home/test", nil },
		lookPath:    func(string) (string, error) { return "", fmt.Errorf("not found") },
		runCommand: func(string, ...string) ([]byte, error) {
			called = true
			return nil, nil
		},
	}
	err := copyRefForOS([]string{f}, d, "linux")
	if err == nil || !strings.Contains(err.Error(), "install wl-clipboard") {
		t.Fatalf("error = %v, want install hint", err)
	}
	if called {
		t.Fatal("runCommand must not be called when wl-copy is missing")
	}
}

func TestCopyRefForOSSuccessSingle(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.png")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotName string
	var gotArgs []string
	out := &strings.Builder{}
	d := deps{
		stdout:      out,
		userHomeDir: func() (string, error) { return "/home/test", nil },
		lookPath:    func(string) (string, error) { return "", nil },
		runCommand: func(name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = args
			return nil, nil
		},
	}
	if err := copyRefForOS([]string{f}, d, "darwin"); err != nil {
		t.Fatalf("error: %v", err)
	}
	if gotName != "osascript" {
		t.Fatalf("name = %q, want osascript", gotName)
	}
	wantScript := fmt.Sprintf(`set the clipboard to POSIX file "%s"`, f)
	if len(gotArgs) != 2 || gotArgs[1] != wantScript {
		t.Fatalf("args = %#v, want script %q", gotArgs, wantScript)
	}
	if out.String() != "copied 1 file reference to clipboard\n" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestCopyRefForOSSuccessMultiplePlural(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	for _, f := range []string{a, b} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := &strings.Builder{}
	d := deps{
		stdout:      out,
		userHomeDir: func() (string, error) { return "/home/test", nil },
		lookPath:    func(string) (string, error) { return "", nil },
		runCommand:  func(string, ...string) ([]byte, error) { return nil, nil },
	}
	if err := copyRefForOS([]string{a, b}, d, "darwin"); err != nil {
		t.Fatalf("error: %v", err)
	}
	if out.String() != "copied 2 file references to clipboard\n" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestCopyRefForOSExpandsTilde(t *testing.T) {
	home := t.TempDir()
	f := filepath.Join(home, "a.png")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotArgs []string
	d := deps{
		stdout:      &strings.Builder{},
		userHomeDir: func() (string, error) { return home, nil },
		lookPath:    func(string) (string, error) { return "", nil },
		runCommand: func(_ string, args ...string) ([]byte, error) {
			gotArgs = args
			return nil, nil
		},
	}
	if err := copyRefForOS([]string{"~/a.png"}, d, "darwin"); err != nil {
		t.Fatalf("error: %v", err)
	}
	wantScript := fmt.Sprintf(`set the clipboard to POSIX file "%s"`, f)
	if len(gotArgs) != 2 || gotArgs[1] != wantScript {
		t.Fatalf("args = %#v, want tilde expanded to %q", gotArgs, f)
	}
}
