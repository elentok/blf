# PRD: goto-agent live-refresh TUI fuzzy finder

See `CONTEXT.md` (**fuzzy finder**, **goto-agent**, **agent window**, **agent
status**, **launcher**) and `docs/adr/0005-goto-agent-self-owned-tui.md`.

## Problem Statement

I run several AI coding agents at once and use `blf kitty goto-agent` to jump to
one. The picker is built on `fzf`, which is a static snapshot: once it's open the
list is frozen. While I'm staring at it, an agent finishes (working → waiting),
or I'd like to leave it open and watch for an agent to become free — but `fzf`
can't show a spinner on the working agents or refresh a status in place. There's
no clean way to drive live updates into an external `fzf` process, so the picker
can't reflect what's actually happening to my agents right now.

## Solution

Replace the `fzf`-based goto-agent picker with our own TUI **fuzzy finder** (the
same kind of UI as the launcher), so the picker is **live**: working agents show
an animated spinner, status transitions (working → waiting → idle) appear in
place, and newly-spawned agents show up without relaunching. Leaving the picker
open becomes a useful way to watch agents.

To avoid a one-off UI, the launcher's reusable picker core is extracted into a
shared embedded **fuzzy finder** widget and the launcher is migrated onto it, so
the new agent picker is its second consumer and the abstraction has two real
users.

## User Stories

1. As a developer with several agents, I want the goto-agent picker to animate a
   spinner on each working agent, so that I can see at a glance which ones are
   still busy.
2. As a developer, I want an agent's row to change from working to waiting in
   place while the picker is open, so that I notice the moment it needs my input.
3. As a developer, I want a waiting agent to jump to the top of the list the
   moment it transitions, so that the agent needing attention surfaces itself.
4. As a developer, I want my cursor to stay on the same agent even as rows
   reorder around it, so that I don't accidentally jump to the wrong agent.
5. As a developer, I want to leave the picker open and have a newly-spawned agent
   appear in it live, so that I can wait for an agent to come up without
   relaunching.
6. As a developer, I want an in-picker "No agent windows" state that keeps
   polling, so that the picker is useful even when nothing is running yet.
7. As a developer, I want to type to fuzzy-filter the agent list, so that I can
   narrow to the agent I want by directory or task title.
8. As a developer, I want fuzzy matches highlighted in each row, so that I can
   see why a row matched my query.
9. As a developer, I want `up`/`down` and `ctrl-k`/`ctrl-j` to move the
   selection, so that navigation matches the launcher and my muscle memory.
10. As a developer, I want a live preview of the highlighted agent's screen, so
    that I can confirm which agent it is before jumping.
11. As a developer, I want the preview to keep updating for the selected agent,
    so that I see the agent's screen change, not just a stale snapshot.
12. As a developer, I want preview fetches debounced while I scroll, so that
    holding a navigation key stays smooth and doesn't spawn a burst of
    subprocesses.
13. As a developer, I want `enter` to focus the selected agent's exact window
    (pulling its tab and OS window forward) and close the picker, so that I land
    where the agent is.
14. As a developer, I want `esc`/`ctrl-c` to dismiss the picker without focusing
    anything, so that I can back out cleanly.
15. As a developer, I want a `?` help view listing the keys, so that I can
    discover what the picker does.
16. As a developer, I want the picker to keep the launcher's look and feel
    (input box, border, footer), so that the two pickers feel like one tool.
17. As a launcher user, I want the launcher to behave exactly as it does today
    after it is rebuilt on the shared widget, so that nothing I rely on
    regresses.
