# Plan: `blf beads` — Beads issue browser TUI

An interactive TUI over the `bd` (Beads) CLI: fuzzy-browse issues, preview detail
(with subtasks + blocked-by trees), create/triage, and hand a chosen issue id to
an AI agent. See CONTEXT.md for the canonical terms (**blf beads**, **issue tree**,
**issue preview**, **create mode**).

> **Re-scoped** after initial shipping: the readiness/scope-cycle design below (task 2,
> parts of task 4/5) was reversed in favor of a plain interactive view of `bd list`. See
> the "Re-scope: plain bd-list view" section at the end and ADR 0009.

## Design decisions (from /grill, original design — see re-scope section below for what changed)

- **Source of truth**: shell out to `bd` (JSON), never read the beads db directly.
- **Widget**: embed `internal/fuzzyfinder`; own item type, rendering, preview, actions.
- **Layout**: side-by-side — list ~40% left, **issue preview** ~60% right; narrow
  terminals (< ~100 cols) hide the preview, `tab` toggles it.
- **List**: flat (epics + subtasks + standalone as rows). Epic rows tagged `epic`;
  subtask rows carry a dim `↳ <parent>` tag. "Roots only" mode deferred.
- **Readiness**: from `bd ready` set membership (authoritative), not `dependency_count`.
  Row shows a readiness glyph/color + a dim `↓N ↑M` badge (blockers / dependents),
  zeros hidden. Sort = readiness bucket (unblocked → blocked → closed/deferred),
  priority then updated-at within each bucket.
- **Data flow**: fetch working set once per scope (`bd list --json` + `bd ready --json`),
  fuzzy client-side. Re-fetch only on scope-cycle, mutation, or manual refresh. No polling.
- **Preview detail**: lazy, debounced (~100ms), cached per id; header renders instantly,
  trees fill in async. Invalidate an id's cache on mutation.
- **Preview content**: two separate sections — **subtasks tree** (`bd children`, epic
  completion count) and **blocked-by tree** (transitive dependencies, rooted at issue,
  diamonds/cycles collapsed to `↑ <id> (see above)`), plus description + metadata.
  Subtask rows show a one-line parent breadcrumb.
- **Actions** (input stays focused; all ctrl-chords; `esc` backs out of a mode):
  - `enter` → copy id to clipboard + print id to stdout + **quit** (primary picker verb)
  - `ctrl+a` → **create mode** (repurpose input as title → `bd create`); on an epic row
    defaults `--parent <epic>`, `ctrl+t` toggles standalone
  - `ctrl+s` → **status-pick mode** (same mode-flip) → `bd update --status <s>`
  - `ctrl+x` → close / reopen toggle (`bd close` / `bd reopen`)
  - `ctrl+e` → edit in `$EDITOR` (`bd edit <id>`), suspend/return
  - `ctrl+g` → dependency graph shell-out (`bd graph`, compact→pager or `--html`→browser)
  - `ctrl+r` → refresh (no scope cycle — see re-scope section: dropped `ctrl+f`)
  - `tab` → toggle preview · `esc`/`ctrl+c` → back out / quit
  - nav: up/down, ctrl+k/j, ctrl+p/n
- **Command**: `blf beads`, single interactive command, `-C/--dir` passthrough.
  Fail fast if `bd` missing or no `.beads` db discoverable; in-TUI empty state otherwise.

## Tasks

### 1. Beads CLI adapter (`internal/beads`)
- [x] `Issue` struct matching `bd ... --json` (id, title, description, status,
      priority, issue_type, labels, dependency_count, dependent_count, parent, timestamps)
- [x] `List(scope)` → `bd list --json` with scope flags; `Ready()` → `bd ready --json` (id set)
- [x] `Show(id)`, `Children(id)`, `DepList(id)` for preview detail
- [x] Mutations: `Create(title, opts)`, `UpdateStatus(id, s)`, `Close/Reopen(id)`
- [x] `bd` presence + db-discovery checks with clear errors; `-C` dir passthrough
- [x] Unit tests with a fake `bd` (exec indirection) covering JSON decode + arg building

### 2. Readiness + sort
- [x] Derive unblocked/blocked per issue from the ready set
- [x] Readiness-bucketed comparator (bucket → priority → updated-at)
- [x] Tests for bucket ordering and readiness derivation

### 3. Preview rendering (`internal/beads` view helpers)
- [x] Subtasks tree (hierarchy, nested, completion count)
- [x] Blocked-by tree (transitive, rooted, diamond/cycle collapse marker)
- [x] Header (status/priority/type) + description + metadata; parent breadcrumb for subtasks
- [x] Tests for tree building incl. diamond + cycle collapsing

