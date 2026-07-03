## Problem Statement

**launcher history** currently records the raw query text you typed (e.g. `"kit"`), not what it
resolved to. This makes the history list low-value: seeing `"kit"` doesn't tell you it launched
Kitty, and two different abbreviations for the same app (`"kit"`, `"kitty"`) produce two separate,
redundant entries instead of coalescing into one. Recalling a history row also requires an extra
step — it only repopulates the input and recomputes, never re-launches directly, even though the
whole point of history is muscle-memory speed.

## Solution

For launch/run/open actions, history now records the picked result's label and its action (e.g.
"Kitty" → launch `/Applications/kitty.app`) instead of the query text — so the history list reads
like "what did I do" rather than "what did I type", and different queries for the same item
collapse into a single entry. Selecting one of these entries from the empty-input history list
direct-fires its action immediately on Enter, rather than repopulating the input for a second
confirmation. Copy actions (calc/unit/currency) are unchanged — those results have no identity
independent of their query, so they keep recording query text and keep the populate-and-recompute
behavior on recall.

This was scoped via a `/grill` session and recorded as ADR 0006
(`docs/adr/0006-launcher-history-item-based-direct-refire.md`); `CONTEXT.md`'s **launcher history**,
**history direct-fire**, and **history hint** entries were updated as part of that session and are
the source of truth for vocabulary here.

## User Stories

1. As a launcher user, I want history to show the app/script/directory I actually opened (e.g. "Kitty"), so that the list is meaningful at a glance instead of showing abbreviations I typed.
2. As a launcher user, I want different queries that resolve to the same item (e.g. "kit" and "kitty" both launching Kitty) to collapse into one history entry, so that my recent-items list isn't cluttered with duplicates.
3. As a launcher user, I want selecting a launch/run/open history entry to immediately perform that action, so that recalling something I do often is a single keystroke instead of search-then-confirm.
4. As a launcher user, I want Ctrl+R/Ctrl+F to still just fill the input with the entry's label so I can keep editing/searching, so that I retain a shell-history-style "recall for editing" path distinct from direct re-fire.
5. As a launcher user, I want calc/unit/currency history to keep working exactly as before (query text, populate + recompute, with its computed hint), so that this change doesn't regress the one action type where the query itself is the meaningful thing to recall.
6. As a launcher user, I want Ctrl+X to reliably delete the exact entry I've selected, so that deleting one launch entry never accidentally removes a different entry that happens to share a similar label.
7. As a launcher user, I want Ctrl+S (explicit save of my current input) to keep working for ad hoc queries, so that I don't lose that escape hatch when I haven't picked a specific result yet.
8. As a launcher user upgrading from an older version, I want my old history file to just start empty rather than error or corrupt the launcher, so that the upgrade is safe even though I lose the old (low-value) query list.
9. As a developer maintaining the launcher, I want the history package's on-disk format and dedup/delete-key logic fully encapsulated behind its existing API shape (`Append`/`Entries`/`Remove`/`Load`/`Save`), so that `model.go` doesn't need to know how entries are serialized or deduplicated.
10. As a developer maintaining the launcher, I want the new `Entry` shape to reuse the existing `ActionType` space (`ActionCopy`/`ActionLaunch`/`ActionRun`/`ActionOpen`) rather than inventing a parallel enum, so that the history package's notion of "action" stays consistent with the rest of the launcher.
11. As a developer maintaining the launcher, I want `model.go`'s existing `recordHistory` call sites updated in place rather than restructured, so that the change stays a targeted diff against a well-tested file.

## Implementation Decisions

- **`internal/launcher/history` package (deep module, redesigned)**:
  - `Entry` replaces the current bare string: `{Label string, ActionType int, Target string}`. `ActionType` mirrors `launcher.ActionType`'s values (`ActionCopy`/`ActionLaunch`/`ActionRun`/`ActionOpen`) by numeric convention, converted at the `model.go` boundary — the `history` package itself does not import `launcher` (avoids an import cycle; `launcher` already imports `history`).
  - For copy entries: `Label` is the query text, `ActionType` is `ActionCopy`, `Target` is unused/empty (no direct-fire target — recall means populate-and-recompute from `Label`).
  - For launch/run/open entries: `Label` is the result's `Title`, `ActionType`/`Target` are the result's `Action.Type`/`Action.Target` — recall means direct-fire that action.
  - **Identity key** (used for dedup-on-append and for `Remove`): `(ActionType, Target)` for launch/run/open entries; falls back to `Label` alone for copy entries (since `Target` is unused there and the prior behavior deduped by exact query string). This one key function is the crux of the module — it must be exercised thoroughly in tests.
  - `Append(entry Entry)` replaces `Append(query string)`: moves an existing entry with the same identity key to the front (same move-to-front-on-repeat semantics as today), then caps at `MaxEntries` (unchanged, 30).
  - `Remove(actionType int, target string, label string)` (or an equivalent signature taking whatever the identity key needs) replaces `Remove(query string)`, matching by the same identity key used in `Append`.
  - `Entries() []Entry` replaces `Entries() []string`.
  - **Storage format**: JSON-lines (one JSON object per entry) replaces the current plain-text-one-query-per-line format. `Load`/`Save` keep their existing failure-tolerant contract (missing/unreadable file → empty history, no error) — a file that fails to parse as JSON-lines (i.e. any pre-upgrade plain-text history file) is treated the same as missing, per ADR 0006. No migration code.
  - `MaxEntries` (30) and the overall package shape (`New`/`Load`/`Save`/`Append`/`Remove`/`Entries`/`Len`) are otherwise unchanged.
