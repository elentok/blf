package cmd

import (
	"math"
	"strings"
	"testing"
)

func TestRunSumSumsFirstTokenFromEachLine(t *testing.T) {
	out := &strings.Builder{}
	err := runSumWithEcho(false, deps{
		stdout: out,
		stdin:  strings.NewReader("1 apple\n2 banana\n\n3\n"),
	})
	if err != nil {
		t.Fatalf("runSumWithEcho returned error: %v", err)
	}
	if out.String() != "= 6\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunSumEchoesTrimmedInput(t *testing.T) {
	out := &strings.Builder{}
	err := runSumWithEcho(true, deps{
		stdout: out,
		stdin:  strings.NewReader("1 apple\n2 banana\n\n3\n"),
	})
	if err != nil {
		t.Fatalf("runSumWithEcho returned error: %v", err)
	}
	if out.String() != "1 apple\n2 banana\n\n3\n\n= 6\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunSumPreservesNaNBehavior(t *testing.T) {
	out := &strings.Builder{}
	err := runSumWithEcho(false, deps{
		stdout: out,
		stdin:  strings.NewReader("4.5 foo\nbar\n"),
	})
	if err != nil {
		t.Fatalf("runSumWithEcho returned error: %v", err)
	}
	if out.String() != "= NaN\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunSumSupportsJSNumberForms(t *testing.T) {
	out := &strings.Builder{}
	err := runSumWithEcho(false, deps{
		stdout: out,
		stdin:  strings.NewReader("0x10 hex\n1e2 sci\nInfinity x\n"),
	})
	if err != nil {
		t.Fatalf("runSumWithEcho returned error: %v", err)
	}
	if out.String() != "= Infinity\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestParseJSSumNumberMatchesExpectedCases(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{input: "4.5", want: 4.5},
		{input: "1e2", want: 100},
		{input: "0x10", want: 16},
		{input: "0o10", want: 8},
		{input: "0b10", want: 2},
		{input: "Infinity", want: math.Inf(1)},
		{input: "-Infinity", want: math.Inf(-1)},
		{input: "true", want: math.NaN()},
	}

	for _, tt := range tests {
		got := parseJSSumNumber(tt.input)
		if math.IsNaN(tt.want) {
			if !math.IsNaN(got) {
				t.Fatalf("parseJSSumNumber(%q) = %v, want NaN", tt.input, got)
			}
			continue
		}
		if got != tt.want {
			t.Fatalf("parseJSSumNumber(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
