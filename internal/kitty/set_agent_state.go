package kitty

import "fmt"

// validAgentStates are the only values AGENT_STATE may take. The vocabulary is a
// contract: external agent hooks and the reader (list-agents/goto-agent) depend
// on these exact words. See ADR 0004.
var validAgentStates = map[string]bool{
	"working": true,
	"waiting": true,
	"idle":    true,
}

// SetAgentState publishes the calling window's status as the Kitty per-window
// user var AGENT_STATE, by running `kitty @ set-user-vars AGENT_STATE=<state>`.
//
// It targets the calling window via KITTY_WINDOW_ID: when that env var is unset
// or empty we are not inside a Kitty window, so this is a silent no-op (exit 0,
// nothing run, nothing printed).
//
// CRITICAL: nothing is ever written to stdout. This command is invoked from a
// Claude Code UserPromptSubmit hook whose stdout is injected into the model's
// context, so any output would silently pollute every prompt. Success is mute;
// errors surface via a returned error (stderr / non-zero exit).
func SetAgentState(state string, d Deps) error {
	if !validAgentStates[state] {
		return fmt.Errorf("invalid agent state %q (want working|waiting|idle)", state)
	}

	if id, ok := d.LookupEnv("KITTY_WINDOW_ID"); !ok || id == "" {
		return nil
	}

	if _, err := d.RunCommand("kitty", "@", "set-user-vars", "AGENT_STATE="+state); err != nil {
		return fmt.Errorf("set agent state: %w", err)
	}

	return nil
}
