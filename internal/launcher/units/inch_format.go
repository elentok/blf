package units

import (
	"fmt"
	"math"
	"strconv"
)

// formatInchFraction renders v (a decimal inch value) as a fractional readout:
// the nearest 1/16" mark when v is within 1/64" of it, or the enclosing range
// of 1/16" marks otherwise. The sign is applied once, from v's absolute value.
func formatInchFraction(v float64) string {
	const epsilon = 1e-9

	neg := v < 0
	abs := math.Abs(v)
	sixteenths := abs * 16
	nearest := math.Round(sixteenths)
	dist := math.Abs(sixteenths - nearest)

	if dist <= 0.25+epsilon {
		prefix := ""
		if dist > epsilon {
			prefix = "~"
		}
		if neg {
			prefix += "-"
		}
		return prefix + renderSixteenths(int64(nearest)) + " inch"
	}

	lower := renderSixteenths(int64(math.Floor(sixteenths)))
	upper := renderSixteenths(int64(math.Floor(sixteenths)) + 1)
	if neg {
		lower, upper = "-"+upper, "-"+lower
	}
	return "between " + lower + " and " + upper + " inch"
}

// renderSixteenths renders a nonnegative count of 1/16" units as a reduced
// fraction, or a mixed number when the magnitude is at least 1 inch.
func renderSixteenths(n int64) string {
	whole := n / 16
	rem := n % 16
	if rem == 0 {
		return strconv.FormatInt(whole, 10)
	}
	g := gcd(rem, 16)
	num, den := rem/g, 16/g
	if whole == 0 {
		return fmt.Sprintf("%d/%d", num, den)
	}
	return fmt.Sprintf("%d %d/%d", whole, num, den)
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
