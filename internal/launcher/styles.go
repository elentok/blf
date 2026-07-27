package launcher

import "charm.land/lipgloss/v2"

// Catppuccin Mocha palette (same family as internal/targets/styles.go)
const (
	colorBase     = "#1e1e2e"
	colorSurface0 = "#313244"
	colorSurface1 = "#45475a"
	colorText     = "#cdd6f4"
	colorSubtext  = "#a6adc8"
	colorOverlay0 = "#6c7086" // dimmer than subtext, used for subtitles
	colorLavender = "#b4befe"
	colorBlue     = "#89b4fa"
	colorSapphire = "#74c7ec"
	colorGreen    = "#a6e3a1"
	colorYellow   = "#f9e2af"
	colorPeach    = "#fab387"
	colorRed      = "#f38ba8"
	colorBorderFg = "#585b70" // surface2
)

var (
	// Outer border wrapping the full launcher
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorderFg)).
			Padding(0, 1)

	// noBorderStyle is used instead of borderStyle for the help overlay when
	// the frame border is disabled; it keeps the same padding.
	noBorderStyle = lipgloss.NewStyle().Padding(0, 1)

	// Input prompt
	inputPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorLavender))

	// Separator line between input and results
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSurface1))

	// Result-row content styling now lives in internal/fuzzyfinder (RenderItem);
	// the launcher only supplies icons and the raw result fields.

	// Help / status footer
	helpBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorLavender)).Background(lipgloss.Color(colorSurface0))
	helpTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPeach))
	helpHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen))
	helpKeyStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorLavender))
	helpDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext))

	// Error notice (non-blocking config error)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))
)
