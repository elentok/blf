package fuzzyfinder

import (
	"strings"

	"charm.land/lipgloss/v2"
)

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
	// Selected applies SelectedBg to every styled piece so the row background
	// is continuous. Set this when the item is the active row.
	Selected bool
}

// RenderItem renders an Item to a single line of content (no gutter, no
// trailing newline) using the shared finder styles.
func RenderItem(it Item) string {
	icon := it.Icon
	if it.Selected && icon != "" {
		icon = lipgloss.NewStyle().Background(SelectedBg).Render(icon)
	}

	line := icon + Highlight(it.Title, it.MatchRanges, lipgloss.NewStyle(), it.Selected)

	if it.Subtitle != "" {
		s := SubtitleStyle
		if it.Selected {
			s = s.Background(SelectedBg)
		}
		sep := " "
		if it.Selected {
			sep = lipgloss.NewStyle().Background(SelectedBg).Render(" ")
		}
		line += sep + s.Render(it.Subtitle)
	}
	return line
}

// Highlight renders text with the runes at the given local rune indices styled
// as fuzzy matches (HighlightStyle layered over base); all other runes use base.
// When selected is true, base (and therefore the gaps between matches) also gets
// SelectedBg so the active-row background stays continuous. ranges are 0-based
// rune indices into text; pass nil for no highlights.
//
// This is the shared highlight primitive: any finder that renders a styled field
// alongside fuzzy-match positions should route it through here so highlighting,
// match coloring, and selection background behave identically everywhere.
func Highlight(text string, ranges []int, base lipgloss.Style, selected bool) string {
	hlStyle := HighlightStyle
	if selected {
		base = base.Background(SelectedBg)
		hlStyle = hlStyle.Background(SelectedBg)
	}

	if len(ranges) == 0 {
		return base.Render(text)
	}

	pos := make(map[int]bool, len(ranges))
	for _, p := range ranges {
		pos[p] = true
	}

	var sb strings.Builder
	for i, ch := range []rune(text) {
		s := string(ch)
		if pos[i] {
			sb.WriteString(hlStyle.Render(s))
		} else {
			sb.WriteString(base.Render(s))
		}
	}
	return sb.String()
}