- **`model.go` wiring**:
  - `recordHistory` (currently `recordHistory(query string)`, called from the `ActionCopy`/`ActionLaunch`/`ActionRun`/`ActionOpen` branches of `act()`) is updated to build a `history.Entry` from the picked `Result`: for `ActionCopy` it's `{Label: query, ActionType: ActionCopy, Target: ""}` (unchanged behavior); for the other three it's `{Label: r.Title, ActionType: r.Action.Type, Target: r.Action.Target}`.
  - The `ActionRecall` branch in `act()` (currently: `setQuery(r.Action.Target)` + `recomputeResults()`) splits on the recalled entry's action type: copy-type entries keep the existing populate-and-recompute behavior; launch/run/open-type entries instead build and immediately execute the corresponding `Action` (reusing the same launch/run/open execution paths already in `act()`) — this is the **history direct-fire** behavior from `CONTEXT.md`/ADR 0006.
  - The empty-input history list (in `recomputeResults()`, where `ActionRecall` results are synthesized from `m.cfg.History.Entries()`) uses each entry's `Label` as `Title` and only computes a `historyHint` subtitle for copy-type entries (launch/run/open entries never show a hint, since there's no query to compute one from).
  - Ctrl+R/Ctrl+F (`m.setQuery(entries[next])`) keep populating the input with the entry's `Label` — unchanged behavior, no direct-fire, regardless of entry type.
  - Ctrl+S (explicit save of current raw input) keeps saving a copy-type entry (`Label` = current input text) — unaffected by this change, since there is no "picked result" in that flow.
  - Ctrl+X (delete) is updated to match on the selected history row's `(ActionType, Target)` (falling back to `Label` for copy-type rows), instead of the current exact-string match on `Action.Target`.
- **`cmd/launcher.go`**: no interface change expected (`history.Load(historyPath)` keeps the same signature); only the package's internal behavior changes.

## Testing Decisions

- Tests should assert observable behavior (what `Entries()` returns, what gets persisted/loaded, resulting dedup/delete outcomes) — not the exact JSON-lines byte layout.
- **`internal/launcher/history`** (primary test target, mirroring the existing `history_test.go` structure and coverage):
  - `Append` of a launch/run/open-shaped entry, then a second `Append` with the same `(ActionType, Target)` but a different `Label`/query origin moves the existing entry to the front and does not create a duplicate.
  - `Append` of two copy-shaped entries with different `Label`s (query text) are kept distinct; appending the same `Label` again moves it to front (regression coverage for existing copy-entry behavior).
  - `MaxEntries` cap still enforced after the `Entry` shape change (adapt existing `TestAppend_cap`).
  - `Remove` matching by `(ActionType, Target)` removes the correct launch/run/open entry even if two entries happen to share a `Label`.
  - `Remove` matching by `Label` still works for copy-shaped entries.
  - `Load`/`Save` round-trip preserves all `Entry` fields exactly (adapt existing `TestLoadSave_roundtrip`).
  - `Load` of a missing file returns empty history (existing `TestLoad_missingFile`, unchanged expectation).
  - `Load` of a pre-upgrade plain-text history file (one bare string per line, not JSON) returns empty history rather than erroring or partially parsing — this is the explicit migration-safety case from ADR 0006.
  - `Load` still enforces the cap when reading a file with more than `MaxEntries` JSON-lines entries (adapt existing `TestLoad_capEnforced`).
- **`internal/launcher` `model_test.go`** (extend existing suite with new cases, called out explicitly per the scoping discussion):
  - Pressing Enter on a launch/run/open-type history row (empty-input list) triggers the same launch/run/open execution path as picking that result fresh — i.e. direct-fire — without an intermediate populate-and-recompute step.
  - Pressing Enter on a copy-type history row still populates the input and recomputes (regression coverage — this path must not change).
  - A launch/run/open-type history row never carries a `historyHint` subtitle; a copy-type row's hint behavior is unchanged.
  - Ctrl+X on a launch/run/open-type row removes the correct entry when another entry shares a `Label` but not the `(ActionType, Target)`.
  - Ctrl+R/Ctrl+F on a launch/run/open-type entry populates the input with its `Label` and does not direct-fire.

## Out of Scope

- Applying item-based history or direct-fire to `ActionCopy` (calc/unit/currency) results — explicitly rejected in ADR 0006, since those results have no identity independent of their query.
- Direct-fire on Ctrl+R/Ctrl+F — they remain a text-recall-for-editing shortcut only.
- Migrating/preserving pre-upgrade plain-text history entries — old files are treated as empty on load, per ADR 0006.
- Any change to `learnedrank` (a separate, already-shipped feature) — it continues to key on `(query, Action.Type+Action.Target)` independently of this change.
- Any change to the `ActionRecall` action type's definition/name in `provider.go` beyond how `model.go` interprets a recalled entry's stored action type.

## Further Notes

- This feature was scoped via a `/grill` session; ADR 0006 and the updated `CONTEXT.md` entries
  (**launcher history**, **history direct-fire**, **history hint**) were written before this PRD and
  are the source of truth for vocabulary and rationale.
- The exact JSON-lines schema (field names, whether `ActionType` is serialized as an int or a
  string) is left to the implementing task as long as it round-trips losslessly; no schema is
  mandated by this PRD.
