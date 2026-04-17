package kitty

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

type doctorDirEntry struct {
	name string
}

func (d doctorDirEntry) Name() string               { return d.name }
func (d doctorDirEntry) IsDir() bool                { return false }
func (d doctorDirEntry) Type() fs.FileMode          { return 0 }
func (d doctorDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestDoctorPrintsSessionDiagnostics(t *testing.T) {
	out := &strings.Builder{}
	d := Deps{
		Stdout: out,
		LookupEnv: func(key string) (string, bool) {
			switch key {
			case "KITTY_WINDOW_ID":
				return "17", true
			default:
				return "", false
			}
		},
		UserHomeDir: func() (string, error) { return "/Users/test", nil },
		ReadDir: func(path string) ([]os.DirEntry, error) {
			return []os.DirEntry{
				doctorDirEntry{name: "alpha.kitty-session"},
			}, nil
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			joined := name + " " + strings.Join(args, " ")
			switch joined {
			case "kitty --version":
				return []byte("kitty 0.46.2\n"), nil
			case "kitty @ ls":
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"shell"}]}]`), nil
			case "kitty @ ls --match-tab session:.":
				return []byte(`[]`), nil
			case "kitty @ ls --match-tab session:~":
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"shell"}]}]`), nil
			case "kitty @ ls --match-tab session:^$":
				return []byte(`[]`), nil
			case `kitty @ ls --match-tab session:^alpha$`:
				return []byte(`[{"id":1,"tabs":[{"id":10,"title":"shell"},{"id":11,"title":"logs"}]}]`), nil
			default:
				t.Fatalf("unexpected command: %s", joined)
				return nil, nil
			}
		},
	}

	if err := Doctor(nil, d); err != nil {
		t.Fatalf("Doctor returned error: %v", err)
	}

	got := out.String()
	for _, snippet := range []string{
		"blf kitty doctor",
		"[environment]",
		"KITTY_WINDOW_ID=17",
		"[kitty binary]",
		"kitty --version: kitty 0.46.2",
		"[kitty ls]",
		"session:~ -> tabs=1",
		"[session dir]",
		"- alpha.kitty-session",
		"stem=alpha",
		"session:^alpha$ -> tabs=2",
		"[active sessions]",
		"- alpha (2 tabs) -> /Users/test/.local/share/kitty/sessions/alpha.kitty-session",
	} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("doctor output missing %q:\n%s", snippet, got)
		}
	}
}

func TestDoctorRejectsArgs(t *testing.T) {
	err := Doctor([]string{"extra"}, Deps{Stdout: &strings.Builder{}})
	if err == nil || !strings.Contains(err.Error(), "usage: blf kitty doctor") {
		t.Fatalf("error = %v", err)
	}
}
