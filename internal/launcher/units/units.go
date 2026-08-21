package units

import (
	"strconv"
	"strings"
	"unicode"
)

// Unit is a single measurement unit within a Group.
// Converting to base: base = value*Factor + Offset
// Converting from base: value = (base - Offset) / Factor
type Unit struct {
	Name    string
	Symbols []string // lowercase lookup symbols
	Factor  float64
	Offset  float64
}

// Group is a named collection of mutually convertible units.
type Group struct {
	Name  string
	Units []*Unit
}

// Conversion is one output row produced by Convert.
type Conversion struct {
	Value float64
	Unit  *Unit
	Group *Group
}

type symbolEntry struct {
	group *Group
	unit  *Unit
}

// Registry holds unit groups and a symbol → unit index.
type Registry struct {
	Groups  []*Group
	symbols map[string]*symbolEntry
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{symbols: make(map[string]*symbolEntry)}
}

// AddGroup registers a group. For each unit symbol, only the first registration
// wins — caller controls priority by registration order.
func (r *Registry) AddGroup(g *Group) {
	r.Groups = append(r.Groups, g)
	for _, u := range g.Units {
		for _, sym := range u.Symbols {
			s := strings.ToLower(sym)
			if _, exists := r.symbols[s]; !exists {
				r.symbols[s] = &symbolEntry{group: g, unit: u}
			}
		}
	}
}

// ForceSymbol overrides any existing symbol registration.
// Use this to implement fixed-precedence entries like "$"→USD.
func (r *Registry) ForceSymbol(sym string, g *Group, u *Unit) {
	r.symbols[strings.ToLower(sym)] = &symbolEntry{group: g, unit: u}
}

// Lookup returns the group and unit for a symbol (case-insensitive).
func (r *Registry) Lookup(symbol string) (*Group, *Unit, bool) {
	if e, ok := r.symbols[strings.ToLower(symbol)]; ok {
		return e.group, e.unit, true
	}
	return nil, nil, false
}

// Convert converts value from fromUnit to all other units in its group,
// returning one Conversion per target unit (excluding the source unit).
func (r *Registry) Convert(value float64, from *Unit, group *Group) []Conversion {
	base := value*from.Factor + from.Offset
	out := make([]Conversion, 0, len(group.Units)-1)
	for _, u := range group.Units {
		if u == from {
			continue
		}
		converted := (base - u.Offset) / u.Factor
		out = append(out, Conversion{Value: converted, Unit: u, Group: group})
	}
	return out
}

// ParseInput extracts a float64 value and a unit symbol from strings like
// "10kg", "5 km", "50$", "3/8 in", "1 1/4 in". Returns ok=false if the input
// does not match.
func ParseInput(input string) (value float64, symbol string, ok bool) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, "", false
	}

	value, j, matched := ScanQuantity(s)
	if !matched {
		return 0, "", false
	}

	// skip optional whitespace between number and unit
	for j < len(s) && s[j] == ' ' {
		j++
	}
	if j >= len(s) {
		return 0, "", false
	}

	sym := strings.ToLower(s[j:])
	for _, c := range sym {
		if !unicode.IsLetter(c) && !isSymbolChar(c) {
			return 0, "", false
		}
	}
	if sym == "" {
		return 0, "", false
	}

	return value, sym, true
}

// ScanQuantity parses a decimal, fraction (3/8), or mixed-number (1 1/4)
// quantity at the start of s, optionally prefixed with '-'. It returns the
// parsed value and the index immediately following it. ok is false if s
// does not start with a well-formed quantity (e.g. missing numerator or
// denominator, denominator of zero, or a stray second '/').
//
// Shared between ParseInput and router.matchesNumberUnit so the two
// functions agree on what counts as unit-shaped input.
func ScanQuantity(s string) (value float64, end int, ok bool) {
	neg := false
	i := 0
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	if i >= len(s) || s[i] < '0' || s[i] > '9' {
		return 0, 0, false
	}

	digitsStart := i
	i = scanDigits(s, i)
	intPart := s[digitsStart:i]

	var val float64
	switch {
	case i < len(s) && s[i] == '/':
		// bare fraction: intPart/denominator
		i++
		denStart := i
		i = scanDigits(s, i)
		if i == denStart || (i < len(s) && s[i] == '/') {
			return 0, 0, false
		}
		num, _ := strconv.ParseFloat(intPart, 64)
		den, _ := strconv.ParseFloat(s[denStart:i], 64)
		if den == 0 {
			return 0, 0, false
		}
		val = num / den

	default:
		// mixed number: <ws>+ <digits> '/' <digits>
		wsEnd := i
		for wsEnd < len(s) && s[wsEnd] == ' ' {
			wsEnd++
		}
		fracStart := wsEnd
		fracEnd := scanDigits(s, fracStart)
		if wsEnd > i && fracEnd > fracStart && fracEnd < len(s) && s[fracEnd] == '/' {
			denStart := fracEnd + 1
			denEnd := scanDigits(s, denStart)
			if denEnd == denStart || (denEnd < len(s) && s[denEnd] == '/') {
				return 0, 0, false
			}
			whole, _ := strconv.ParseFloat(intPart, 64)
			num, _ := strconv.ParseFloat(s[fracStart:fracEnd], 64)
			den, _ := strconv.ParseFloat(s[denStart:denEnd], 64)
			if den == 0 {
				return 0, 0, false
			}
			val = whole + num/den
			i = denEnd
		} else {
			// plain decimal, possibly with a fractional part
			if i < len(s) && s[i] == '.' {
				i++
				i = scanDigits(s, i)
			}
			f, err := strconv.ParseFloat(s[digitsStart:i], 64)
			if err != nil {
				return 0, 0, false
			}
			val = f
		}
	}

	if neg {
		val = -val
	}
	return val, i, true
}

func scanDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

func isSymbolChar(c rune) bool {
	switch c {
	case '$', '€', '£', '¥', '₹', '₪', '°', 'μ':
		return true
	}
	return false
}
