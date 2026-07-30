package claude

import (
	"encoding/json"
	"fmt"
	"io"
)

func RunStatusLine(args []string, stdin io.Reader, stdout io.Writer) error {
	silent := false
	demo := false
	for _, arg := range args {
		switch arg {
		case "--silent":
			silent = true
		case "--demo":
			demo = true
		default:
			return fmt.Errorf("usage: blf claude statusline [--silent] [--demo]")
		}
	}
	if demo {
		for _, row := range []struct {
			tokens  float64
			percent int
		}{
			{50000, 10},
			{85000, 30},
			{120000, 60},
		} {
			fmt.Fprintln(stdout, statusLineFromValues("TheModel", row.tokens, float64(row.percent), 12, 34))
		}
		return nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var payload statusLineData
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(stdout, "%s\n", errorStyle.Render("error: malformed JSON input"))
		return fmt.Errorf("parse JSON: %w", err)
	}

	rawTokens, _ := parseNumber(payload.ContextWindow.TotalInputTokens)
	modelText := parseStringField(payload.Model.DisplayName, "model", silent)
	tokensText := parseNumberField(payload.ContextWindow.TotalInputTokens, "tokens", false, silent)
	ctxText := parseContextProgressField(payload.ContextWindow.UsedPercentage, rawTokens, silent)
	fiveText := parseNumberField(payload.RateLimits.FiveHour.UsedPercentage, "5h", true, true)
	weekText := parseNumberField(payload.RateLimits.SevenDay.UsedPercentage, "weekly", true, true)

	fmt.Fprintln(stdout, statusLineFromParts(rawTokens, modelText, tokensText, ctxText, fiveText, weekText))
	return nil
}
