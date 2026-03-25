package tmuxtargets

import "charm.land/lipgloss/v2"

const (
	paletteBase  = "#1e1e2e"
	paletteText  = "#a6adc8"
	palettePeach = "#fab387"
	paletteGreen = "#a6e3a1"
)

var (
	baseStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color(paletteText))
	targetStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color(palettePeach))
	selectedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(paletteBase)).Background(lipgloss.Color(palettePeach))
	searchTargetStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(paletteBase)).Background(lipgloss.Color(paletteGreen))
	searchSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(paletteBase)).Background(lipgloss.Color(paletteGreen))
	searchBoxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(paletteGreen)).Foreground(lipgloss.Color(paletteGreen)).Width(38)
)
