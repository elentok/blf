## Problem Statement

In the **launcher**, a short query often surfaces several plausible results — e.g. typing "cal"
returns both "Calendar" and "Google Calendar". The current ranking (exact match > prefix match >
source weight > fuzzy score) is static: it never adapts to what the user actually picks. If the
user consistently reaches for "Google Calendar" on "cal", they must press `down` + `enter` every
single time, forever, because "Calendar" always wins the prefix-match tier.

## Solution

The launcher learns from repeated behavior. Whenever the user picks a result that wasn't first in
the list, the launcher remembers that (query, result) pair. The next time the exact same query is
typed, results with a remembered pick sort above every other tier — including exact and prefix
matches — highest pick-count first. No configuration or manual weighting is required; the ranking
adapts purely from usage.

## User Stories

1. As a launcher user, I want the launcher to remember which result I picked for a given query, so that I don't have to press `down` every time I search for the same thing.
2. As a launcher user, I want the remembered preference to apply even when my usual pick isn't an exact or prefix match, so that a "buried" result I actually want can still surface first.
3. As a launcher user, I want the learning to kick in immediately (not after some hidden threshold), so that behavior is predictable and I can feel the launcher adapting after just one correction.
4. As a launcher user, I want repeated corrections to reinforce the preference more strongly, so that if I sometimes pick the default and sometimes pick my preferred result, the one I pick more often wins.
5. As a launcher user, I want this learning to apply no matter what kind of result I'm picking (app, script, directory, computed value), so that the same convenience applies everywhere, not just app launches.
6. As a launcher user, I want the learned preference to be tied to the exact query text I typed, so that learning from "cal" doesn't unexpectedly change results for unrelated queries like "goo cal" or "c".
7. As a launcher user, I want my learned preferences to persist across launcher restarts, so that the adaptation isn't lost when the process restarts.
8. As a launcher user, I want learned preferences to never silently decay, so that a preference I set once keeps working reliably.
9. As a launcher user who wants to undo a learned preference, I want the data stored in a plain, human-editable file, so that I can fix it by hand even though there's no in-TUI clear action yet.
10. As a developer maintaining the launcher, I want the ranking logic (`Rank`) to remain a pure, dependency-free function, so that ranking behavior stays easy to unit test without file I/O or mocks.
11. As a developer maintaining the launcher, I want the new learned-rank storage to be a self-contained package (mirroring the existing `history` package), so that its persistence/increment logic can be tested in isolation from the TUI model.
12. As a developer maintaining the launcher, I want the existing static `Weight` field/concept left untouched, so that this feature doesn't force a rename across every provider.

## Implementation Decisions

- **New concept: "learned rank"** — distinct from the existing **source weight** (the static, per-provider `Weight` field on `Result`). Documented in `CONTEXT.md`. Ranking order becomes:
  `learned rank > exact match > prefix match > source weight > fuzzy score`.
