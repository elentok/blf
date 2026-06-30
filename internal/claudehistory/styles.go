package claudehistory

import "charm.land/lipgloss/v2"

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

var previewStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#cdd6f4")).
	Border(lipgloss.RoundedBorder()).
	Padding(0, 1).
	BorderForeground(lipgloss.Color("#585b70"))
