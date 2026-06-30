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
// "10kg", "5 km", "50$". Returns ok=false if the input does not match.
func ParseInput(input string) (value float64, symbol string, ok bool) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, "", false
	}

	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return 0, "", false
	}

	numStart := 0
	hasDot := false
	hasDigit := false
	j := i
	for j < len(s) {
		c := s[j]
		if c >= '0' && c <= '9' {
			hasDigit = true
			j++
		} else if c == '.' && !hasDot {
			hasDot = true
			j++
		} else {
			break
		}
	}
	if !hasDigit {
		return 0, "", false
	}

	numStr := s[numStart:j]

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

	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, "", false
	}
	return f, sym, true
}

func isSymbolChar(c rune) bool {
	switch c {
	case '$', '€', '£', '¥', '₹', '₪', '°', 'μ':
		return true
	}
	return false
}