### 4. TUI model (`internal/beads/model.go`)
- [x] Embed `fuzzyfinder`; flat list, row rendering (icons, tags, `↓/↑` badge, readiness color)
- [x] Side-by-side layout + narrow fallback + `tab` toggle (`SetSize` with reduced width)
- [x] Lazy debounced cached preview fetch (async `tea.Cmd`), loading placeholders
- [x] Scope filter cycle (`ctrl+f`) re-fetches set
- [x] Manual refresh (`ctrl+r`)
- [x] Client-side fuzzy via `fuzzyfinder.Find`
- [x] Empty-state line; `bd`/db error surfacing before entering the TUI

### 5. Modes + actions
- [x] Create mode (input repurpose, epic `--parent` + `ctrl+t` toggle) → `bd create`, re-fetch, select new
- [x] Status-pick mode → `bd update --status`
- [x] Close/reopen (`ctrl+x`), edit handoff (`ctrl+e`, suspend/exec/return like claude history)
- [x] Graph shell-out (`ctrl+g`)
- [x] Enter: copy id (lipgloss? no — clipboard via platform) + print to stdout + quit

### 6. Command wiring + docs
- [x] `cmd/beads.go` cobra command `blf beads` with `-C/--dir`
- [x] Help/footer key hints (mirror launcher's `?`/footer approach)
- [x] README section + CHANGELOG entry (`/changelog`)
- [x] Manual verification against this repo's `.beads` db (35 issues)

## Review

- `blf beads` now uses the same compact-footer-plus-`?` help pattern as the launcher, keeping browse-mode chrome quiet while leaving the full keymap discoverable.
- The CLI boundary held cleanly: all Beads reads and writes still route through `bd`, which was stable enough to document as an ADR.

## Open / deferred
- "New blocked-by dependency" creation — out of scope for v1.
- Live polling / auto-refresh — intentionally omitted.
- Nav that skips dimmed ancestor rows — not implemented; dimming is visual only, rows stay
  normally selectable (would require extending the shared `internal/fuzzyfinder` widget).

## ADRs
- **0008** — shell out to `bd` CLI (JSON) as source of truth vs. reading the dolt db directly.
- **0009** — plain `bd list` view over readiness/scope-cycle triage queue (see below).

## Re-scope: plain bd-list view (supersedes readiness/scope-cycle design)

Reversed the original readiness-bucketed sort + `ctrl+f` scope cycle (actionable → ready →
blocked → all) in favor of a direct interactive view of `bd list` / `bd list --all`. See
ADR 0009 for the rationale and CONTEXT.md's **blf beads**/**issue tree** entries for the
current terms.

- [x] `internal/beads`: `Adapter.List(scope Scope)` → `Adapter.List(all bool)`; dropped
      `Adapter.Ready()`, `Readiness`/`ClassifyReadiness`/`SortIssues` (readiness.go deleted)
- [x] New `internal/beads/listtree.go`: `TreeRow`, `BuildIssueTree(issues, matchIDs)` —
      groups by `parent`, sorts top-level/siblings by id (matching `bd list`'s own terminal
      order, since `bd list --json` isn't tree-ordered), prunes non-matching subtrees while
      dimming a non-matching ancestor of a match
- [x] `internal/beads/model.go`: `ModelConfig.Scope` → `ModelConfig.All bool`; `displayRef`
      is now `*[]TreeRow`; dropped `readyLoadedMsg`/`loadReadyCmd`/`loadSnapshotCmd`/
      `nextScope`/the `ctrl+f` handler; row rendering drops the readiness glyph, `↓N ↑M`
      badge, and epic/parent text tag in favor of tree indentation + bold/colored epic rows
- [x] `internal/beads/styles.go`: readiness colors → `epicRowStyle` (bold + color),
      `dimRowStyle` (dimmed ancestor context)
- [x] `cmd/beads.go`: added `-a/--all` flag, wired to `ModelConfig.All`
- [x] Updated `internal/beads/adapter_test.go`, `model_test.go`; added `listtree_test.go`
- [x] Updated PRD (`docs/prds/blf-beads-tui.md`) and this plan for the new design
- [x] Manual verification: `blf beads` / `blf beads -a` against this repo's `.beads` db —
      confirmed nested epic/child rendering matches `bd list`'s terminal tree, epic rows
      render bold+colored, and fuzzy-filtering dims a non-matching parent epic while keeping
      a matching child visible
