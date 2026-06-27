package launcher

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/units"
)

// RatesFetchedMsg is emitted by FetchRatesCmd when the currency fetch completes.
type RatesFetchedMsg struct{ Err error }

// RatesTTLMsg is emitted when the currency TTL expires and a re-fetch is due.
type RatesTTLMsg struct{}

// FetchRatesCmd returns a tea.Cmd that fetches fresh currency rates.
func FetchRatesCmd(cache *currency.Cache) tea.Cmd {
	return func() tea.Msg {
		err := cache.Fetch()
		return RatesFetchedMsg{Err: err}
	}
}

// ScheduleRatesTick returns a tea.Cmd that sleeps for d then emits RatesTTLMsg.
func ScheduleRatesTick(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		<-time.After(d)
		return RatesTTLMsg{}
	}
}

// currencySymbols maps single-character/special currency symbols to ISO codes.
var currencySymbols = map[string]string{
	"$": "USD",
	"€": "EUR",
	"£": "GBP",
	"¥": "JPY",
	"₹": "INR",
}

// UnitsProvider is a Provider that handles unit conversions and currency exchange.
type UnitsProvider struct {
	registry   *units.Registry
	currency   *currency.Cache
	currencies []string // ordered ISO codes to show in currency results
}

var _ Provider = (*UnitsProvider)(nil)

// NewUnitsProvider creates a UnitsProvider. currencyCache may be nil to disable currency.
// currencies is the ordered list of ISO codes to show; nil produces no currency results.
func NewUnitsProvider(registry *units.Registry, currencyCache *currency.Cache, currencies []string) *UnitsProvider {
	return &UnitsProvider{registry: registry, currency: currencyCache, currencies: currencies}
}

func (p *UnitsProvider) Query(input string) []Result {
	if Classify(input) != Computational {
		return nil
	}

	value, sym, ok := units.ParseInput(input)
	if !ok {
		return nil
	}

	// Static unit lookup
	if group, unit, found := p.registry.Lookup(sym); found {
		return p.convertUnit(value, unit, group)
	}

	// Currency lookup
	return p.convertCurrency(value, sym)
}

func (p *UnitsProvider) convertUnit(value float64, from *units.Unit, group *units.Group) []Result {
	convs := p.registry.Convert(value, from, group)
	results := make([]Result, 0, len(convs))
	for _, c := range convs {
		formatted := FormatNumber(roundSigFigs(c.Value, 12))
		results = append(results, Result{
			Title:    formatted + " " + c.Unit.Name,
			Subtitle: FormatNumber(value) + " " + from.Name,
			Icon:     IconRoleUnit,
			Source:   "units",
			Weight:   1.5,
			Action:   Action{Type: ActionCopy, Target: formatted},
		})
	}
	return results
}

func (p *UnitsProvider) convertCurrency(value float64, sym string) []Result {
	if p.currency == nil {
		return nil
	}

	// Resolve symbol to ISO code
	upper := resolveToISO(sym)

	// If we don't know if this is a currency, only guess on 3-letter alpha codes or known symbols
	if !looksLikeCurrencyCode(sym) && upper == sym {
		return nil
	}

	rates := p.currency.Rates()
	if rates == nil {
		return []Result{{
			Title:  "loading rates…",
			Icon:   IconRoleLoading,
			Source: "currency",
		}}
	}

	fromRate, ok := rates.USD[upper]
	if !ok || fromRate == 0 {
		return nil
	}

	results := make([]Result, 0, len(p.currencies))
	for _, code := range p.currencies {
		if code == upper {
			continue
		}
		toRate, ok := rates.USD[code]
		if !ok || toRate == 0 {
			continue
		}
		// value in `upper` → USD → `code`
		// 1 upper = (1/fromRate) USD; 1 USD = toRate code
		converted := value * toRate / fromRate
		formatted := formatCurrencyAmount(converted)
		results = append(results, Result{
			Title:     formatted + " " + strings.ToLower(code),
			Icon:      IconRoleCurrency,
			IconGlyph: CurrencyIcons[code],
			Source:    "currency",
			Weight:    1.5,
			Action:    Action{Type: ActionCopy, Target: formatted},
		})
	}
	return results
}

func resolveToISO(sym string) string {
	if code, ok := currencySymbols[sym]; ok {
		return code
	}
	return strings.ToUpper(sym)
}

func looksLikeCurrencyCode(sym string) bool {
	if _, ok := currencySymbols[sym]; ok {
		return true
	}
	upper := strings.ToUpper(sym)
	if len([]rune(upper)) != 3 {
		return false
	}
	for _, c := range upper {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// roundSigFigs rounds v to n significant figures to eliminate floating point noise.
func roundSigFigs(v float64, n int) float64 {
	if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	d := math.Ceil(math.Log10(math.Abs(v)))
	pow := math.Pow(10, float64(n)-d)
	return math.Round(v*pow) / pow
}

// formatCurrencyAmount formats a currency value to a readable number of decimal places.
func formatCurrencyAmount(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return FormatNumber(v)
	}
	abs := math.Abs(v)
	var s string
	switch {
	case abs == 0:
		s = "0.00"
	case abs >= 100:
		s = fmt.Sprintf("%.2f", v)
	case abs >= 1:
		s = fmt.Sprintf("%.4f", v)
	default:
		s = fmt.Sprintf("%.6f", v)
	}
	// strip trailing zeros after decimal point (but keep at least 2 for amounts >= 1)
	return s
}
