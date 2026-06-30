# Launcher history hints

Show a dimmed-italic computed result (a **history hint**) beside math and currency
entries in the launcher history list, rendered as the row's subtitle (`= <result>`).

## Decisions (from grilling session)

- **Live, not stored.** History file stays plain query strings; hints are computed
  when the empty-input history list is built. Reflects current rates each time.
- **Cap history at 30** (`MaxEntries` 500 → 30). With ≤30 entries the hint cost is
  sub-millisecond on a cold path (list rebuilds only when input returns to empty,
  never while typing), so compute **eagerly** — no lazy/memo machinery.
- **Subtitle format:** Title stays the raw query; Subtitle is `"= " + value`, so the
  row reads `10+20 = 30` with `= 30` dimmed (reuses existing `SubtitleStyle`).
- **Scope:** math expressions and currency only. Non-currency unit conversions,
  bare numbers, and non-resolving queries get no hint.
- **Currency line:** reuse `formatCurrencyAmount` + configured `currencies` order,
  skip the source currency, join with `, `. Show **symbol** where available, else
  lowercase ISO code. Symbol map = reverse of `currencySymbols` + `ILS → ₪`.
- **Also** add `₪ → ILS` to the input `currencySymbols` map (so `100₪` parses).
- **Wiring:** optional `HintProvider { Hint(query string) string }` interface;
  `CalcProvider` and `UnitsProvider` implement it; model uses first non-empty hint.
- **Edge cases:** rates not loaded → no hint; calc failure / plain text → no hint.

## Tasks

- [x] Drop `history.MaxEntries` from 500 to 30
- [x] Add `HintProvider` interface in `provider.go`
- [x] Add `₪ → ILS` to `currencySymbols`; build code→symbol display map (incl. `ILS→₪`)
- [x] Implement `CalcProvider.Hint` (math expression → `= <value>`, else `""`)
- [x] Implement `UnitsProvider.Hint` (currency → joined symbol line; unit/no-rates → `""`)
- [x] In `model.go` history-list block, set each row's `Subtitle` from first non-empty `Hint`
- [x] Tests: calc hint, currency symbol-joined hint, ILS/₪, no hint for units/bare/plain, no-rates
- [x] Update CHANGELOG.md
