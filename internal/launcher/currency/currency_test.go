package currency_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elentok/blf/internal/launcher/currency"
)

func TestFetchPrimary(t *testing.T) {
	primary := openERResponse(map[string]float64{"USD": 1, "EUR": 0.92, "GBP": 0.79}, 9999)
	fetcher := func(url string) ([]byte, error) {
		if url == "https://open.er-api.com/v6/latest/USD" {
			return primary, nil
		}
		return nil, errors.New("unexpected URL")
	}
	c := currency.NewCache("", fetcher, fixedNow(1000))

	if err := c.Fetch(); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	rates := c.Rates()
	if rates == nil {
		t.Fatal("Rates is nil after fetch")
	}
	if rates.USD["EUR"] != 0.92 {
		t.Errorf("EUR rate: got %v, want 0.92", rates.USD["EUR"])
	}
	if rates.NextUpdate.Unix() != 9999 {
		t.Errorf("NextUpdate: got %v, want 9999", rates.NextUpdate.Unix())
	}
}

func TestFetchFallback(t *testing.T) {
	fallback := fawazResponseJSON(map[string]float64{"usd": 1, "eur": 0.91, "gbp": 0.78})
	fetcher := func(url string) ([]byte, error) {
		if url == "https://open.er-api.com/v6/latest/USD" {
			return nil, errors.New("primary down")
		}
		if url == "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json" {
			return fallback, nil
		}
		return nil, errors.New("unexpected URL")
	}
	c := currency.NewCache("", fetcher, fixedNow(5000))

	if err := c.Fetch(); err != nil {
		t.Fatalf("Fetch with fallback: %v", err)
	}

	rates := c.Rates()
	if rates == nil {
		t.Fatal("Rates is nil after fallback fetch")
	}
	if rates.USD["EUR"] != 0.91 {
		t.Errorf("EUR rate from fallback: got %v, want 0.91", rates.USD["EUR"])
	}
}

func TestStaleCacheOnFailure(t *testing.T) {
	staleRates := map[string]float64{"USD": 1, "EUR": 0.88}
	tmp := writeCache(t, staleRates, 100) // TTL already expired at t=5000

	fetcher := func(url string) ([]byte, error) {
		return nil, errors.New("network unavailable")
	}
	c := currency.NewCache(tmp, fetcher, fixedNow(5000))

	// Rates loaded from disk despite expired TTL
	rates := c.Rates()
	if rates == nil {
		t.Fatal("stale rates not loaded from disk")
	}
	if rates.USD["EUR"] != 0.88 {
		t.Errorf("stale EUR: got %v, want 0.88", rates.USD["EUR"])
	}

	// Fetch fails but stale rates are preserved
	err := c.Fetch()
	if err == nil {
		t.Fatal("expected fetch error, got nil")
	}
	if c.Rates().USD["EUR"] != 0.88 {
		t.Error("stale rates should be preserved after failed fetch")
	}
}

func TestTTL(t *testing.T) {
	fetcher := func(url string) ([]byte, error) {
		return openERResponse(map[string]float64{"USD": 1}, 2000), nil
	}
	now := fixedNow(1000)
	c := currency.NewCache("", fetcher, now)
	_ = c.Fetch()

	ttl := c.TTL()
	want := time.Duration(1000) * time.Second // 2000 - 1000
	if ttl != want {
		t.Errorf("TTL: got %v, want %v", ttl, want)
	}
}

func TestCurrencyKeyNormalization(t *testing.T) {
	// fawazahmed0 returns lowercase keys; they must be normalized to uppercase
	fallback := fawazResponseJSON(map[string]float64{"usd": 1, "eur": 0.92, "jpy": 149.5})
	fetcher := func(url string) ([]byte, error) {
		if url == "https://open.er-api.com/v6/latest/USD" {
			return nil, errors.New("primary down")
		}
		return fallback, nil
	}
	c := currency.NewCache("", fetcher, fixedNow(0))
	_ = c.Fetch()

	if c.Rates().USD["EUR"] != 0.92 {
		t.Error("EUR not normalized to uppercase key")
	}
	if c.Rates().USD["JPY"] != 149.5 {
		t.Error("JPY not normalized to uppercase key")
	}
}

func TestCurrencyCrossRateComputation(t *testing.T) {
	// Cross-rate: EUR→GBP = EUR/USD rate / GBP/USD rate
	// 1 USD = 0.92 EUR, 1 USD = 0.79 GBP
	// So 1 EUR = 0.79/0.92 GBP ≈ 0.8587
	rates := &currency.Rates{
		USD: map[string]float64{
			"USD": 1,
			"EUR": 0.92,
			"GBP": 0.79,
		},
	}

	// 50 EUR → USD: 50 / 0.92
	eurToUSD := 50 * rates.USD["USD"] / rates.USD["EUR"]
	want := 50 / 0.92
	if math.Abs(eurToUSD-want) > 1e-10 {
		t.Errorf("50 EUR→USD: got %v, want %v", eurToUSD, want)
	}

	// 50 EUR → GBP: 50 * GBP / EUR
	eurToGBP := 50 * rates.USD["GBP"] / rates.USD["EUR"]
	wantGBP := 50 * 0.79 / 0.92
	if math.Abs(eurToGBP-wantGBP) > 1e-10 {
		t.Errorf("50 EUR→GBP: got %v, want %v", eurToGBP, wantGBP)
	}
}

// --- helpers ---

func fixedNow(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}

func openERResponse(rates map[string]float64, nextUpdateUnix int64) []byte {
	data, _ := json.Marshal(map[string]any{
		"result":               "success",
		"rates":                rates,
		"time_next_update_unix": nextUpdateUnix,
	})
	return data
}

func fawazResponseJSON(rates map[string]float64) []byte {
	data, _ := json.Marshal(map[string]any{"usd": rates})
	return data
}

func writeCache(t *testing.T, rates map[string]float64, nextUpdateUnix int64) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "currency.json")
	data, _ := json.Marshal(map[string]any{
		"rates":            rates,
		"next_update_unix": nextUpdateUnix,
	})
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
