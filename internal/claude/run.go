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
			return fmt.Errorf("usage: blf claude-statusline [--silent] [--demo]")
		}
	}
	if demo {
		for _, percent := range []int{10, 30, 60} {
			fmt.Fprintln(stdout, statusLineFromValues("TheModel", 25000, float64(percent), 12, 34))
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

	modelText := parseStringField(payload.Model.DisplayName, "model", silent)
	tokensText := parseNumberField(payload.ContextWindow.TotalInputTokens, "tokens", false, silent)
	ctxText := parseContextProgressField(payload.ContextWindow.UsedPercentage, silent)
	fiveText := parseNumberField(payload.RateLimits.FiveHour.UsedPercentage, "5h", true, true)
	weekText := parseNumberField(payload.RateLimits.SevenDay.UsedPercentage, "weekly", true, true)

	fmt.Fprintln(stdout, statusLineFromParts(modelText, tokensText, ctxText, fiveText, weekText))
	return nil
}
