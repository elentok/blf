package currency

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	primaryURL   = "https://open.er-api.com/v6/latest/USD"
	fallback1URL = "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json"
	fallback2URL = "https://latest.currency-api.pages.dev/v1/currencies/usd.json"
	defaultTTL   = 12 * time.Hour
)

// Rates holds exchange rates with USD as the base (1 USD = N <code>).
type Rates struct {
	USD        map[string]float64
	NextUpdate time.Time
}

// Cache manages in-memory and on-disk currency rates.
type Cache struct {
	mu        sync.RWMutex
	current   *Rates
	cachePath string
	fetcher   func(url string) ([]byte, error)
	now       func() time.Time
}

// NewCache creates a Cache backed by cachePath. If fetcher is nil, a default
// HTTP client is used. If now is nil, time.Now is used. Existing disk cache is
// loaded eagerly (errors are silently ignored).
func NewCache(cachePath string, fetcher func(string) ([]byte, error), now func() time.Time) *Cache {
	if fetcher == nil {
		fetcher = defaultHTTPGet
	}
	if now == nil {
		now = time.Now
	}
	c := &Cache{cachePath: cachePath, fetcher: fetcher, now: now}
	_ = c.loadFromDisk()
	return c
}

// Rates returns the current in-memory rates, or nil if none are loaded.
func (c *Cache) Rates() *Rates {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// NeedsRefresh reports whether the cache is empty or has passed its TTL.
func (c *Cache) NeedsRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.current == nil {
		return true
	}
	return c.now().After(c.current.NextUpdate)
}

// TTL returns the duration until the next scheduled refresh.
// Returns 0 if a refresh is already overdue.
func (c *Cache) TTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.current == nil {
		return 0
	}
	d := c.current.NextUpdate.Sub(c.now())
	if d < 0 {
		return 0
	}
	return d
}

// Fetch fetches fresh rates from the API (primary + fallbacks) and updates
// both the in-memory cache and the on-disk cache file.
// On fetch failure, stale in-memory rates (if any) are preserved.
func (c *Cache) Fetch() error {
	rates, err := c.fetchFromAPI()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.current = rates
	c.mu.Unlock()
	_ = c.saveToDisk(rates)
	return nil
}

// --- disk ---

type diskCache struct {
	Rates          map[string]float64 `json:"rates"`
	NextUpdateUnix int64              `json:"next_update_unix"`
}

func (c *Cache) loadFromDisk() error {
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return err
	}
	var dc diskCache
	if err := json.Unmarshal(data, &dc); err != nil {
		return err
	}
	if len(dc.Rates) == 0 {
		return fmt.Errorf("empty rates in cache")
	}
	c.current = &Rates{
		USD:        dc.Rates,
		NextUpdate: time.Unix(dc.NextUpdateUnix, 0),
	}
	return nil
}

func (c *Cache) saveToDisk(r *Rates) error {
	if err := os.MkdirAll(filepath.Dir(c.cachePath), 0o755); err != nil {
		return err
	}
	dc := diskCache{
		Rates:          r.USD,
		NextUpdateUnix: r.NextUpdate.Unix(),
	}
	data, err := json.Marshal(dc)
	if err != nil {
		return err
	}
	return os.WriteFile(c.cachePath, data, 0o644)
}

// --- API fetch ---

type openERResponse struct {
	Result             string             `json:"result"`
	Rates              map[string]float64 `json:"rates"`
	TimeNextUpdateUnix int64              `json:"time_next_update_unix"`
}

type fawazResponse struct {
	USD map[string]float64 `json:"usd"`
}

func (c *Cache) fetchFromAPI() (*Rates, error) {
	// Primary: open.er-api.com
	if data, err := c.fetcher(primaryURL); err == nil {
		var r openERResponse
		if jsonErr := json.Unmarshal(data, &r); jsonErr == nil && r.Result == "success" && len(r.Rates) > 0 {
			var nextUpdate time.Time
			if r.TimeNextUpdateUnix > 0 {
				nextUpdate = time.Unix(r.TimeNextUpdateUnix, 0)
			} else {
				nextUpdate = c.now().Add(defaultTTL)
			}
			return &Rates{USD: normalizeKeys(r.Rates), NextUpdate: nextUpdate}, nil
		}
	}

	// Fallbacks: fawazahmed0 CDN + mirror
	for _, url := range []string{fallback1URL, fallback2URL} {
		if data, err := c.fetcher(url); err == nil {
			var r fawazResponse
			if jsonErr := json.Unmarshal(data, &r); jsonErr == nil && len(r.USD) > 0 {
				return &Rates{USD: normalizeKeys(r.USD), NextUpdate: c.now().Add(defaultTTL)}, nil
			}
		}
	}

	return nil, fmt.Errorf("all currency sources unavailable")
}

func normalizeKeys(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[strings.ToUpper(k)] = v
	}
	return out
}

func defaultHTTPGet(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
