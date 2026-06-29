package fuzzyfinder

import "charm.land/lipgloss/v2"

// Catppuccin Mocha palette — mirrors internal/launcher/styles.go.
const (
	colorSurface0 = "#313244"
	colorSurface1 = "#45475a"
	colorText     = "#cdd6f4"
	colorLavender = "#b4befe"
	colorPeach    = "#fab387"
	colorBorderFg = "#585b70"
)

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorderFg)).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorLavender))

	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSurface1))

	RowNormalStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
	RowSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorText)).
				Background(lipgloss.Color(colorSurface1))
	HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPeach)).Bold(true)
	HighlightSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPeach)).
				Background(lipgloss.Color(colorSurface1)).
				Bold(true)

	helpBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorLavender)).
			Background(lipgloss.Color(colorSurface0))
)
