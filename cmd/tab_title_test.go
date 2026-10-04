package cmd

import (
	"strings"
	"testing"
)

func newTabTitleDeps(out *strings.Builder, vimruntime string, pwd string, home string) deps {
	return deps{
		stdout: out,
		lookupEnv: func(key string) (string, bool) {
			if key == "VIMRUNTIME" {
				if vimruntime == "" {
					return "", false
				}
				return vimruntime, true
			}
			return "", false
		},
		getwd:       func() (string, error) { return pwd, nil },
		userHomeDir: func() (string, error) { return home, nil },
	}
}

func TestRunTabTitleVimruntimeNonEmptyPrintsNothing(t *testing.T) {
	out := &strings.Builder{}
	d := newTabTitleDeps(out, "/usr/share/nvim/runtime", "/home/david", "/home/david")

	if err := runTabTitle([]string{"ls"}, d); err != nil {
		t.Fatalf("runTabTitle returned error: %v", err)
	}

	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestRunTabTitleVimruntimeEmptyDoesNotTriggerGuard(t *testing.T) {
	out := &strings.Builder{}
	d := newTabTitleDeps(out, "", "/home/david", "/home/david")

	if err := runTabTitle([]string{"ls"}, d); err != nil {
		t.Fatalf("runTabTitle returned error: %v", err)
	}

	if out.String() == "" {
		t.Errorf("stdout = %q, want non-empty", out.String())
	}
}

func TestRunTabTitleNoArgumentPrintsDirectoryForm(t *testing.T) {
	out := &strings.Builder{}
	d := newTabTitleDeps(out, "", "/home/david/projects/blf", "/home/david")

	if err := runTabTitle(nil, d); err != nil {
		t.Fatalf("runTabTitle returned error: %v", err)
	}

	want := "blf\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
}
