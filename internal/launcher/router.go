package launcher

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// QueryType classifies a launcher input string.
type QueryType int

const (
	// NameLike inputs are fuzzy-matched against apps and scripts.
	NameLike QueryType = iota
	// LargeBareNumber is a bare integer with ≥4 digits. It is treated as
	// name-like (fuzzy app/script search still runs) but also produces a
	// comma-formatted result row.
	LargeBareNumber
	// Computational inputs resolve to a value (math expression or unit/currency
	// conversion) and suppress the fuzzy app/script list.
	Computational
)

var knownFunctions = map[string]bool{
	"sqrt": true, "cbrt": true, "abs": true, "round": true,
	"floor": true, "ceil": true, "ln": true, "log": true,
	"log2": true, "exp": true, "pow": true, "min": true,
	"max": true, "sin": true, "cos": true, "tan": true,
	"asin": true, "acos": true, "atan": true, "rad": true,
}

// Classify determines whether input is a computational query, a large bare
// number, or a name-like query.
func Classify(input string) QueryType {
	s := strings.TrimSpace(input)
	if s == "" {
		return NameLike
	}

	if isBareNumber(s) {
		if digitCount(s) >= 4 {
			return LargeBareNumber
		}
		return NameLike
	}

	if matchesNumberUnit(s) {
		return Computational
	}

	// Unambiguous math operators
	for _, ch := range s {
		if ch == '+' || ch == '*' || ch == '/' || ch == '^' || ch == '%' {
			return Computational
		}
	}

	// Binary minus: a '-' not at position 0 means subtraction
	if idx := strings.IndexByte(s, '-'); idx > 0 {
		return Computational
	}

	// Function call: known name immediately followed by '('
	if containsFunctionCall(s) {
		return Computational
	}

	return NameLike
}

// Rank returns a copy of results sorted by:
//
//	exact match > prefix match > source weight > fuzzy score
func Rank(results []Result) []Result {
	out := make([]Result, len(results))
	copy(out, results)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.IsExactMatch != b.IsExactMatch:
			return a.IsExactMatch
		case a.IsPrefixMatch != b.IsPrefixMatch:
			return a.IsPrefixMatch
		case a.Weight != b.Weight:
			return a.Weight > b.Weight
		default:
			return a.FuzzyScore > b.FuzzyScore
		}
	})
	return out
}

// FormatNumber formats n with comma grouping. Integers omit the decimal;
// fractions use the shortest representation.
func FormatNumber(n float64) string {
	if math.IsNaN(n) {
		return "NaN"
	}
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}

	isInt := n == math.Trunc(n)
	var s string
	if isInt {
		s = fmt.Sprintf("%.0f", math.Abs(n))
	} else {
		s = fmt.Sprintf("%g", math.Abs(n))
		// Only add commas to the integer part
		if dot := strings.IndexByte(s, '.'); dot >= 0 {
			s = addCommas(s[:dot]) + s[dot:]
		} else {
			s = addCommas(s)
		}
		if n < 0 {
			return "-" + s
		}
		return s
	}

	s = addCommas(s)
	if n < 0 {
		return "-" + s
	}
	return s
}

func addCommas(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	rem := len(digits) % 3
	if rem > 0 {
		b.WriteString(digits[:rem])
	}
	for i := rem; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// isBareNumber reports whether s is a plain integer or decimal number
// (optionally negative). E.g. "42", "-3", "3.14".
func isBareNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	if i == len(s) {
		return false
	}
	hasDot := false
	hasDigit := false
	for ; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if c == '.' {
			if hasDot {
				return false
			}
			hasDot = true
		} else {
			return false
		}
	}
	return hasDigit
}

func digitCount(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n++
		}
	}
	return n
}

// matchesNumberUnit reports whether s looks like a number followed by a unit
// token (letters or known currency/degree symbols), e.g. "10cm", "123$", "5kg".
func matchesNumberUnit(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	if i >= len(s) || !(s[i] >= '0' && s[i] <= '9') {
		return false
	}
	hasDigit := false
	hasDot := false
	for i < len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
			i++
		} else if c == '.' && !hasDot {
			hasDot = true
			i++
		} else {
			break
		}
	}
	if !hasDigit {
		return false
	}
	// skip optional whitespace
	for i < len(s) && s[i] == ' ' {
		i++
	}
	if i >= len(s) {
		return false
	}
	// Unit token must start with a lowercase letter or a unit symbol.
	// This rejects app names like "1Password" whose suffix starts uppercase.
	unitRunes := []rune(s[i:])
	if len(unitRunes) == 0 {
		return false
	}
	first := unitRunes[0]
	if !isUnitSymbol(first) && (!unicode.IsLetter(first) || !unicode.IsLower(first)) {
		return false
	}
	// remaining characters must be unit-like (letters or unit symbols)
	for _, c := range unitRunes {
		if !unicode.IsLetter(c) && !isUnitSymbol(c) {
			return false
		}
	}
	return true
}

func isUnitSymbol(c rune) bool {
	switch c {
	case '$', '€', '£', '¥', '₹', '₪', '°', 'μ':
		return true
	}
	return false
}

// containsFunctionCall reports whether s contains a known math function
// name immediately followed by '(' (ignoring case).
func containsFunctionCall(s string) bool {
	lower := strings.ToLower(s)
	for fn := range knownFunctions {
		idx := 0
		for {
			pos := strings.Index(lower[idx:], fn)
			if pos < 0 {
				break
			}
			abs := idx + pos
			end := abs + len(fn)
			// skip whitespace between name and paren
			j := end
			for j < len(lower) && lower[j] == ' ' {
				j++
			}
			if j < len(lower) && lower[j] == '(' {
				return true
			}
			idx = abs + 1
		}
	}
	return false
}
