package claude

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

type statusField struct {
	text    string
	invalid bool
}

type statusLineData struct {
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

func errorField(raw json.RawMessage, name string, silent bool) statusField {
	if silent {
		return statusField{}
	}
	if len(raw) == 0 {
		return statusField{text: name + " missing/invalid", invalid: true}
	}
	return statusField{text: name + " missing/invalid: " + string(raw), invalid: true}
}

func parseStringField(raw json.RawMessage, name string, silent bool) statusField {
	if len(raw) == 0 {
		return errorField(raw, name, silent)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil || strings.TrimSpace(text) == "" {
		if silent {
			return statusField{}
		}
		return errorField(raw, name, silent)
	}
	return statusField{text: text}
}

func parseNumberField(raw json.RawMessage, name string, asPercent bool, silent bool) statusField {
	if len(raw) == 0 {
		return errorField(raw, name, silent)
	}

	value, ok := parseNumber(raw)
	if !ok {
		if silent {
			return statusField{}
		}
		return errorField(raw, name, silent)
	}

	return statusField{text: numberFromValue(value, asPercent)}
}

func parseContextProgressField(raw json.RawMessage, silent bool) statusField {
	if len(raw) == 0 {
		return errorField(raw, "context", silent)
	}

	value, ok := parseNumber(raw)
	if !ok {
		if silent {
			return statusField{}
		}
		return errorField(raw, "context", silent)
	}

	return statusField{text: contextProgressValue(value)}
}

func parseNumber(raw json.RawMessage) (float64, bool) {
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
