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
	claudeContextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	claudeErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	claudeFaintStyle   = lipgloss.NewStyle().Faint(true)
	claudeSeparator    = claudeFaintStyle.Render("·")
	claudeLeftBracket  = claudeFaintStyle.Render("[")
	claudeRightBracket = claudeFaintStyle.Render("]")
)

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
	fiveText := claudeStatusNumberField(payload.RateLimits.FiveHour.UsedPercentage, "5h", true, silent)
	weekText := claudeStatusNumberField(payload.RateLimits.SevenDay.UsedPercentage, "weekly", true, silent)

	fmt.Fprintln(d.stdout, claudeStatusLineFromParts(modelText, tokensText, ctxText, fiveText, weekText))
	return nil
}

func claudeStatusLineFromValues(model string, tokens float64, contextPercent float64, fiveHourPercent float64, weekPercent float64) string {
	modelText := model
	tokensText := claudeStatusNumberFromValue(tokens, false)
	ctxText := claudeStatusContextProgressValue(contextPercent)
	fiveText := claudeStatusNumberFromValue(fiveHourPercent, true)
	weekText := claudeStatusNumberFromValue(weekPercent, true)
	return claudeStatusLineFromParts(modelText, tokensText, ctxText, fiveText, weekText)
}

func claudeStatusLineFromParts(modelText string, tokensText string, ctxText string, fiveText string, weekText string) string {
	parts := make([]string, 0, 5)
	if modelText != "" {
		parts = append(parts, claudeStatusStyledValue(modelText, claudeModelStyle))
	}
	if tokensText != "" {
		tokensSegment := claudeStatusStyledValue("  "+tokensText, claudeTokensStyle)
		parts = append(parts, tokensSegment)
	}

	usageParts := make([]string, 0, 3)
	if ctxText != "" {
		usageParts = append(usageParts, claudeStatusStyledValue(ctxText, claudeContextStyle))
	}
	if fiveText != "" {
		usageParts = append(usageParts, fiveText+" of 5h")
	}
	if weekText != "" {
		usageParts = append(usageParts, claudeFaintStyle.Render(weekText+" of weekly"))
	}
	if len(usageParts) > 0 {
		parts = append(parts, strings.Join(usageParts, " "+claudeSeparator+" "))
	}
	return strings.Join(parts, " "+claudeSeparator+" ")
}

func claudeStatusStyledValue(value string, style lipgloss.Style) string {
	if strings.Contains(value, "missing/invalid") {
		return claudeErrorStyle.Render(value)
	}
	return style.Render(value)
}

func claudeStatusStringField(raw json.RawMessage, name string, silent bool) string {
	if len(raw) == 0 {
		if silent {
			return ""
		}
		return name + " missing/invalid"
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil || strings.TrimSpace(text) == "" {
		if silent {
			return ""
		}
		return name + " missing/invalid: " + string(raw)
	}
	return text
}

func claudeStatusNumberField(raw json.RawMessage, name string, asPercent bool, silent bool) string {
	if len(raw) == 0 {
		if silent {
			return ""
		}
		return name + " missing/invalid"
	}

	value, ok := claudeStatusParseNumber(raw)
	if !ok {
		if silent {
			return ""
		}
		return name + " missing/invalid: " + string(raw)
	}

	formatted := strconv.FormatFloat(value, 'f', 0, 64)
	if asPercent {
		return formatted + "%"
	}
	return claudeStatusNumberFromValue(value, false)
}

func claudeStatusNumberFromValue(value float64, asPercent bool) string {
	formatted := strconv.FormatFloat(value, 'f', 0, 64)
	if asPercent {
		return formatted + "%"
	}
	if value > 1000 {
		return strconv.FormatFloat(math.Round(value/1000), 'f', 0, 64) + "k"
	}
	return formatted
}

func claudeStatusContextProgressField(raw json.RawMessage, silent bool) string {
	if len(raw) == 0 {
		if silent {
			return ""
		}
		return "context missing/invalid"
	}

	value, ok := claudeStatusParseNumber(raw)
	if !ok {
		if silent {
			return ""
		}
		return "context missing/invalid: " + string(raw)
	}

	return claudeStatusContextProgressValue(value)
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
		return num, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, false
	}

	parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
