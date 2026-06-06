package claude

import "charm.land/lipgloss/v2"

var (
	modelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	faintStyle   = lipgloss.NewStyle().Faint(true)
	plainStyle   = lipgloss.NewStyle()
	separator    = faintStyle.Render("·")
	leftBracket  = faintStyle.Render("[")
	rightBracket = faintStyle.Render("]")
)
