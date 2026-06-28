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
// When onlyIfWorking is true the write is applied only if the window's current
// AGENT_STATE is "working"; otherwise it is a silent no-op. This guards the
// Notification → waiting hook: Claude Code fires Notification both when it needs
// input (real waiting) AND as a ~60s idle nag, so an already-idle agent must not
// be flipped back to waiting. A real permission/question only arrives while the
// agent is working, so gating on "working" keeps those while ignoring the nag.
//
// CRITICAL: nothing is ever written to stdout. This command is invoked from a
// Claude Code UserPromptSubmit hook whose stdout is injected into the model's
// context, so any output would silently pollute every prompt. Success is mute;
// errors surface via a returned error (stderr / non-zero exit).
func SetAgentState(state string, onlyIfWorking bool, d Deps) error {
	if !validAgentStates[state] {
		return fmt.Errorf("invalid agent state %q (want working|waiting|idle)", state)
	}

	id, ok := d.LookupEnv("KITTY_WINDOW_ID")
	if !ok || id == "" {
		return nil
	}

	if onlyIfWorking {
		current, err := currentAgentState(id, d)
		if err != nil {
			return err
		}
		if current != "working" {
			return nil
		}
	}

	if _, err := d.RunCommand("kitty", "@", "set-user-vars", "AGENT_STATE="+state); err != nil {
		return fmt.Errorf("set agent state: %w", err)
	}

	return nil
}

// currentAgentState reads the AGENT_STATE user var of the window with the given
// id via `kitty @ ls --match id:<id>`. A missing/unset var reads as "".
func currentAgentState(id string, d Deps) (string, error) {
	if d.RunCommand == nil {
		return "", fmt.Errorf("kitty command runner is not configured")
	}

	out, err := d.RunCommand("kitty", "@", "ls", "--match", "id:"+id)
	if err != nil {
		return "", fmt.Errorf("read agent state: %w", err)
	}

	windows, err := ParseOSWindows(out)
	if err != nil {
		return "", fmt.Errorf("read agent state: %w", err)
	}

	for _, osWindow := range windows {
		for _, tab := range osWindow.Tabs {
			for _, window := range tab.Windows {
				return window.UserVars[agentStateUserVar], nil
			}
		}
	}

	return "", nil
}
