package claudehistory

import "charm.land/lipgloss/v2"

var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))

var previewStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#cdd6f4")).
	Border(lipgloss.NormalBorder(), true, false, false, false).
	BorderForeground(lipgloss.Color("#585b70"))