- **New package `internal/launcher/learnedrank`** (deep module, mirrors `internal/launcher/history`'s shape and API style):
  - A `Store` holding counts keyed by `(query, resultKey)`, where `query` is the trimmed, exact query text and `resultKey` is a caller-supplied opaque string identifying the result (the launcher package will build this from `Action.Type` + `Action.Target`, i.e. the actual launch/copy/run/open target — not the display Title).
  - `New() *Store` — empty store.
  - `Load(path string) *Store` — reads from disk; returns an empty store if the file is missing or unreadable (same failure-tolerant contract as `history.Load`).
  - `(*Store) Save(path string) error` — persists to disk, creating parent dirs as needed.
  - `(*Store) Increment(query, resultKey string)` — increments the count for that pair. Trims/normalizes `query` the same way `history.Append` trims entries. No-op on empty query or resultKey.
  - `(*Store) Counts(query string) map[string]int` — returns a snapshot map of `resultKey -> count` for all result keys ever recorded against that exact query (empty map if none).
  - Storage format: a plain, human-editable text file (consistent with `history`'s plain-file approach, not JSON), stored at `launcher-learned-ranks` in the same XDG state dir as `launcher-history`. Exact line format is an implementation detail for the package (e.g. one `query\tresultKey\tcount` per line), but must round-trip losslessly and remain easy to hand-edit/delete a line.
  - No decay, no expiry, no cap on distinct (query, resultKey) pairs for v1.
- **`router.go` `Rank()` extended** to accept a second parameter: `learnedRanks map[string]int` (resultKey → count, pre-scoped to the current query by the caller). `Rank` stays pure — it does not know about `Store`, files, or the current query string, only the pre-computed counts map for whatever query produced the given `results`.
  - For each `Result`, its learned-rank count is looked up via the same resultKey scheme (`Action.Type` + `Action.Target`); results with count `0` (unset) always sort below any result with a nonzero count.
  - Sort key becomes, in order: `learnedRankCount (desc) > IsExactMatch > IsPrefixMatch > Weight (desc) > FuzzyScore (desc)`.
  - `Rank`'s existing call sites that don't care about learned rank (if any arise) can pass `nil`/empty map, which is equivalent to today's behavior.
- **`model.go` wiring**:
  - `ModelConfig` gains `LearnedRank *learnedrank.Store` (optional; nil disables the feature) and `LearnedRankPath string` (path to persist; empty skips persistence) — mirroring the existing `History`/`HistoryPath` fields exactly.
  - In `recomputeResults()`, before calling `Rank`, compute `m.cfg.LearnedRank.Counts(query)` (if non-nil) and pass it through.
  - In `act()`, when the picked result's index (`m.selected`) is not `0` at the time of the pick, call `m.cfg.LearnedRank.Increment(query, resultKey)` and persist via `Save`, mirroring the existing `recordHistory`/`saveHistory` pattern. This applies for every action type that already calls `recordHistory` today (copy, launch, run, open) — no special-casing by action type.
  - Result key construction (`Action.Type` + `Action.Target`) lives as a small helper, likely alongside `Result`/`Action` in `provider.go`, so both `model.go` and `router.go` build the same key consistently.
- **`cmd/launcher.go` wiring**: load the store at startup from `filepath.Join(stateDir, "launcher-learned-ranks")` (same `stateDir` already used for `launcher-history`), and pass both the store and its path into `ModelConfig`, mirroring the existing history load/wire block.
- **No in-TUI clear/reset action** for v1 — explicitly deferred. The state file is plain and human-editable as the escape hatch.
- **No rename of the existing `Weight` field/term** — it keeps meaning "source weight" as today; only the new concept gets a new name.

## Testing Decisions

- Tests should assert observable behavior (stored counts, resulting sort order, persisted file round-trips) — not internal representation details like the exact on-disk line format.
- **`internal/launcher/learnedrank`**: new `learnedrank_test.go` mirroring `history_test.go`'s structure and coverage —
  - `Increment` on a fresh store, then `Counts` reflects it.
  - Multiple increments on the same (query, resultKey) accumulate.
  - Different queries / different resultKeys are tracked independently (no cross-contamination).
  - Empty/whitespace query or resultKey is a no-op.
  - `Load`/`Save` round-trip preserves all counts exactly.
  - `Load` of a missing file returns an empty store (no error).
- **`router.go` `Rank()`**: extend `router_test.go` (or add a sibling test file) with cases covering the new top tier —
  - A result with a nonzero learned-rank count outranks an exact match with zero count.
  - Among two results with nonzero counts for the same query, the higher count wins.
  - When all results have zero count, behavior is unchanged from the pre-existing exact > prefix > weight > fuzzy ordering (regression coverage for the existing tests).
  - Ties within the same learned-rank count fall through to the existing tiers.
- **`model.go`**: extend `model_test.go` with integration-style cases (simulating key events, following existing patterns in that file) —
  - Selecting a non-first result via `down` + `enter` records an increment for that (query, resultKey).
  - Selecting the first result does not record an increment.
  - After a recorded increment, re-issuing the same query re-ranks the previously non-first result to the top.
  - A different query does not pick up the increment recorded under another query's text.

## Out of Scope

- Any in-TUI action to view, clear, or edit learned-rank entries (deferred; the plain state file is the only escape hatch for now).
- Decay, expiry, or time-based weakening of learned-rank counts.
- Fuzzy/prefix-based query matching for learned rank (e.g. matching "ca" against a stored "cal" entry) — only exact trimmed query text matches.
- Renaming the existing `Weight` field/concept.
- Any config-file-level (declarative) way to pre-seed or override learned ranks.
- Syncing or sharing learned-rank data across machines.

## Further Notes

- This feature was scoped via a `/grill` session; the resulting terminology ("learned rank" vs.
  "source weight") and relationship notes were already written into `CONTEXT.md` before this PRD was
  drafted — implementers should treat that as the source of truth for vocabulary.
- The `learnedrank` package's file-format choice (plain text vs. something more structured) is left
  to the implementing task as long as it stays human-editable and round-trips losslessly; no schema
  is mandated by this PRD.
