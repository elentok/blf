package beads

import "charm.land/lipgloss/v2"

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

var emptyStateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true)

// Readiness row-indicator colors, distinct from the plain status icon.
var (
	readinessUnblockedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")) // green
	readinessBlockedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")) // red
	readinessOtherStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")) // dim
)
