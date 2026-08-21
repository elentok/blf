package launcher_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/units"
)

// loadedCache returns a currency.Cache with the given USD-based rates loaded.
func loadedCache(t *testing.T, rates map[string]float64) *currency.Cache {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"result":                "success",
		"rates":                 rates,
		"time_next_update_unix": int64(1 << 40), // far future, never stale
	})
	fetcher := func(string) ([]byte, error) { return body, nil }
	c := currency.NewCache("", fetcher, nil)
	if err := c.Fetch(); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return c
}

func TestCalcProviderHint(t *testing.T) {
	p := launcher.CalcProvider{}
	if got := p.Hint("10+20"); got != "= 30" {
		t.Errorf("Hint(\"10+20\") = %q, want \"= 30\"", got)
	}
	// Non-math: bare number, plain text, and unit/currency inputs get no calc hint.
	for _, q := range []string{"1000000", "1Password", "10cm", "1$", ""} {
		if got := p.Hint(q); got != "" {
			t.Errorf("Hint(%q) = %q, want \"\"", q, got)
		}
	}
}

// TestCalcProviderQuery_FractionUnit checks that CalcProvider suppresses its
// "Invalid math expression" row for valid fraction/mixed-number unit input
// (that's UnitsProvider's job), but still shows it for fraction input that's
// malformed as both math and a unit.
func TestCalcProviderQuery_FractionUnit(t *testing.T) {
	p := launcher.CalcProvider{}

	for _, q := range []string{"3/8 in", "1 1/4 in"} {
		if got := p.Query(q); got != nil {
			t.Errorf("Query(%q) = %v, want nil (suppressed for UnitsProvider)", q, got)
		}
	}

	for _, q := range []string{"3/ in", "/8 in", "3/0 in"} {
		got := p.Query(q)
		if len(got) != 1 || got[0].Title != "Invalid math expression" {
			t.Errorf("Query(%q) = %v, want a single \"Invalid math expression\" row", q, got)
		}
	}
}

func TestUnitsProviderHint_Currency(t *testing.T) {
	cache := loadedCache(t, map[string]float64{"USD": 1, "ILS": 2.987, "GBP": 0.79, "EUR": 0.92})
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), cache, []string{"ILS", "GBP", "EUR", "USD"})

	// Symbols where available (₪, £, €), source currency (USD) skipped, no trailing zeros.
	if got := p.Hint("1$"); got != "= 2.987 ₪, 0.79 £, 0.92 €" {
		t.Errorf("Hint(\"1$\") = %q", got)
	}

	// ₪ parses as ILS input: source ILS is skipped, the rest render with symbols.
	got := p.Hint("1₪")
	if got == "" {
		t.Fatal("Hint(\"1₪\") returned no hint; ₪ should parse as ILS")
	}
	for _, want := range []string{"= ", " £", " €", " $"} {
		if !strings.Contains(got, want) {
			t.Errorf("Hint(\"1₪\") = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "₪") {
		t.Errorf("Hint(\"1₪\") = %q, should skip the source currency (ILS)", got)
	}
}

func TestUnitsProviderHint_NoHint(t *testing.T) {
	cache := loadedCache(t, map[string]float64{"USD": 1, "ILS": 2.987})
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), cache, []string{"ILS", "USD"})

	// Non-currency unit conversions and pure math get no units hint.
	for _, q := range []string{"10cm", "5kg", "1+2", "hello"} {
		if got := p.Hint(q); got != "" {
			t.Errorf("Hint(%q) = %q, want \"\"", q, got)
		}
	}
}

func TestUnitsProviderHint_RatesNotLoaded(t *testing.T) {
	// nil cache and an unfetched cache both yield no currency hint.
	regHint := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), nil, []string{"ILS"}).Hint("1$")
	if regHint != "" {
		t.Errorf("nil-cache Hint(\"1$\") = %q, want \"\"", regHint)
	}
	unfetched := currency.NewCache("", func(string) ([]byte, error) { return nil, nil }, nil)
	p := launcher.NewUnitsProvider(units.NewBuiltinRegistry(), unfetched, []string{"ILS"})
	if got := p.Hint("1$"); got != "" {
		t.Errorf("unloaded-cache Hint(\"1$\") = %q, want \"\"", got)
	}
}
