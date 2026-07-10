package beads

import "charm.land/lipgloss/v2"

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

var emptyStateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true)

// epicRowStyle highlights epic rows in the issue list so they read as
// epics at a glance, distinct from plain task/subtask rows.
var epicRowStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f9e2af")) // yellow

// dimRowStyle renders a non-matching ancestor row kept only for a matching
// descendant's tree context (see CONTEXT.md's "issue tree" entry).
var dimRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

var previewStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#cdd6f4")).
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1).
	BorderForeground(lipgloss.Color("#585b70"))

var previewTitleStyle = lipgloss.NewStyle().Bold(true)

var previewMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

var previewHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#89b4fa"))

var previewSectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Italic(true)
