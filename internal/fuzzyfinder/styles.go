package fuzzyfinder

import "charm.land/lipgloss/v2"

// Catppuccin Mocha palette — mirrors internal/launcher/styles.go.
const (
	colorSurface0 = "#313244"
	colorSurface1 = "#45475a"
	colorOverlay0 = "#6c7086" // dimmer than text, used for subtitles
	colorText     = "#cdd6f4"
	colorLavender = "#b4befe"
	colorPeach    = "#fab387"
	colorBorderFg = "#585b70"
)

// gutterMarker is the active-row indicator rendered in the left gutter of every
// finder. The gutter is two columns wide; non-selected rows render two spaces.
const gutterMarker = "❯"

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorderFg)).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorLavender))

	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSurface1))

	// gutterStyle colors the active-row marker.
	gutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorLavender)).Bold(true)

	// RowNormalStyle is the default foreground for row content.
	RowNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
	// HighlightStyle marks fuzzy-match characters within a row.
	HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorPeach)).Bold(true)
	// SubtitleStyle renders the trailing subtitle of a row.
	SubtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOverlay0)).Italic(true)

	helpBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorLavender)).
			Background(lipgloss.Color(colorSurface0))
)

// gutter returns the two-column left-gutter string for a row.
func gutter(selected bool) string {
	if selected {
		return gutterStyle.Render(gutterMarker) + " "
	}
	return "  "
}
