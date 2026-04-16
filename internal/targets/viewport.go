package targets

import "strings"

// NormalizeViewportText cleans prompt glyphs and line endings before the
// viewport is split into lines for display and target detection.
func NormalizeViewportText(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.ReplaceAll(text, "", " ")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}
