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

var previewStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#cdd6f4")).
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1).
	BorderForeground(lipgloss.Color("#585b70"))

var previewTitleStyle = lipgloss.NewStyle().Bold(true)

var previewMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

var previewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa"))

var previewSectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true)
