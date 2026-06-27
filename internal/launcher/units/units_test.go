package units_test

import (
	"math"
	"testing"

	"github.com/elentok/blf/internal/launcher/units"
)

func TestLinearConversion(t *testing.T) {
	r := units.NewBuiltinRegistry()

	tests := []struct {
		value  float64
		symbol string
		want   map[string]float64
	}{
		{5, "km", map[string]float64{"m": 5000, "cm": 500000, "mi": 5000 / 1609.344}},
		{1, "mi", map[string]float64{"km": 1.609344, "m": 1609.344, "ft": 5280}},
		{100, "cm", map[string]float64{"m": 1, "mm": 1000}},
		{1, "kg", map[string]float64{"g": 1000, "lb": 1 / 0.45359237}},
		{1, "h", map[string]float64{"min": 60, "s": 3600}},
		{1, "gb", map[string]float64{"mb": 1000, "kb": 1e6, "b": 1e9}},
	}

	for _, tc := range tests {
		group, unit, ok := r.Lookup(tc.symbol)
		if !ok {
			t.Fatalf("symbol %q not found", tc.symbol)
		}
		convs := r.Convert(tc.value, unit, group)
		got := map[string]float64{}
		for _, c := range convs {
			for _, sym := range c.Unit.Symbols {
				got[sym] = c.Value
			}
		}
		for wantSym, wantVal := range tc.want {
			v, found := got[wantSym]
			if !found {
				t.Errorf("%v %s → %s: not found in conversions", tc.value, tc.symbol, wantSym)
				continue
			}
			if !approx(v, wantVal, 1e-6) {
				t.Errorf("%v %s → %s: got %v, want %v", tc.value, tc.symbol, wantSym, v, wantVal)
			}
		}
	}
}

func TestAffineTemperature(t *testing.T) {
	r := units.NewBuiltinRegistry()

	cases := []struct {
		fromSym string
		value   float64
		toSym   string
		want    float64
	}{
		{"c", 0, "f", 32},
		{"c", 100, "f", 212},
		{"f", 32, "c", 0},
		{"f", 212, "c", 100},
		{"c", 0, "k", 273.15},
		{"k", 273.15, "c", 0},
		{"f", 32, "k", 273.15},
	}

	for _, tc := range cases {
		fromGroup, fromUnit, ok := r.Lookup(tc.fromSym)
		if !ok {
			t.Fatalf("symbol %q not found", tc.fromSym)
		}
		_, toUnit, ok := r.Lookup(tc.toSym)
		if !ok {
			t.Fatalf("symbol %q not found", tc.toSym)
		}
		convs := r.Convert(tc.value, fromUnit, fromGroup)
		var got float64
		found := false
		for _, c := range convs {
			if c.Unit == toUnit {
				got = c.Value
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%v %s → %s: no conversion found", tc.value, tc.fromSym, tc.toSym)
			continue
		}
		if !approx(got, tc.want, 1e-9) {
			t.Errorf("%v %s → %s: got %v, want %v", tc.value, tc.fromSym, tc.toSym, got, tc.want)
		}
	}
}

func TestSymbolPriority(t *testing.T) {
	r := units.NewBuiltinRegistry()

	// "m" → meters (length), not minutes
	g, u, ok := r.Lookup("m")
	if !ok {
		t.Fatal("symbol 'm' not found")
	}
	if g.Name != "length" {
		t.Errorf("'m' → group %q, want length", g.Name)
	}
	if u.Name != "meter" {
		t.Errorf("'m' → unit %q, want meter", u.Name)
	}

	// "min" → minutes (time)
	g2, u2, ok := r.Lookup("min")
	if !ok {
		t.Fatal("symbol 'min' not found")
	}
	if g2.Name != "time" {
		t.Errorf("'min' → group %q, want time", g2.Name)
	}
	if u2.Name != "minute" {
		t.Errorf("'min' → unit %q, want minute", u2.Name)
	}
}

func TestParseInput(t *testing.T) {
	tests := []struct {
		input  string
		value  float64
		symbol string
		ok     bool
	}{
		{"10km", 10, "km", true},
		{"5 kg", 5, "kg", true},
		{"100$", 100, "$", true},
		{"0.5l", 0.5, "l", true},
		{"-5c", -5, "c", true},
		{"abc", 0, "", false},
		{"10", 0, "", false},
		{"", 0, "", false},
		{"10KM", 10, "km", true}, // symbols are lowercased
	}

	for _, tc := range tests {
		v, sym, ok := units.ParseInput(tc.input)
		if ok != tc.ok {
			t.Errorf("ParseInput(%q) ok=%v, want %v", tc.input, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		if v != tc.value {
			t.Errorf("ParseInput(%q) value=%v, want %v", tc.input, v, tc.value)
		}
		if sym != tc.symbol {
			t.Errorf("ParseInput(%q) symbol=%q, want %q", tc.input, sym, tc.symbol)
		}
	}
}

func approx(a, b, eps float64) bool {
	diff := math.Abs(a - b)
	if diff < eps {
		return true
	}
	mag := math.Max(math.Abs(a), math.Abs(b))
	if mag == 0 {
		return diff == 0
	}
	return diff/mag < eps
}
