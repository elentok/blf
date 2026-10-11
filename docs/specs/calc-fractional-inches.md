# Calculator fractional inches

## Problem Statement

The launcher's calculator already understands unit-quantity queries like `3.5 in` and converts them
to other length units. But inches are conventionally written and read as fractions — tape measures,
lumber, drill bits, and most everyday inch measurements are `3/8 in`, `1 1/4 in`, `3/4 inch` — never
as decimals. Today typing a fraction before `in` fails to parse as a unit query at all, so the one
unit where fractional notation matters most is the one the calculator can't read.

## Solution

`UnitsProvider`'s quantity parsing (`units.ParseInput`) learns to read a fraction or mixed number
(whole number + fraction) in front of a unit symbol, in addition to the plain decimals it already
accepts:

- `3/8 in` → 0.375 inch
- `1 1/4 in` → 1.25 inch
- `3/4 inch` → 0.75 inch
- `1/4in` (no space) → 0.25 inch — matching how `1.25in` already needs no space today

This is a change to the shared numeric-parsing stage, not an inch-specific branch — any unit symbol
works after a fraction or mixed number (`3/8 cm` parses the same way `3/8 in` does), matching how
plain decimals already work for any unit today.

`router.go`'s `matchesNumberUnit` — a separate, simpler shape-check used to suppress `CalcProvider`'s
math-error row for unit-looking input — learns the same fraction/mixed-number shape (see
Implementation Decisions), so `3/8 in` shows only the unit conversion, not a bogus "Invalid math
expression" row alongside it.

Converting a value *to* inches additionally shows a fractional readout alongside the decimal one:

