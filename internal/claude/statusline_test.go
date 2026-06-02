package claude

import (
	"strings"
	"testing"
)

func TestRunStatusLineValidInput(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine(nil, strings.NewReader(`{"context_window":{"total_input_tokens":1234,"used_percentage":51.2},"rate_limits":{"five_hour":{"used_percentage":11.4},"seven_day":{"used_percentage":72.8}},"model":{"display_name":"Claude Sonnet"}}`), out)
	if err != nil {
		t.Fatalf("RunStatusLine returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"Claude Sonnet", "1.2k", "51%", "11% of 5h", "73% of weekly"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "51% of total") {
		t.Fatalf("expected context to render as progress bar, got %q", got)
	}
}

func TestRunStatusLineTokensBelowThreshold(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine(nil, strings.NewReader(`{"context_window":{"total_input_tokens":999,"used_percentage":1},"rate_limits":{"five_hour":{"used_percentage":2},"seven_day":{"used_percentage":3}},"model":{"display_name":"Claude"}}`), out)
	if err != nil {
		t.Fatalf("RunStatusLine returned error: %v", err)
	}
	if !strings.Contains(out.String(), "999") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunStatusLineInvalidFields(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine(nil, strings.NewReader(`{"context_window":{"total_input_tokens":"NaN?","used_percentage":"oops"},"rate_limits":{"five_hour":{},"seven_day":{"used_percentage":88}},"model":{"display_name":9}}`), out)
	if err != nil {
		t.Fatalf("RunStatusLine returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"model missing/invalid: 9",
		"tokens missing/invalid: \"NaN?\"",
		"context missing/invalid: \"oops\"",
		"88% of weekly",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "5h missing/invalid") || strings.Contains(got, "weekly missing/invalid") {
		t.Fatalf("expected missing 5h/weekly values to be hidden, got %q", got)
	}
}

func TestRunStatusLineRejectsNonFiniteNumbers(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine(nil, strings.NewReader(`{"context_window":{"total_input_tokens":"NaN","used_percentage":"Infinity"},"rate_limits":{"five_hour":{"used_percentage":10},"seven_day":{"used_percentage":20}},"model":{"display_name":"Claude"}}`), out)
	if err != nil {
		t.Fatalf("RunStatusLine returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		`tokens missing/invalid: "NaN"`,
		`context missing/invalid: "Infinity"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
}

func TestRunStatusLineSilentSkipsInvalid(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine([]string{"--silent"}, strings.NewReader(`{"context_window":{"total_input_tokens":"bad"},"rate_limits":{"five_hour":{"used_percentage":10}},"model":{"display_name":"Claude"}}`), out)
	if err != nil {
		t.Fatalf("RunStatusLine returned error: %v", err)
	}

	got := out.String()
	if strings.Contains(got, "missing/invalid") {
		t.Fatalf("unexpected placeholder in silent output: %q", got)
	}
	for _, want := range []string{"Claude", "10% of 5h"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
}

func TestRunStatusLineMalformedJSON(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine(nil, strings.NewReader(`{"model":`), out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out.String(), "error: malformed JSON input") {
		t.Fatalf("expected malformed JSON message, got %q", out.String())
	}
}

func TestContextProgressColorThresholds(t *testing.T) {
	cases := []struct {
		name    string
		input   float64
		wantHex string
	}{
		{name: "green range", input: 20, wantHex: "#22c55e"},
		{name: "orange range", input: 21, wantHex: "#f59e0b"},
		{name: "red range", input: 41, wantHex: "#ef4444"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := contextProgressColor(tc.input)
			if got != tc.wantHex {
				t.Fatalf("expected %s, got %s", tc.wantHex, got)
			}
		})
	}
}

func TestRunStatusLineDemo(t *testing.T) {
	out := &strings.Builder{}

	err := RunStatusLine([]string{"--demo"}, strings.NewReader(`{"model":"ignored"}`), out)
	if err != nil {
		t.Fatalf("RunStatusLine returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), out.String())
	}

	for _, line := range lines {
		for _, want := range []string{"TheModel", "25k", "12% of 5h", "34% of weekly"} {
			if !strings.Contains(line, want) {
				t.Fatalf("line missing %q: %q", want, line)
			}
		}
	}

	for _, want := range []string{"10%", "30%", "60%"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("demo output missing %q: %q", want, out.String())
		}
	}
}