18. As a developer extending the picker later, I want to add my own keybindings
    from outside the widget, so that consumer-specific actions (like the
    launcher's history binds) don't require changing shared code.
19. As a maintainer, I want the session and os-window pickers left on fzf, so
    that the change stays scoped to where live refresh is actually needed.

## Implementation Decisions

### Modules

- **`internal/fuzzyfinder` (new, deep module).** The shared embedded TUI widget
  built on bubbletea v2 + lipgloss. Owns: the query input (`textinput`), the
  selection index, the viewport scroll math, the border / input-prompt /
  separator / footer chrome, the selected/normal/highlight row styles, and
  laying out the visible rows. **Index-based and type-agnostic** — it does not
  hold the consumer's items.
  - Interface (shape, not signatures): construct from a config carrying a
    selection-aware `renderRow(i, selected) -> string` callback and footer text;
    `Update(msg)` / `View()`; `Query()`, `Selected()` (index), `SetSelected(idx)`,
    `SetItemCount(n)`. It consumes only input + nav keys
    (`up`/`down`/`ctrl-k`/`ctrl-j`) and leaves all other keys for the consumer.
  - Also exports a reusable fuzzy match+highlight helper wrapping
    `github.com/sahilm/fuzzy` (already used by `internal/targets`) that consumers
    may call from their own ranking; ranking itself stays in the consumer.

- **Agent picker model (new, in `internal/kitty`).** A bubbletea model that
  embeds the widget and owns everything agent-specific: the `[]Agent` slice,
  fuzzy filtering on the query, status-first-then-recency sorting, per-row
  rendering (status glyph / spinner, dir, title, dimmed agent name), the two
  refresh tickers, the preview pane, and ID-stable selection. `GotoAgent` runs
  the program and reads the selected agent id from the final model.

- **`internal/launcher` (modified).** Migrated to embed the `fuzzyfinder` widget,
  delegating input/nav/scroll/chrome to it while keeping its own ranking,
  history, providers (math/units/currency/apps/scripts/settings), icons, action
  dispatch, and tickers. External behavior unchanged.

- **`internal/kitty` cleanup (modified).** Remove the fzf-specific agent
  plumbing; keep the still-used pieces.

### Composition and boundary (per ADR 0005)

- **Composition, not configuration.** Each consumer embeds the widget and keeps
  its own top-level `Update`/`View`, composing its own chrome around it (agent
  preview pane; launcher history/actions). The widget knows nothing about
  previews or actions.
- **Custom keybindings from the consumer.** The consumer's `Update` sees every
  `KeyMsg` first, handles its own binds, then forwards input/nav keys to
  `widget.Update`. The widget must not assume it owns the whole keymap.

### Live refresh (agent picker)

- **Two cadences.** A ~1s data tick re-runs `ListAgents` off the UI goroutine and
  delivers a fresh `[]Agent` as a message; a ~100ms spinner tick advances a
  single shared frame counter.
- **Spinner.** A shared-counter braille spinner (U+2800 range, matching the
  convention agents already use in their titles) for `working` rows; `waiting`
  and `idle` keep their static glyphs.
- **Reorder + cursor.** Selection is preserved by agent **ID** across refreshes;
  the list re-sorts live (waiting → working → idle, then recency) so waiting
  agents surface to top with the cursor following. When a query is active,
  ordering is fuzzy-match order instead.

### Preview pane

- Kept. The consumer renders the widget on the left and the preview on the right
  (`lipgloss.JoinHorizontal`); the widget stays preview-agnostic.
- Screen text is fetched via `RenderAgentPreview` in a `Cmd` (off the UI
  goroutine) on a ~80ms-debounced selection-change, and the selected agent's
  preview is also refreshed on each ~1s data tick so it stays live.

### Keys and exit

- Type to filter; `up`/`down` + `ctrl-k`/`ctrl-j` to move; `enter` records the
  selected agent and quits; `esc`/`ctrl-c` quit without focusing; `?` toggles
  help.
- `GotoAgent` reads the selected id from the final model and runs
  `kitten @ focus-window --match id:N` after the program exits — keeping the
  focus side-effect out of the model.

### Empty state

- The pre-launch "No agent windows" short-circuit in `GotoAgent` is dropped; the
  empty state renders inside the live TUI and keeps polling, so a newly-spawned
  agent appears without relaunching.

### Cleanup / scope

- Remove the dead fzf agent plumbing: `pickAgent`, `formatAgentChoices`,
  `parseAgentSelection`, `agentPreviewCommand`, `agentPreviewWindow`, and the
  hidden `__preview-agent` subcommand plus its `PreviewAgent` wrapper. Keep
  `RenderAgentPreview` (now called in-process), `FormatAgents`, and `ListAgents`.
- `pickSession` and `pickOSWindow` stay on fzf — out of scope.

## Testing Decisions

Good tests here assert **external behavior** by constructing the model, sending
`tea.Msg`s to `Update`, and asserting the resulting state/view — no real TUI, no
spawned subprocesses, no assertions on private helpers. Prior art:
`internal/launcher/model_test.go` and `internal/targets/model_test.go`.

- **`internal/fuzzyfinder` widget** — navigation keys move the selection within
  bounds, viewport scroll-offset math keeps the selection visible, and the fuzzy
  match+highlight helper returns the expected match ranges.
- **Agent picker model** — selection stays on the same agent **ID** across a
  data-refresh message even when rows reorder; a `working → waiting` transition
  re-sorts the row to the top live; typing fuzzy-filters the list; the spinner
  frame advances on the spinner tick; the in-TUI empty state renders when there
  are no agents.
- **`internal/launcher` (regression)** — the existing `model_test.go` must pass
  **unchanged** after the migration onto the widget; that is the proof the
  extraction preserved behavior.
- The two refresh tickers' real timing and the `kitty @ get-text` /
  `kitten @ focus-window` subprocess calls are **not** unit-tested directly,
  consistent with `pickSession` being untested. The existing pure-function tests
  (detection / status / sort / format) and `ListAgents` tests stay as-is.

## Out of Scope

- Migrating `pickSession` / `pickOSWindow` off fzf.
- New row actions in the agent picker (kill / delete an agent) — navigate-and-
  focus only.
- Any change to agent detection or status derivation (`AGENT_STATE` user var,
  braille-title fallback, OpenCode-always-idle) — that logic is reused as-is.
- Migrating the nvim send-to-agent feature to `list-agents --json`.
- Extracting a fully generic picker beyond what the launcher and agent picker
  both actually need.

## Further Notes

- The launcher migration is the riskiest part (the launcher works today), so it
  should land as its own change gated on `launcher/model_test.go` passing
  unchanged. The fuzzyfinder widget and agent picker can land first to get the
  live-refresh win early, with the launcher migrated after.
- The braille spinner range and the existing static glyphs come from the current
  `internal/kitty/agents.go` rendering; reuse those conventions.
