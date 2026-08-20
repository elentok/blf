// Package commands defines the launcher's built-in, hardcoded, fuzzy-matched
// commands (contrast with scripts, which are user-configurable and shell
// out). See docs/adr/0007-launcher-commands-vs-scripts.md.
package commands

import (
	tea "charm.land/bubbletea/v2"
)

// Command is a named, in-process launcher action.
type Command struct {
	Name string
	Icon string // optional nerd-font glyph; empty = use IconRoleCommand
	Run  func() tea.Cmd
}

// NewBuiltins returns the built-in commands, wired to the given run funcs.
// reload, cleanURL, ai and improve are injected rather than called directly
// because their implementations live in internal/launcher, which imports
// this package to build a CommandsProvider — this package can't import back
// without a cycle.
func NewBuiltins(reload, cleanURL, ai, improve func() tea.Cmd) []Command {
	return []Command{
		{Name: "reload", Run: reload},
		{Name: "cleanurl", Run: cleanURL},
		{Name: "ai", Run: ai},
		{Name: "improve", Run: improve},
	}
}
