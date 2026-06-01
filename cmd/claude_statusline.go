package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

var (
	claudeModelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	claudeTokensStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	claudeErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	claudeFaintStyle   = lipgloss.NewStyle().Faint(true)
	claudePlainStyle   = lipgloss.NewStyle()
	claudeSeparator    = claudeFaintStyle.Render("·")
	claudeLeftBracket  = claudeFaintStyle.Render("[")
	claudeRightBracket = claudeFaintStyle.Render("]")
)

type claudeStatusField struct {
	text    string
	invalid bool
}

type claudeStatusLineData struct {
	ContextWindow struct {
		TotalInputTokens json.RawMessage `json:"total_input_tokens"`
		UsedPercentage   json.RawMessage `json:"used_percentage"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour struct {
			UsedPercentage json.RawMessage `json:"used_percentage"`
		} `json:"five_hour"`
		SevenDay struct {
			UsedPercentage json.RawMessage `json:"used_percentage"`
		} `json:"seven_day"`
	} `json:"rate_limits"`
	Model struct {
		DisplayName json.RawMessage `json:"display_name"`
	} `json:"model"`
}

func runClaudeStatusLine(args []string, d deps) error {
	silent := false
	demo := false
	for _, arg := range args {
		switch arg {
		case "--silent":
			silent = true
		case "--demo":
			demo = true
		default:
			return fmt.Errorf("usage: blf claude-statusline [--silent] [--demo]")
		}
	}
	if demo {
		for _, percent := range []int{10, 30, 60} {
			fmt.Fprintln(d.stdout, claudeStatusLineFromValues("TheModel", 25000, float64(percent), 12, 34))
		}
		return nil
	}

	data, err := io.ReadAll(d.stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var payload claudeStatusLineData
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(d.stdout, "%s\n", claudeErrorStyle.Render("error: malformed JSON input"))
		return fmt.Errorf("parse JSON: %w", err)
	}

	modelText := claudeStatusStringField(payload.Model.DisplayName, "model", silent)
	tokensText := claudeStatusNumberField(payload.ContextWindow.TotalInputTokens, "tokens", false, silent)
	ctxText := claudeStatusContextProgressField(payload.ContextWindow.UsedPercentage, silent)
	fiveText := claudeStatusNumberField(payload.RateLimits.FiveHour.UsedPercentage, "5h", true, true)
	weekText := claudeStatusNumberField(payload.RateLimits.SevenDay.UsedPercentage, "weekly", true, true)

	fmt.Fprintln(d.stdout, claudeStatusLineFromParts(modelText, tokensText, ctxText, fiveText, weekText))
	return nil
}

func claudeStatusLineFromValues(model string, tokens float64, contextPercent float64, fiveHourPercent float64, weekPercent float64) string {
	modelText := claudeStatusField{text: model}
	tokensText := claudeStatusField{text: claudeStatusNumberFromValue(tokens, false)}
	ctxText := claudeStatusField{text: claudeStatusContextProgressValue(contextPercent)}
	fiveText := claudeStatusField{text: claudeStatusNumberFromValue(fiveHourPercent, true)}
	weekText := claudeStatusField{text: claudeStatusNumberFromValue(weekPercent, true)}
	return claudeStatusLineFromParts(modelText, tokensText, ctxText, fiveText, weekText)
}

func claudeStatusLineFromParts(model claudeStatusField, tokens claudeStatusField, ctx claudeStatusField, five claudeStatusField, week claudeStatusField) string {
	parts := make([]string, 0, 5)
	if model.text != "" {
		parts = append(parts, claudeStatusStyledValue(model, claudeModelStyle))
	}
	if tokens.text != "" {

		tokensSegment := claudeStatusField{text: "🤔 " + tokens.text, invalid: tokens.invalid}
		tokensSegmentRendered := claudeStatusStyledValue(tokensSegment, claudeTokensStyle)
		parts = append(parts, tokensSegmentRendered)
	}

	usageParts := make([]string, 0, 3)
	if ctx.text != "" {
		usageParts = append(usageParts, claudeStatusStyledValue(ctx, claudePlainStyle))
	}
	if five.text != "" {
		usageParts = append(usageParts, claudeStatusStyledValue(
			claudeStatusField{text: five.text + " of 5h", invalid: five.invalid}, claudePlainStyle,
		))
	}
	if week.text != "" {
		usageParts = append(usageParts, claudeStatusStyledValue(
			claudeStatusField{text: week.text + " of weekly", invalid: week.invalid}, claudeFaintStyle,
		))
	}
	if len(usageParts) > 0 {
		parts = append(parts, strings.Join(usageParts, " "+claudeSeparator+" "))
	}
	return strings.Join(parts, " "+claudeSeparator+" ")
}

func claudeStatusStyledValue(value claudeStatusField, style lipgloss.Style) string {
	if value.invalid {
		return claudeErrorStyle.Render(value.text)
	}
	return style.Render(value.text)
}

func claudeStatusMissingInvalid(raw json.RawMessage, name string, silent bool) claudeStatusField {
	if silent {
		return claudeStatusField{}
	}
	if len(raw) == 0 {
		return claudeStatusField{text: name + " missing/invalid", invalid: true}
	}
	return claudeStatusField{text: name + " missing/invalid: " + string(raw), invalid: true}
}

func claudeStatusStringField(raw json.RawMessage, name string, silent bool) claudeStatusField {
	if len(raw) == 0 {
		return claudeStatusMissingInvalid(raw, name, silent)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil || strings.TrimSpace(text) == "" {
		if silent {
			return claudeStatusField{}
		}
		return claudeStatusMissingInvalid(raw, name, silent)
	}
	return claudeStatusField{text: text}
}

func claudeStatusNumberField(raw json.RawMessage, name string, asPercent bool, silent bool) claudeStatusField {
	if len(raw) == 0 {
		return claudeStatusMissingInvalid(raw, name, silent)
	}

	value, ok := claudeStatusParseNumber(raw)
	if !ok {
		if silent {
			return claudeStatusField{}
		}
		return claudeStatusMissingInvalid(raw, name, silent)
	}

	if asPercent {
		return claudeStatusField{text: claudeStatusNumberFromValue(value, true)}
	}
	return claudeStatusField{text: claudeStatusNumberFromValue(value, false)}
}

func claudeStatusNumberFromValue(value float64, asPercent bool) string {
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

func claudeStatusContextProgressField(raw json.RawMessage, silent bool) claudeStatusField {
	if len(raw) == 0 {
		return claudeStatusMissingInvalid(raw, "context", silent)
	}

	value, ok := claudeStatusParseNumber(raw)
	if !ok {
		if silent {
			return claudeStatusField{}
		}
		return claudeStatusMissingInvalid(raw, "context", silent)
	}

	return claudeStatusField{text: claudeStatusContextProgressValue(value)}
}

func claudeStatusContextProgressValue(value float64) string {
	percent := math.Max(0, math.Min(100, value))
	bar := progress.New(
		progress.WithWidth(12),
		progress.WithFillCharacters('■', '·'),
		progress.WithColors(lipgloss.Color(claudeStatusContextProgressColor(percent))),
		progress.WithoutPercentage(),
	)

	return fmt.Sprintf("%s%s%s %s", claudeLeftBracket, bar.ViewAs(percent/100),
		claudeRightBracket, claudeStatusNumberFromValue(value, true))
}

func claudeStatusContextProgressColor(percent float64) string {
	switch {
	case percent <= 20:
		return "#22c55e"
	case percent <= 40:
		return "#f59e0b"
	default:
		return "#ef4444"
	}
}

func claudeStatusParseNumber(raw json.RawMessage) (float64, bool) {
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		if math.IsNaN(num) || math.IsInf(num, 0) {
			return 0, false
		}
		return num, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, false
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}
