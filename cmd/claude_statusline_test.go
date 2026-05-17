package cmd

import (
	"strings"
	"testing"
)

func TestRunClaudeStatusLineValidInput(t *testing.T) {
	out := &strings.Builder{}
	d := deps{stdout: out, stdin: strings.NewReader(`{"context_window":{"total_input_tokens":1234,"used_percentage":51.2},"rate_limits":{"five_hour":{"used_percentage":11.4},"seven_day":{"used_percentage":72.8}},"model":{"display_name":"Claude Sonnet"}}`)}

	if err := runClaudeStatusLine(nil, d); err != nil {
		t.Fatalf("runClaudeStatusLine returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"Claude Sonnet", "used 1k tokens", "51% of total", "11% of 5h", "73% of weekly"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
}

func TestRunClaudeStatusLineTokensBelowThreshold(t *testing.T) {
	out := &strings.Builder{}
	d := deps{stdout: out, stdin: strings.NewReader(`{"context_window":{"total_input_tokens":999,"used_percentage":1},"rate_limits":{"five_hour":{"used_percentage":2},"seven_day":{"used_percentage":3}},"model":{"display_name":"Claude"}}`)}

	if err := runClaudeStatusLine(nil, d); err != nil {
		t.Fatalf("runClaudeStatusLine returned error: %v", err)
	}
	if !strings.Contains(out.String(), "used 999 tokens") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunClaudeStatusLineInvalidFields(t *testing.T) {
	out := &strings.Builder{}
	d := deps{stdout: out, stdin: strings.NewReader(`{"context_window":{"total_input_tokens":"NaN?","used_percentage":"oops"},"rate_limits":{"five_hour":{},"seven_day":{"used_percentage":88}},"model":{"display_name":9}}`)}

	if err := runClaudeStatusLine(nil, d); err != nil {
		t.Fatalf("runClaudeStatusLine returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"model missing/invalid: 9",
		"tokens missing/invalid: \"NaN?\"",
		"context missing/invalid: \"oops\"",
		"5h missing/invalid",
		"88% of weekly",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
}

func TestRunClaudeStatusLineSilentSkipsInvalid(t *testing.T) {
	out := &strings.Builder{}
	d := deps{stdout: out, stdin: strings.NewReader(`{"context_window":{"total_input_tokens":"bad"},"rate_limits":{"five_hour":{"used_percentage":10}},"model":{"display_name":"Claude"}}`)}

	if err := runClaudeStatusLine([]string{"--silent"}, d); err != nil {
		t.Fatalf("runClaudeStatusLine returned error: %v", err)
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

func TestRunClaudeStatusLineMalformedJSON(t *testing.T) {
	out := &strings.Builder{}
	d := deps{stdout: out, stdin: strings.NewReader(`{"model":`)}

	err := runClaudeStatusLine(nil, d)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(out.String(), "error: malformed JSON input") {
		t.Fatalf("expected malformed JSON message, got %q", out.String())
	}
}
