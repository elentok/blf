package fuzzyfinder

import "strings"

// Item is the common row-content shape shared by finders: an optional icon, a
// title (with fuzzy-match characters highlighted), and an optional subtitle —
// rendered as "{icon}{title} {subtitle}". The active-row gutter is owned by the
// widget, so RenderItem must not include it.
type Item struct {
	// Icon is rendered verbatim before the title; include any trailing space.
	Icon string
	// Title is the primary label.
	Title string
	// Subtitle is dimmed trailing context; omitted when empty.
	Subtitle string
	// MatchRanges are rune indices in Title to highlight as fuzzy matches.
	MatchRanges []int
}

// RenderItem renders an Item to a single line of content (no gutter, no
// trailing newline) using the shared finder styles.
func RenderItem(it Item) string {
	line := it.Icon + highlightTitle(it.Title, it.MatchRanges)
	if it.Subtitle != "" {
		line += " " + SubtitleStyle.Render(it.Subtitle)
	}
	return line
}

// highlightTitle styles the rune positions in MatchRanges with HighlightStyle.
func highlightTitle(title string, ranges []int) string {
	if len(ranges) == 0 {
		return title
	}
	pos := make(map[int]bool, len(ranges))
	for _, p := range ranges {
		pos[p] = true
	}
	var sb strings.Builder
	for i, ch := range []rune(title) {
		if pos[i] {
			sb.WriteString(HighlightStyle.Render(string(ch)))
		} else {
			sb.WriteString(string(ch))
		}
	}
	return sb.String()
}
