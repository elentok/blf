package claude

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

func statusLineFromValues(model string, tokens float64, contextPercent float64, fiveHourPercent float64, weekPercent float64) string {
	return statusLineFromParts(
		tokens,
		statusField{text: model},
		statusField{text: numberFromValue(tokens, false)},
		statusField{text: contextProgressValue(contextPercent, tokens)},
		statusField{text: numberFromValue(fiveHourPercent, true)},
		statusField{text: numberFromValue(weekPercent, true)},
	)
}

func statusLineFromParts(rawTokens float64, model, tokens, ctx, five, week statusField) string {
	parts := make([]string, 0, 5)
	if model.text != "" {
		parts = append(parts, styledValue(model, modelStyle))
	}
	if tokens.text != "" {
		icon := tokenIcon(rawTokens)
		tokensSegment := statusField{text: icon + " " + tokens.text, invalid: tokens.invalid}
		dynamicStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tokenColor(rawTokens)))
		parts = append(parts, styledValue(tokensSegment, dynamicStyle))
	}

	usageParts := make([]string, 0, 3)
	if ctx.text != "" {
		usageParts = append(usageParts, styledValue(ctx, plainStyle))
	}
	if five.text != "" {
		usageParts = append(usageParts, styledValue(
			statusField{text: five.text + " of 5h", invalid: five.invalid}, plainStyle,
		))
	}
	if week.text != "" {
		usageParts = append(usageParts, styledValue(
			statusField{text: week.text + " of weekly", invalid: week.invalid}, faintStyle,
		))
	}
	if len(usageParts) > 0 {
		parts = append(parts, strings.Join(usageParts, " "+separator+" "))
	}
	return strings.Join(parts, " "+separator+" ")
}

func styledValue(value statusField, style lipgloss.Style) string {
	if value.invalid {
		return errorStyle.Render(value.text)
	}
	return style.Render(value.text)
}

func numberFromValue(value float64, asPercent bool) string {
	formatted := strconv.FormatFloat(value, 'f', 0, 64)
	if asPercent {
		return formatted + "%"
	}
	if value > 1000 {
		withDecimal := strconv.FormatFloat(math.Round(value/100)/10, 'f', 1, 64)
		return strings.TrimSuffix(withDecimal, ".0") + "k"
	}
	return formatted
}

func contextProgressValue(percent, tokens float64) string {
	pct := math.Max(0, math.Min(100, percent))
	color := tokenColor(tokens)
	bar := progress.New(
		progress.WithWidth(12),
		progress.WithFillCharacters('■', '·'),
		progress.WithColors(lipgloss.Color(color)),
		progress.WithoutPercentage(),
	)
	percentText := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(numberFromValue(percent, true))
	return fmt.Sprintf("%s%s%s %s", leftBracket, bar.ViewAs(pct/100), rightBracket, percentText)
}

func tokenColor(tokens float64) string {
	switch {
	case tokens < 75000:
		return "#22c55e"
	case tokens < 100000:
		return "#f59e0b"
	default:
		return "#ef4444"
	}
}

func tokenIcon(tokens float64) string {
	switch {
	case tokens < 75000:
		return "🙂"
	case tokens < 100000:
		return "🤔"
	default:
		return "🥵"
	}
}
