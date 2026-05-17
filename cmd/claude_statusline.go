package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	claudeModelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	claudeTokensStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	claudeContextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	claudeErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
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
	for _, arg := range args {
		switch arg {
		case "--silent":
			silent = true
		default:
			return fmt.Errorf("usage: blf claude-statusline [--silent]")
		}
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
	ctxText := claudeStatusNumberField(payload.ContextWindow.UsedPercentage, "context", true, silent)
	fiveText := claudeStatusNumberField(payload.RateLimits.FiveHour.UsedPercentage, "5h", true, silent)
	weekText := claudeStatusNumberField(payload.RateLimits.SevenDay.UsedPercentage, "weekly", true, silent)

	parts := make([]string, 0, 5)
	if modelText != "" {
		parts = append(parts, claudeStatusStyledValue(modelText, claudeModelStyle))
	}
	if tokensText != "" {
		tokensSegment := claudeStatusStyledValue("used "+tokensText+" tokens", claudeTokensStyle)
		parts = append(parts, tokensSegment)
	}

	usageParts := make([]string, 0, 3)
	if ctxText != "" {
		usageParts = append(usageParts, claudeStatusStyledValue(ctxText+" of total", claudeContextStyle))
	}
	if fiveText != "" {
		usageParts = append(usageParts, fiveText+" of 5h")
	}
	if weekText != "" {
		usageParts = append(usageParts, weekText+" of weekly")
	}
	if len(usageParts) > 0 {
		parts = append(parts, "("+strings.Join(usageParts, ", ")+")")
	}

	fmt.Fprintln(d.stdout, strings.Join(parts, " "))
	return nil
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
	if value > 1000 {
		return strconv.FormatFloat(math.Round(value/1000), 'f', 0, 64) + "k"
	}
	return formatted
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
