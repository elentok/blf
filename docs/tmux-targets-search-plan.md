# blf tmux-targets Search Plan

## Goal

Add search to `blf tmux-targets` where `/` enters search mode with fuzzy filtering on target text.

Expected behavior:

- Typing updates query and filters targets in real time
- Selection jumps to first match
- `esc` clears search and restores unfiltered target set
- `enter` locks search (filtered navigation scope)
- In locked mode, `j/k` navigate only filtered matches
- If there are no matches, selection is cleared and status shows `0/N`

## Decisions (Confirmed)

- Fuzzy library: `github.com/sahilm/fuzzy`
- Match input: target text only (not target kind)
- Locked mode navigation: only filtered matches
- Zero matches: clear selection and show `0/N`
- No overlay/footer that could hide content; use `tmux display-message` for mode/status feedback
- Extract `tmux display-message` usage into a shared helper reused by `tmux-targets` and `tmux-links`

## UX States

### Normal

- Existing behavior
- `j/k`, arrows, `h/l`, `gg`, `G`, `y/c`, `enter/o`, `q`
- `/` enters search typing mode

### Search Typing (Transient)

- Keypresses edit query
- Filter recomputes after each keypress
- Selection follows first match when available
- `enter` transitions to Locked Filtered mode
- `esc` clears query + exits search mode back to Normal
- Emit status with `tmux display-message -d 5000`, e.g. `SEARCH /abc (4/27)`

### Locked Filtered

- Query stays active
- Navigation and actions are constrained to filtered matches
- `/` re-enters typing mode to edit query
- `esc` clears filter and returns to Normal
- Emit status: `FILTERED /abc (4/27)`

## Data Model Changes

Add fields to `internal/tmuxtargets/model.go`:

- `searchMode bool` (currently typing)
- `filterLocked bool`
- `query string`
- `filteredIdx []int` (indexes into `targets`)
- `selected int` semantics updated to index in current active list (or `-1` for none)

Derived helpers:

- `activeIndexes() []int`:
  - if `filterLocked || searchMode` and query non-empty -> `filteredIdx`
  - else -> all target indexes
- `selectedTarget() (target, bool)`
- `setSelectionToFirstMatchOrNone()`

## Fuzzy Filtering Strategy

- Use `fuzzy.Find(query, candidates)` with candidates = target texts
- Preserve ranked order from fuzzy results for filtered navigation
- Keep filtering case-insensitive (library default behavior)
- For empty query:
  - in search mode: treat as no filter (all targets)
  - on `esc`: clear and leave search/locked modes

## Key Handling Changes

### In Search Typing mode

- text input chars append to query
- `backspace` removes last rune
- `enter`: lock filter (`filterLocked=true`, `searchMode=false`)
- `esc`: clear query and leave both search/lock modes
- `q` still exits

### In Normal / Locked

- `/`: enter typing mode (`searchMode=true`)
- `j/k` etc:
  - if locked or typing with query: move within `filteredIdx`
  - else move within all targets
- `y/c` and `enter/o` use selected target if any
- when no selected target (e.g. `0/N`): action keys show message and no-op

## Rendering Changes

- Keep current colors:
  - selected target style
  - non-selected highlightable targets in blue
- In filtered contexts:
  - matched targets keep blue/selected styles
  - non-matching targets render in base dim style
- If zero matches:
  - render all targets as non-selected/non-matching
  - no selected highlight

## tmux Status Messaging

Create shared tmux notify helper (for both features), for example in `internal/tmuxutil`:

- `DisplayMessage(msg string)` (default delay)
- `DisplayError(tool string, err error)` (prefix handling, no duplication)

Then use it in `tmux-targets` search/status flows:

- Emit on mode/query transitions:
  - Enter search typing
  - Query update (throttled by key events; acceptable for viewport scope)
  - Lock filter
  - Clear filter

Message formats:

- `SEARCH /<query> (<matches>/<total>)`
- `FILTERED /<query> (<matches>/<total>)`
- `SEARCH CLEARED`

## Milestones

### Milestone 1: Search state + fuzzy backend

- Add `sahilm/fuzzy` dependency
- Add model fields/helpers for filtered indexes and no-selection state
- Implement filter recomputation function + tests

### Milestone 2: Key handling + mode transitions

- Implement `/`, typing, backspace, `enter`, `esc`
- Implement locked navigation scope
- Ensure `0/N` clears selection and action no-ops message correctly

### Milestone 3: Rendering updates

- Distinguish matches vs non-matches in filtered states
- Keep single selected highlight rule
- Validate behavior with no matches

### Milestone 4: tmux status integration

- Extract shared `tmux display-message` helper and migrate `tmux-links` + `tmux-targets` to it
- Emit mode/query/filter status messages
- Ensure no duplicate noisy prefixes in status output

### Milestone 5: Verification + docs

- Update README `tmux-targets` section with search controls
- Add changelog note (patch)
- Run `go test ./...`
- Manual tmux check with real binding

## Test Plan

Unit tests (`internal/tmuxtargets/model_test.go` and helpers):

- Query to filtered index mapping
- Ranked match ordering from fuzzy results
- Mode transitions (`/`, `enter`, `esc`)
- Locked navigation constrained to filtered set
- `0/N` behavior: no selected target + action no-op

Integration-style tests (`internal/tmuxtargets/run_test.go`):

- status message emission on search mode transitions
- shared notifier behavior (delay, prefix normalization, tmux-env guard)
- no regression in popup orchestration

Manual verification checklist:

- `/` opens search, typing filters in place
- `enter` locks, `j/k` cycles filtered only
- `esc` clears and restores all targets
- last-line targets remain visible (no footer overlay)
- `0/N` displays status and prevents accidental actions
