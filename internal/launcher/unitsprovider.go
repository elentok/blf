package launcher

import (
	"fmt"
	"math"
	"strconv"
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
	"₪": "ILS",
}

// codeSymbols is the reverse of currencySymbols (ISO code → symbol), used to
// render history hints with symbols where available.
var codeSymbols = func() map[string]string {
	m := make(map[string]string, len(currencySymbols))
	for sym, code := range currencySymbols {
		m[code] = sym
	}
	return m
}()

// currencyDisplay returns a currency's symbol if one exists, else its lowercase ISO code.
func currencyDisplay(code string) string {
	if sym, ok := codeSymbols[code]; ok {
		return sym
	}
	return strings.ToLower(code)
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

// Hint returns "= <joined currency line>" for a currency query, or "" otherwise.
// Non-currency unit conversions, unrecognized symbols, and not-yet-loaded rates
// all yield no hint.
func (p *UnitsProvider) Hint(query string) string {
	if Classify(query) != Computational {
		return ""
	}
	value, sym, ok := units.ParseInput(query)
	if !ok {
		return ""
	}
	// Non-currency unit conversions get no hint (too many targets to join).
	if _, _, found := p.registry.Lookup(sym); found {
		return ""
	}
	if p.currency == nil {
		return ""
	}
	upper := resolveToISO(sym)
	if !looksLikeCurrencyCode(sym) && upper == sym {
		return ""
	}
	rates := p.currency.Rates()
	if rates == nil {
		return ""
	}
	fromRate, ok := rates.USD[upper]
	if !ok || fromRate == 0 {
		return ""
	}
	parts := make([]string, 0, len(p.currencies))
	for _, code := range p.currencies {
		if code == upper {
			continue
		}
		toRate, ok := rates.USD[code]
		if !ok || toRate == 0 {
			continue
		}
		converted := value * toRate / fromRate
		parts = append(parts, formatCurrencyAmount(converted)+" "+currencyDisplay(code))
	}
	if len(parts) == 0 {
		return ""
	}
	return "= " + strings.Join(parts, ", ")
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
		rounded := roundSigFigs(c.Value, 12)
		formatted := FormatNumber(rounded)
		title := formatted + " " + c.Unit.Name
		if c.Group.Name == "length" && c.Unit.Name == "inch" {
			title += " (" + formatInchFraction(rounded) + ")"
		}
		results = append(results, Result{
			Title:    title,
			Subtitle: FormatNumber(value) + " " + from.Name,
			Icon:     IconRoleUnit,
			Source:   "units",
			Weight:   1.5,
			Action:   Action{Type: ActionCopy, Target: formatted},
		})
	}
	return results
}

// formatInchFraction renders v (a decimal inch value) as a fractional readout:
// the nearest 1/16" mark when v is within 1/64" of it, or the enclosing range
// of 1/16" marks otherwise. The sign is applied once, from v's absolute value.
func formatInchFraction(v float64) string {
	const epsilon = 1e-9

	neg := v < 0
	abs := math.Abs(v)
	sixteenths := abs * 16
	nearest := math.Round(sixteenths)
	dist := math.Abs(sixteenths - nearest)

	if dist <= 0.25+epsilon {
		prefix := ""
		if dist > epsilon {
			prefix = "~"
		}
		if neg {
			prefix += "-"
		}
		return prefix + renderSixteenths(int64(nearest)) + " inch"
	}

	lower := renderSixteenths(int64(math.Floor(sixteenths)))
	upper := renderSixteenths(int64(math.Floor(sixteenths)) + 1)
	if neg {
		lower, upper = "-"+upper, "-"+lower
	}
	return "between " + lower + " and " + upper + " inch"
}

// renderSixteenths renders a nonnegative count of 1/16" units as a reduced
// fraction, or a mixed number when the magnitude is at least 1 inch.
func renderSixteenths(n int64) string {
	whole := n / 16
	rem := n % 16
	if rem == 0 {
		return strconv.FormatInt(whole, 10)
	}
	g := gcd(rem, 16)
	num, den := rem/g, 16/g
	if whole == 0 {
		return fmt.Sprintf("%d/%d", num, den)
	}
	return fmt.Sprintf("%d %d/%d", whole, num, den)
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
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
	if abs == 0 {
		return "0"
	}
	var s string
	switch {
	case abs >= 100:
		s = fmt.Sprintf("%.2f", v)
	case abs >= 1:
		s = fmt.Sprintf("%.4f", v)
	default:
		s = fmt.Sprintf("%.6f", v)
	}
	// strip trailing zeros after the decimal point (but keep at least 2 for amounts >= 1)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if abs >= 1 {
		dot := strings.IndexByte(s, '.')
		if dot < 0 {
			s += ".00"
		} else if decimals := len(s) - dot - 1; decimals < 2 {
			s += strings.Repeat("0", 2-decimals)
		}
	}
	return s
}
