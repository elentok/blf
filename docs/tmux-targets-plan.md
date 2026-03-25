# blf tmux-targets Implementation Plan

## Goal

Implement `blf tmux-targets` for tmux key bindings (e.g. `bind-key t run 'blf tmux-targets'`) that:

1. Captures the visible viewport of the current pane
2. Opens a tmux popup at the same size
3. Renders the same content with one active highlighted target at a time
4. Lets users navigate targets and trigger actions (`y`, `enter`/`o`, `q`)

This plan follows decisions in `docs/prompts/tmux-targets.md`.

## UX and Behavior (Locked)

- Command: `blf tmux-targets`
- Scope: viewport only (not scrollback)
- Navigation:
  - next/prev target: `j`/`k` and arrows
  - same-line horizontal movement (if implemented): `h`/`l`
  - first/last target: `gg` / `G`
- Actions:
  - `y`: copy selected target and exit
  - `enter` / `o`: open selected target if openable, then exit
  - non-openable open action: no-op + `tmux display-message`
  - `q`/`esc`: exit
- Highlight model: only one selected target at a time

## Target Types (v1)

- URLs:
  - `https://...`, `http://...`
  - bare domains and domain+path forms (`hello.com`, `hello.com/world`, `hello.org`)
  - PR/issue URLs
- File references:
  - absolute and relative paths
  - `path:line` and `path:line:col`
- Git hashes:
  - short and long commit hashes
- Additional text patterns:
  - email addresses
  - IP addresses and `host:port`
  - `#123` style issue refs
  - UUIDs
  - branch/tag-like tokens

## Proposed Architecture

- `cmd/cmd.go`
  - add `tmux-targets` command routing
- `internal/tmuxtargets/`
  - `run.go`: command orchestration + tmux integration
  - `capture.go`: viewport capture + geometry
  - `patterns.go`: compiled regex patterns + precedence
  - `scan.go`: extract target spans with type metadata
  - `render.go`: render dimmed text + highlighted selected span
  - `model.go` / `update.go` / `view.go`: interactive state machine
  - `actions.go`: open/copy + openability checks
- Reuse existing helpers where possible:
  - open/copy from `internal/platform`
  - tmux error display strategy from `internal/tmuxlinks`

## tmux Integration Plan

1. Verify running inside tmux (`TMUX` env, `tmux` binary available)
2. Capture viewport content only (not history)
3. Read pane dimensions (width/height) from tmux format values
4. Launch popup at matching size
5. Run interactive `blf tmux-targets --popup` mode inside popup (internal flag)
6. If top-level command fails, emit `tmux display-message -d 5000 ...`

Notes:

- Keep popup lifecycle robust: clean exit statuses and no raw terminal artifacts
- Preserve line order/content exactly; only color treatment changes

## Matching and Precedence Strategy

Because patterns overlap, apply deterministic precedence to avoid noisy selections.

Proposed precedence (highest first):

1. Full URLs (http/https, PR/issue URLs)
2. File refs with line/col (`path:line:col`, `path:line`)
3. Plain file paths
4. Commit hashes
5. Email
6. IP / host:port
7. UUID
8. Issue refs (`#123`)
9. Branch/tag-like tokens
10. Bare domains/domain+path

Conflict rule:

- Prefer earliest start position; if same start, prefer longer match; if still tied, prefer higher precedence.

## Rendering Strategy

- Preserve original text layout and wrapping exactly as captured
- Render all text in base color
- Render selected target in accent color/style (e.g. inverse or bold)
- Optional: render non-selected targets in a subtle secondary tone (still distinguishable)

Implementation detail:

- Track spans as line+column ranges based on captured text
- Render line by line, slicing by spans for stable highlighting

## Interaction Model

- Build ordered target list in reading order (top-left to bottom-right)
- Maintain selected index in model
- Key handling:
  - `j`/down: next
  - `k`/up: previous
  - `h`/left, `l`/right: optional same-line nav fallback to prev/next
  - `g`: first
  - `G`: last
  - `y`: copy selected target, exit 0
  - `enter`/`o`: open if openable, else message + stay
  - `q`/`esc`/`ctrl+c`: exit 0

## Openability Rules (v1)

Openable:

- URLs (explicit and normalized bare domain forms)

Not openable (for now):

- file paths, hashes, UUIDs, issue refs, etc.

Behavior:

- On non-openable `enter`/`o`, show `tmux display-message -d 5000 "target is not openable"` and continue.

## Milestones

### Milestone 1: Command and tmux plumbing

- Add `tmux-targets` command entry
- Add viewport capture and popup launch scaffolding
- Add failure reporting via `tmux display-message`
- Smoke test popup launch from tmux binding

### Milestone 2: Pattern engine and span extraction

- Implement pattern set and precedence
- Implement overlap resolution and ordered target list
- Add normalization metadata (e.g. bare domain -> openable URL)
- Unit tests for extraction quality and precedence conflicts

### Milestone 3: Interactive popup UI

- Implement model/update/view loop
- Render full content with selected target highlight
- Implement navigation keys (`hjkl`, arrows, `gg`, `G`)
- Unit tests for navigation and selection boundaries

### Milestone 4: Actions and integration polish

- Implement `y` copy action and exit
- Implement `enter`/`o` open action and exit/no-op-message behavior
- Add user feedback messaging and edge-case handling
- Integration-style tests with mocked tmux/open/copy runners

### Milestone 5: Docs and validation

- Update README command docs and usage notes
- Add changelog entry (patch release)
- Run `go test ./...` and manual tmux verification checklist

## Testing Plan

Unit tests:

- regex coverage per target type
- overlap/precedence resolution correctness
- stable span positions across mixed lines
- key handling (`g`, `G`, arrows, `hjkl`, action keys)
- openability classification and normalization

Integration-style tests (mocked command runners):

- viewport capture + popup invocation args
- no-target behavior
- non-openable open action triggers tmux message
- copy/open action dispatch and exit behavior

Manual verification:

- run inside tmux on real noisy logs/code output
- verify wrapped content fidelity in popup
- verify action behavior and failure messages via tmux status line

## Risks and Mitigations

- Pattern false positives: mitigate with precedence + strict regex boundaries
- Pattern false negatives: add focused fixtures and iterative regex tuning
- Rendering drift: preserve original lines and avoid reflow
- Key-sequence ambiguity (`gg`): track short-lived pending `g` state
- Performance on large viewports: precompile regex, single-pass conflict resolution

## Out of Scope (for v1)

- Custom user-defined patterns (e.g. Jira config)
- Opening non-URL types (files, commits, tickets) with external resolvers
- Multi-select/multi-action workflows
