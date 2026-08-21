package launcher_test

import (
	"strings"
	"testing"

	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/units"
)

func TestUnitsProviderLength(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("5km")
	if len(results) == 0 {
		t.Fatal("expected results for '5km', got none")
	}
	// Should include a meters result
	found := false
	for _, r := range results {
		if r.Title == "5,000 meter" {
			found = true
			break
		}
	}
	if !found {
		titles := make([]string, len(results))
		for i, r := range results {
			titles[i] = r.Title
		}
		t.Errorf("expected '5,000 meter' in results, got: %v", titles)
	}
}

func TestUnitsProviderTemperature(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("0c")
	if len(results) == 0 {
		t.Fatal("expected results for '0c'")
	}
	// Should convert 0°C to 32°F
	found := false
	for _, r := range results {
		if r.Title == "32 fahrenheit" {
			found = true
			break
		}
	}
	if !found {
		titles := make([]string, len(results))
		for i, r := range results {
			titles[i] = r.Title
		}
		t.Errorf("expected '32 fahrenheit' in results, got: %v", titles)
	}
}

func TestUnitsProviderSymbolCollision_M(t *testing.T) {
	// "100m" should match meters (length), not minutes (time)
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("100m")
	if len(results) == 0 {
		t.Fatal("expected results for '100m'")
	}
	for _, r := range results {
		if r.Source == "units" && r.Title == "0.1 kilometer" {
			return // found expected km conversion from 100m
		}
	}
	t.Errorf("'100m' should produce a kilometer result; got: %v", titlesOf(results))
}

func TestUnitsProviderSymbolCollision_Min(t *testing.T) {
	// "60min" should match minutes (time), not something in length
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("60min")
	if len(results) == 0 {
		t.Fatal("expected results for '60min'")
	}
	for _, r := range results {
		if r.Source == "units" && r.Title == "1 hour" {
			return
		}
	}
	t.Errorf("'60min' should produce '1 hour'; got: %v", titlesOf(results))
}

func TestUnitsProviderNonUnit(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	// Pure math expression should return nil from UnitsProvider
	results := p.Query("1+2")
	if len(results) != 0 {
		t.Errorf("'1+2' should return no results from UnitsProvider, got: %v", len(results))
	}
}

func TestUnitsProviderExtraFormat_Hook(t *testing.T) {
	r := units.NewRegistry()
	from := &units.Unit{Name: "widget", Symbols: []string{"wg"}, Factor: 1}
	hooked := &units.Unit{Name: "gizmo", Symbols: []string{"gz"}, Factor: 1, ExtraFormat: func(v float64) string {
		return "hooked"
	}}
	r.AddGroup(&units.Group{Name: "test", Units: []*units.Unit{from, hooked}})

	p := launcher.NewUnitsProvider(r, nil, nil)
	results := p.Query("5wg")
	found := false
	for _, res := range results {
		if res.Title == "5 gizmo (hooked)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected '5 gizmo (hooked)' in results, got: %v", titlesOf(results))
	}
}

func TestUnitsProviderExtraFormat_NilHookNoParenthetical(t *testing.T) {
	r := units.NewRegistry()
	from := &units.Unit{Name: "widget", Symbols: []string{"wg"}, Factor: 1}
	plain := &units.Unit{Name: "sprocket", Symbols: []string{"sp"}, Factor: 1}
	r.AddGroup(&units.Group{Name: "test", Units: []*units.Unit{from, plain}})

	p := launcher.NewUnitsProvider(r, nil, nil)
	results := p.Query("5wg")
	for _, res := range results {
		if res.Title == "5 sprocket" {
			return
		}
	}
	t.Errorf("expected '5 sprocket' with no parenthetical, got: %v", titlesOf(results))
}

func TestUnitsProviderInchFraction_NearMarkApprox(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	// 1.6mm = 0.062992... inch, within 1/64" of 1/16 (0.0625)
	results := p.Query("1.6mm")
	found := false
	for _, r := range results {
		if r.Source == "units" && strings.HasPrefix(r.Title, "0.062992") && strings.Contains(r.Title, "(~1/16 inch)") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an inch result with '(~1/16 inch)' suffix, got: %v", titlesOf(results))
	}
}

func TestUnitsProviderInchFraction_ExactMark(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("9.525mm") // = 0.375 inch exactly
	found := false
	for _, r := range results {
		if r.Source == "units" && r.Title == "0.375 inch (3/8 inch)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '0.375 inch (3/8 inch)', got: %v", titlesOf(results))
	}
}

func TestUnitsProviderInchFraction_Range(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("10.16mm") // = 0.4 inch exactly, between 3/8 and 7/16
	found := false
	for _, r := range results {
		if r.Source == "units" && r.Title == "0.4 inch (between 3/8 and 7/16 inch)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '0.4 inch (between 3/8 and 7/16 inch)', got: %v", titlesOf(results))
	}
}

func TestUnitsProviderInchFraction_Negative(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("-10.16mm")
	found := false
	for _, r := range results {
		if r.Source == "units" && r.Title == "-0.4 inch (between -7/16 and -3/8 inch)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '-0.4 inch (between -7/16 and -3/8 inch)', got: %v", titlesOf(results))
	}
}

func TestUnitsProviderInchFraction_MixedNumber(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("31.75mm") // = 1.25 inch exactly => 1 1/4
	found := false
	for _, r := range results {
		if r.Source == "units" && r.Title == "1.25 inch (1 1/4 inch)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '1.25 inch (1 1/4 inch)', got: %v", titlesOf(results))
	}
}

func TestUnitsProviderInchFraction_Zero(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("0mm")
	found := false
	for _, r := range results {
		if r.Source == "units" && r.Title == "0 inch (0 inch)" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '0 inch (0 inch)', got: %v", titlesOf(results))
	}
}

func TestUnitsProviderInchFraction_AreaAndVolumeUnaffected(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)

	results := p.Query("1sqm")
	for _, r := range results {
		if strings.Contains(r.Title, "in²") && strings.Contains(r.Title, "(") {
			t.Errorf("expected no fractional suffix on in² result, got: %v", r.Title)
		}
	}

	results = p.Query("1cbm")
	for _, r := range results {
		if strings.Contains(r.Title, "in³") && strings.Contains(r.Title, "(") {
			t.Errorf("expected no fractional suffix on in³ result, got: %v", r.Title)
		}
	}
}

func TestUnitsProviderInchFraction_OtherLengthUnitsUnaffected(t *testing.T) {
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, nil)
	results := p.Query("10.16mm")
	for _, r := range results {
		if r.Source != "units" {
			continue
		}
		if strings.HasSuffix(r.Title, " millimeter") || strings.Contains(r.Title, " meter") ||
			strings.Contains(r.Title, " centimeter") || strings.Contains(r.Title, " foot") {
			if strings.Contains(r.Title, "(") {
				t.Errorf("expected no fractional suffix on non-inch result, got: %v", r.Title)
			}
		}
	}
}

func titlesOf(results []launcher.Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Title
	}
	return out
}
