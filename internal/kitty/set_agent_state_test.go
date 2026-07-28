package kitty

import (
	"errors"
	"strings"
	"testing"
)

func TestSetAgentStateValidStateIssuesCommand(t *testing.T) {
	for _, state := range []string{"working", "waiting", "idle"} {
		t.Run(state, func(t *testing.T) {
			var gotName string
			var gotArgs []string
			calls := 0
			stdout := &strings.Builder{}
			d := Deps{
				LookupEnv: func(key string) (string, bool) {
					if key != "KITTY_WINDOW_ID" {
						t.Fatalf("unexpected env lookup: %q", key)
					}
					return "42", true
				},
				RunCommand: func(name string, args ...string) ([]byte, error) {
					calls++
					gotName = name
					gotArgs = args
					return nil, nil
				},
				Stdout: stdout,
				Stderr: &strings.Builder{},
			}

			if err := SetAgentState(state, false, d); err != nil {
				t.Fatalf("SetAgentState(%q) returned error: %v", state, err)
			}
			if calls != 1 {
				t.Fatalf("RunCommand called %d times, want 1", calls)
			}
			if gotName != "kitty" {
				t.Fatalf("command name = %q, want kitty", gotName)
			}
			want := []string{"@", "set-user-vars", "AGENT_STATE=" + state}
			if strings.Join(gotArgs, " ") != strings.Join(want, " ") {
				t.Fatalf("args = %v, want %v", gotArgs, want)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestSetAgentStateInvalidStateIssuesNoCommand(t *testing.T) {
	stdout := &strings.Builder{}
	d := Deps{
		LookupEnv: func(string) (string, bool) {
			t.Fatalf("LookupEnv should not be called for invalid state")
			return "", false
		},
		RunCommand: func(string, ...string) ([]byte, error) {
			t.Fatalf("RunCommand should not be called for invalid state")
			return nil, nil
		},
		Stdout: stdout,
		Stderr: &strings.Builder{},
	}

	if err := SetAgentState("bogus", false, d); err == nil {
		t.Fatal("SetAgentState(\"bogus\") returned nil error, want non-nil")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestSetAgentStateNoWindowIDIsSilentNoop(t *testing.T) {
	for _, env := range []struct {
		name  string
		value string
		ok    bool
	}{
		{"unset", "", false},
		{"empty", "", true},
	} {
		t.Run(env.name, func(t *testing.T) {
			stdout := &strings.Builder{}
			d := Deps{
				LookupEnv: func(string) (string, bool) {
					return env.value, env.ok
				},
				RunCommand: func(string, ...string) ([]byte, error) {
					t.Fatalf("RunCommand should not be called when KITTY_WINDOW_ID is %s", env.name)
					return nil, nil
				},
				Stdout: stdout,
				Stderr: &strings.Builder{},
			}

			if err := SetAgentState("working", false, d); err != nil {
				t.Fatalf("SetAgentState returned error: %v", err)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

// onlyIfWorkingDeps wires LookupEnv/RunCommand so that an `ls --match id:42`
// returns a window whose AGENT_STATE is current, and records any
// set-user-vars write. The bool reports whether a write happened.
func onlyIfWorkingDeps(t *testing.T, current string, wrote *bool) Deps {
	t.Helper()
	return Deps{
		LookupEnv: func(key string) (string, bool) {
			if key != "KITTY_WINDOW_ID" {
				t.Fatalf("unexpected env lookup: %q", key)
			}
			return "42", true
		},
		RunCommand: func(name string, args ...string) ([]byte, error) {
			switch {
			case len(args) >= 2 && args[1] == "ls":
				return agentsLS(rawWindow{ID: 42, UserVars: map[string]string{"AGENT_STATE": current}}), nil
			case len(args) >= 2 && args[1] == "set-user-vars":
				*wrote = true
				return nil, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
		Stdout: &strings.Builder{},
		Stderr: &strings.Builder{},
	}
}

// TestSetAgentStateStaleWindowIsSilentNoop covers a multiplexer (tmux, herdr)
// pane whose KITTY_WINDOW_ID/KITTY_LISTEN_ON were inherited from a kitty
// session but is now viewed from a different terminal (e.g. ghostty): `kitty
// @` can't reach a live window and fails the same way it fails for "no
// matching window" (exit status 1). That must stay a silent no-op, not a
// surfaced error.
func TestSetAgentStateStaleWindowIsSilentNoop(t *testing.T) {
	stdout := &strings.Builder{}
	d := Deps{
		LookupEnv: func(string) (string, bool) {
			return "42", true
		},
		RunCommand: func(string, ...string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
		Stdout: stdout,
		Stderr: &strings.Builder{},
	}

	if err := SetAgentState("working", false, d); err != nil {
		t.Fatalf("SetAgentState returned error: %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestSetAgentStateOnlyIfWorkingWritesWhenWorking(t *testing.T) {
	var wrote bool
	if err := SetAgentState("waiting", true, onlyIfWorkingDeps(t, "working", &wrote)); err != nil {
		t.Fatalf("SetAgentState returned error: %v", err)
	}
	if !wrote {
		t.Fatal("expected a set-user-vars write when current state is working")
	}
}

func TestSetAgentStateOnlyIfWorkingSkipsWhenNotWorking(t *testing.T) {
	for _, current := range []string{"idle", "waiting", ""} {
		t.Run("current="+current, func(t *testing.T) {
			var wrote bool
			if err := SetAgentState("waiting", true, onlyIfWorkingDeps(t, current, &wrote)); err != nil {
				t.Fatalf("SetAgentState returned error: %v", err)
			}
			if wrote {
				t.Fatalf("expected no write when current state is %q", current)
			}
		})
	}
}
