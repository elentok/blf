package kitty

import (
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

			if err := SetAgentState(state, d); err != nil {
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

	if err := SetAgentState("bogus", d); err == nil {
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

			if err := SetAgentState("working", d); err != nil {
				t.Fatalf("SetAgentState returned error: %v", err)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}
