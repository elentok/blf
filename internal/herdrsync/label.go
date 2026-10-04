// Package herdrsync computes the tab label herdr's event-mode sync writes
// for a tab's active pane.
package herdrsync

import (
	"strings"

	"github.com/elentok/blf/internal/herdr"
	"github.com/elentok/blf/internal/tabtitle"
)

// Label returns the tab label for pane, the active pane of its tab. home is
// used for the Title "~" rule. The result is never cut a second time after
// the agent/dir fallback is built.
func Label(pane herdr.PaneInfo, home string) string {
	if pane.Agent != "" {
		return agentLabel(pane, home)
	}
	return shellLabel(pane, home)
}

func agentLabel(pane herdr.PaneInfo, home string) string {
	if stripped := stripSpinner(pane.TerminalTitleStripped); stripped != "" {
		return tabtitle.Ellipsis(stripped, 25)
	}

	dir := pane.ForegroundCwd
	if dir == "" {
		dir = pane.Cwd
	}
	return pane.Agent + ": " + tabtitle.Title("", dir, home)
}

func shellLabel(pane herdr.PaneInfo, home string) string {
	if pane.TerminalTitleStripped != "" {
		return tabtitle.Ellipsis(pane.TerminalTitleStripped, 40)
	}
	return tabtitle.Title("", pane.Cwd, home)
}

// stripSpinner removes leading braille spinner frames (U+2800-U+28FF), "✳"
// and spaces from s, so a frozen spinner frame never lies about status.
func stripSpinner(s string) string {
	return strings.TrimLeftFunc(s, func(r rune) bool {
		return (r >= '⠀' && r <= '⣿') || r == '✳' || r == ' '
	})
}
