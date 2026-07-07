# Plan: `blf beads` — Beads issue browser TUI

An interactive TUI over the `bd` (Beads) CLI: fuzzy-browse issues, preview detail
(with subtasks + blocked-by trees), create/triage, and hand a chosen issue id to
an AI agent. See CONTEXT.md for the canonical terms (**blf beads**, **readiness**,
**issue preview**, **create mode**).

## Design decisions (from /grill)

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
  - `ctrl+r` → refresh · `ctrl+f` → cycle scope filter (actionable→ready→blocked→all)
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
- [ ] Mutations: `Create(title, opts)`, `UpdateStatus(id, s)`, `Close/Reopen(id)`
- [x] `bd` presence + db-discovery checks with clear errors; `-C` dir passthrough
- [x] Unit tests with a fake `bd` (exec indirection) covering JSON decode + arg building

### 2. Readiness + sort
- [x] Derive unblocked/blocked per issue from the ready set
- [x] Readiness-bucketed comparator (bucket → priority → updated-at)
- [x] Tests for bucket ordering and readiness derivation

### 3. Preview rendering (`internal/beads` view helpers)
- [ ] Subtasks tree (hierarchy, nested, completion count)
- [ ] Blocked-by tree (transitive, rooted, diamond/cycle collapse marker)
- [ ] Header (status/priority/type) + description + metadata; parent breadcrumb for subtasks
- [ ] Tests for tree building incl. diamond + cycle collapsing

### 4. TUI model (`internal/beads/model.go`)
- [x] Embed `fuzzyfinder`; flat list, row rendering (icons, tags, `↓/↑` badge, readiness color)
- [ ] Side-by-side layout + narrow fallback + `tab` toggle (`SetSize` with reduced width)
- [ ] Lazy debounced cached preview fetch (async `tea.Cmd`), loading placeholders
- [x] Scope filter cycle (`ctrl+f`) re-fetches set
- [ ] Manual refresh (`ctrl+r`)
- [x] Client-side fuzzy via `fuzzyfinder.Find`
- [x] Empty-state line; `bd`/db error surfacing before entering the TUI

### 5. Modes + actions
- [ ] Create mode (input repurpose, epic `--parent` + `ctrl+t` toggle) → `bd create`, re-fetch, select new
- [ ] Status-pick mode → `bd update --status`
- [ ] Close/reopen (`ctrl+x`), edit handoff (`ctrl+e`, suspend/exec/return like claude history)
- [ ] Graph shell-out (`ctrl+g`)
- [x] Enter: copy id (lipgloss? no — clipboard via platform) + print to stdout + quit

### 6. Command wiring + docs
- [x] `cmd/beads.go` cobra command `blf beads` with `-C/--dir`
- [ ] Help/footer key hints (mirror launcher's `?`/footer approach)
- [ ] README section + CHANGELOG entry (`/changelog`)
- [x] Manual verification against this repo's `.beads` db (35 issues)

## Open / deferred
- "Roots only" list mode (hide subtasks) — deferred unless the flat density bothers us.
- "New blocked-by dependency" creation — out of scope for v1.
- Live polling / auto-refresh — intentionally omitted.

## Possible ADR
- **Shell out to `bd` CLI (JSON) as source of truth vs. reading the dolt db directly.**
  Hard-ish to reverse, surprising to a future reader, real trade-off (CLI coupling &
  process spawns vs. direct db access / a Go beads lib). Offer once implementation confirms
  the boundary holds.
