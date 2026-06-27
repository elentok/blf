package launcher_test

import (
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

func titlesOf(results []launcher.Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Title
	}
	return out
}