- If the decimal is within 1/64 inch of some 1/16 mark, show that single fraction:
  `0.0625 inch (~1/16 inch)`, or without `~` when it's an exact 1/16 multiple: `0.375 inch (3/8
  inch)`.
- Otherwise (the decimal sits nearer the midpoint between two 1/16 marks than either mark) show the
  enclosing range instead of a falsely-precise single fraction: `0.4 inch (between 3/8 and 7/16
  inch)`.

Every other unit in the length group keeps its current decimal-only formatting.

## User Stories

1. As a launcher user, I want to type `3/8 in` and get the same conversions as `0.375 in`, so that I
   can enter a measurement the way I'd read it off a tape measure.
2. As a launcher user, I want to type a mixed number like `1 1/4 in`, so that I can enter measurements
   larger than one inch without converting to a decimal in my head first.
3. As a launcher user, I want `3/4 inch` and `3/4 inches` to parse the same as `3/4 in`, so that the
   fraction support covers every existing inch spelling.
4. As a launcher user, I want `1/4in` (no space before the unit) to parse the same as `1/4 in`, so
   that fractions aren't held to a stricter spacing rule than decimals already are.
5. As a launcher user, I want extra whitespace in a mixed number (`1   1/4 in`) to still parse, so
   that incidental extra spaces don't break an otherwise-valid measurement.
6. As a launcher user, I want `3/8 in` to still divide (`3` divided by `8`) when there's no unit
   attached, so that the plain calculator's existing `/` division isn't affected by this change.
7. As a launcher user, I want a negative fractional measurement like `-3/8 in` to parse correctly, so
   that the feature doesn't silently misparse valid negative input.
8. As a launcher user, I want malformed fraction-like input (`3/ in`, `/8 in`, `1 / in`, `3/0 in`) to
   fail to parse as a unit query, so that garbage input doesn't produce a garbage conversion — I
   should see the same thing I see today for other bare-`/` input that isn't valid math or a unit
   (e.g. `3/xyz`): no app/script fuzzy list, just `CalcProvider`'s normal "Invalid math expression"
   row, with no extra bogus unit-conversion row alongside it.
9. As a launcher user, when I convert some other unit to inches, I want the inch result shown as both
   a decimal and its nearest 1/16 fraction, so that I can read off the closest mark on a tape measure
   without doing the math myself.
10. As a launcher user, I want the fractional readout to drop the `~` when the decimal is an exact
    1/16 multiple, so that I can tell at a glance whether the fraction is exact or approximate.
11. As a launcher user, when a converted value sits close to the midpoint between two 1/16 marks
    rather than close to either mark, I want to see the enclosing range (`between 3/8 and 7/16 inch`)
    instead of a single fraction, so that the readout never implies more precision than the
    conversion actually has.
12. As a launcher user, I want a negative inch result's fractional readout to show the sign once,
    before the fraction or range (`-0.375 inch (~-3/8 inch)`, `-0.4 inch (between -7/16 and -3/8
    inch)`), so that the sign is unambiguous and not duplicated or misplaced.
13. As a launcher user, I want every other converted unit (mm, cm, ft, m, ...) to keep showing plain
    decimals, so that the new fractional formatting doesn't clutter results I don't need it for.
14. As a launcher user, I want area/volume inch units (`in²`, `in³`) to keep their current decimal-only
    formatting, so that the fractional readout only ever applies to linear inches, where "nearest
    1/16 on a tape measure" actually means something.

## Implementation Decisions

- **Seam**: `units.ParseInput` (`internal/launcher/units/units.go`) is the seam for the input side. It
  currently scans a decimal number, then a unit symbol. It grows to also recognize, in place of the
  plain decimal it already scans:
  - a bare fraction: `<int>/<int>`
  - a mixed number: `<int><whitespace><int>/<int>` — one-or-more whitespace characters between the
    whole part and the fraction (tolerating extra spaces/tabs, not just exactly one space)

  Zero or more whitespace characters are allowed between the fraction and the following unit symbol,
  exactly like the existing decimal grammar (`1.25in` and `1.25 in` both already work; `1/4in` and
  `1/4 in` should both work the same way).

  The whole-number-only case (`3 in`) and the plain-decimal case (`3.5 in`) are unaffected — those
  code paths are unchanged.
- Division stays exclusively `calc.Evaluate`'s job (`internal/launcher/calc/calc.go`). `ParseInput`
  is only ever invoked by `UnitsProvider`. Note that `Classify` (`router.go`) routes `3/8` (bare, no
  unit) to `Computational` purely because it contains an unambiguous math operator (`/`), not because
  it recognized a unit shape — `UnitsProvider.Query`/`Hint` still run on it, but `units.ParseInput`
  rejects it (no unit symbol follows the fraction) and returns no results, so `calc.Evaluate`'s
  division answer is the only thing shown. This is existing behavior and needs no change.
- Fraction parsing only accepts a single `/` with digits (no `.`) on both sides and a nonzero
  denominator; anything else (missing numerator, missing denominator, a second `/`, denominator `0`)
  causes `ParseInput` to return `ok=false`, same as any other unparseable input today.
- **Seam**: `matchesNumberUnit` (`internal/launcher/router.go`) is the seam that keeps `CalcProvider`
  from showing a spurious "Invalid math expression" row next to a valid unit conversion. It currently
  recognizes `<number><unit>`/`<number> <unit>` shapes only. It grows to recognize the same
  fraction/mixed-number grammar `units.ParseInput` recognizes (bare fraction, mixed number, optional
  surrounding whitespace, rejecting zero-denominator and other malformed shapes) — the two functions'
  grammars must stay in sync, since a mismatch is exactly the bug this seam exists to prevent. For
  genuinely malformed input (`3/ in`, `3/0 in`), `matchesNumberUnit` correctly returns `false`, so
  `CalcProvider` falls through to its existing "Invalid math expression" row and `UnitsProvider`
  produces nothing — matching today's behavior for other bare-`/` non-unit, non-math input.
- **Seam**: `UnitsProvider.convertUnit` (`internal/launcher/unitsprovider.go`) is the seam for the
  output side. When the *target* unit (`c.Unit`) is the linear inch unit specifically — a pointer
  comparison against the one `Unit` the length group registers for `in`/`inch`/`inches`, which
  excludes the separate `in²`/`in³` area/volume units entirely — the formatted title gains a
  parenthetical fractional readout after the existing decimal:
  `formatted + " " + c.Unit.Name + " (" + fractionSuffix + ")"`. Every other unit's title format,
  including `in²`/`in³`, is unchanged.
- The fractional readout is computed from the same value already used for display —
  `roundSigFigs(c.Value, 12)` (the value `formatted` is derived from), not the raw pre-rounding
  float — so floating-point noise from the unit conversion itself can't cause a should-be-exact 1/16
  boundary to render as approximate (or vice versa).
- A new helper (name TBD by the implementer, e.g. `formatInchFraction`) takes that rounded decimal
  inch value and:
  1. Finds the nearest 1/16 mark, rounding half away from zero on an exact tie (e.g. a value exactly
     1/32 away from two marks rounds to the farther-from-zero one).
  2. If the decimal is within 1/64 inch of that mark, renders it as a single reduced fraction (e.g.
     8/16 → 1/2), as a mixed number when the magnitude is ≥ 1 inch (`1 1/4 inch`, not `5/4 inch`), and
     reports whether the distance was exactly 0 (no `~`) or not (prefix `~`).
  3. Otherwise (more than 1/64 inch from the nearest mark — i.e. closer to the midpoint between two
     marks than to either) renders the enclosing range instead: `"between " + lower + " and " + upper
     + " inch"`, using the same reduced-fraction/mixed-number rendering for `lower` and `upper`.
  4. Applies the sign once to the whole result — a negative value's fraction or range is rendered
     from its absolute value, then prefixed with `-` (`-0.375 inch (~-3/8 inch)`, `-0.4 inch (between
     -7/16 and -3/8 inch)`), never nested inside the numerator or the range's inner values.

  No special-casing for zero (`0 inch (0 inch)` renders through the same exact-match path — 0 is
  0/16, "close enough," and the ~ is omitted) or for very large magnitudes (a huge mixed number is
  still rendered in full; this feature doesn't bound the input's magnitude).
- The `Subtitle` echo of the input (currently `FormatNumber(value) + " " + from.Name`, e.g.
  `3.5 inch`) keeps showing the parsed decimal value, not the original fraction text the user typed —
  consistent with how it already normalizes `3.50 in` down to `3.5 inch` today.

## Testing Decisions

- Only test external behavior (`ParseInput`'s returned `(value, symbol, ok)`, `matchesNumberUnit`'s
  returned bool, and `UnitsProvider`'s produced `Result`s), not the internal fraction-reduction
  helper's intermediate representation.
- `internal/launcher/units/units_test.go` already table-tests `ParseInput` against strings like
  `"10kg"`, `"5 km"`, `"50$"` — extend that table with fraction and mixed-number cases (`"3/8 in"`,
  `"1/4in"`, `"1 1/4 in"`, `"1   1/4 in"`, `"-3/8 in"`) and malformed cases expecting `ok=false`
  (`"3/ in"`, `"/8 in"`, `"3/0 in"`).
- Add an equivalent table-test extension (or a new table) for `matchesNumberUnit` in
  `internal/launcher/router_test.go` (or wherever its existing tests live) covering the same
  fraction/mixed-number/malformed cases, asserting it agrees with `ParseInput`'s notion of what
  parses.
- `internal/launcher/unitsprovider_test.go` already tests `UnitsProvider.Query` conversion output —
  extend it with:
  - a case converting to inches landing within 1/64" of a 1/16 mark, asserting the `~<fraction>
    inch`-style suffix
  - a case landing on an exact 1/16 multiple, asserting no `~`
  - a case landing nearer the midpoint between two marks, asserting the `between X and Y inch` form
  - a case producing a negative inch result, asserting the sign appears once before the fraction/range
  - a case converting to `in²` or `in³`, asserting no fractional suffix is added

## Out of Scope

- Fractional notation is not added to `calc.Evaluate`/the general math-expression grammar — only to
  `UnitsProvider`'s quantity+unit parsing (and the `matchesNumberUnit` shape-check that supports it).
- No fractional formatting for units other than linear inches (feet-and-inches display, fractional
  centimeters, `in²`/`in³`, etc. are not part of this change).
- No configurable denominator precision — 1/16, and the 1/64 closeness threshold, are fixed, not
  settings.
- No change to how currency amounts are parsed or formatted.

## Further Notes

Feature originated from wanting to type real-world lumber/hardware measurements (`3/8 in`, `1 1/4
in`) directly into the launcher's unit converter. Scope was settled by grilling before writing this
spec — no map/tickets were needed since the destination had no open decisions left, only
implementation detail. A design-review pass surfaced a real router-side bug (`matchesNumberUnit`
needing the same grammar) and several open formatting questions (whitespace tolerance, zero
denominator, rounding ties, negative sign placement, near-midpoint precision), all resolved via a
follow-up grilling round and folded into this document.
