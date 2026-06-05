package cmd

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func runSumWithEcho(echo bool, d deps) error {
	data, err := io.ReadAll(d.stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	input := strings.TrimSpace(string(data))

	sum := 0.0
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		sum += sumLineValue(line)
	}

	if echo {
		fmt.Fprintf(d.stdout, "%s\n\n", input)
	}
	fmt.Fprintf(d.stdout, "= %s\n", formatSumNumber(sum))
	return nil
}

func sumLineValue(line string) float64 {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0
	}
	token := strings.Split(line, " ")[0]
	return parseJSSumNumber(token)
}

func parseJSSumNumber(token string) float64 {
	if token == "" {
		return 0
	}
	if value, err := strconv.ParseFloat(token, 64); err == nil {
		return value
	}

	sign := 1.0
	unsigned := token
	switch {
	case strings.HasPrefix(unsigned, "+"):
		unsigned = unsigned[1:]
	case strings.HasPrefix(unsigned, "-"):
		sign = -1
		unsigned = unsigned[1:]
	}

	base := 0
	switch {
	case strings.HasPrefix(unsigned, "0x") || strings.HasPrefix(unsigned, "0X"):
		base = 16
		unsigned = unsigned[2:]
	case strings.HasPrefix(unsigned, "0o") || strings.HasPrefix(unsigned, "0O"):
		base = 8
		unsigned = unsigned[2:]
	case strings.HasPrefix(unsigned, "0b") || strings.HasPrefix(unsigned, "0B"):
		base = 2
		unsigned = unsigned[2:]
	}
	if base == 0 || unsigned == "" {
		return math.NaN()
	}

	n, err := strconv.ParseUint(unsigned, base, 64)
	if err != nil {
		return math.NaN()
	}
	return sign * float64(n)
}

func formatSumNumber(n float64) string {
	switch {
	case math.IsNaN(n):
		return "NaN"
	case math.IsInf(n, 1):
		return "Infinity"
	case math.IsInf(n, -1):
		return "-Infinity"
	case n == math.Trunc(n):
		return strconv.FormatFloat(n, 'f', 0, 64)
	default:
		return strconv.FormatFloat(n, 'g', -1, 64)
	}
}
