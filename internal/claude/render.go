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
		statusField{text: model},
		statusField{text: numberFromValue(tokens, false)},
		statusField{text: contextProgressValue(contextPercent)},
		statusField{text: numberFromValue(fiveHourPercent, true)},
		statusField{text: numberFromValue(weekPercent, true)},
	)
}

func statusLineFromParts(model, tokens, ctx, five, week statusField) string {
	parts := make([]string, 0, 5)
	if model.text != "" {
		parts = append(parts, styledValue(model, modelStyle))
	}
	if tokens.text != "" {
		tokensSegment := statusField{text: "🤔 " + tokens.text, invalid: tokens.invalid}
		parts = append(parts, styledValue(tokensSegment, tokensStyle))
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

func contextProgressValue(value float64) string {
	percent := math.Max(0, math.Min(100, value))
	bar := progress.New(
		progress.WithWidth(12),
		progress.WithFillCharacters('■', '·'),
		progress.WithColors(lipgloss.Color(contextProgressColor(percent))),
		progress.WithoutPercentage(),
	)
	return fmt.Sprintf("%s%s%s %s", leftBracket, bar.ViewAs(percent/100),
		rightBracket, numberFromValue(value, true))
}

func contextProgressColor(percent float64) string {
	switch {
	case percent <= 20:
		return "#22c55e"
	case percent <= 40:
		return "#f59e0b"
	default:
		return "#ef4444"
	}
}
