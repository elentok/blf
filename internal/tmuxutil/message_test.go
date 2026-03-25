package tmuxutil

import "testing"

func TestDisplayToolMessageDoesNotDuplicatePrefix(t *testing.T) {
	t.Setenv("TMUX", "1")

	var got []string
	run := func(name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}

	DisplayToolMessage(run, "tmux-targets", "blf tmux-targets: already prefixed")

	if len(got) != 5 {
		t.Fatalf("unexpected args length: %#v", got)
	}
	if got[4] != "blf tmux-targets: already prefixed" {
		t.Fatalf("unexpected message: %q", got[4])
	}
}

func TestDisplayMessageSkipsOutsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")

	called := false
	run := func(name string, args ...string) error {
		called = true
		return nil
	}

	DisplayMessage(run, "hello")
	if called {
		t.Fatal("expected no call outside tmux")
	}
}
