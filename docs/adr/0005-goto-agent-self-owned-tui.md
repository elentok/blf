# ADR 0005: goto-agent uses a self-owned TUI fuzzy finder instead of fzf

**Status**: Accepted

## Context

`blf kitty goto-agent` opens a picker of all **agent windows** with their **agent
status** (working / waiting / idle) and focuses the chosen one. It was built on
`fzf` (like `pickSession` / `pickOSWindow`).

The motivating new requirement is a **live** picker: an animated spinner on
working agents and in-place status transitions (e.g. `working → waiting`) while
the picker is open. `fzf` is an external process that owns the terminal and only
re-reads its input on an explicit `reload()` binding — there is no clean way to
drive a per-frame spinner animation or push status changes into it from outside.

## Decision

`goto-agent` runs its **own** bubbletea v2 + lipgloss TUI instead of `fzf`. The
reusable core of the launcher's UI (query input + ranked, scrollable,
fuzzy-highlighted result list + border/footer chrome) is **extracted into a
shared embedded widget**, `internal/fuzzyfinder`, and the **launcher is migrated
onto it** so the abstraction has two real consumers (validating its seams rather
than guessing them).

- **Composition, not configuration.** The widget is *embedded* in each consumer's
  model (as the repo already embeds `textinput`); the consumer keeps its own
  top-level `Update`/`View` and composes its own chrome (the agent picker's
  preview pane, the launcher's history/actions) around it. The widget never
  knows previews or actions exist.
- **Boundary.** Widget owns input + selection + scroll + chrome + row-layout (via
  a selection-aware `renderRow(i, selected)` callback) and is index-based /
  type-agnostic. The consumer owns the item type, ranking/filtering, per-row
  rendering (incl. match highlight), preview, actions, and live-refresh tickers.
  Custom keybindings are added from the consumer's `Update` (which sees every key
  first), so the widget needs no changes to support them.
- **Live refresh** lives in the agent-picker consumer: a ~1s data tick re-runs
  `ListAgents` off the UI goroutine; a ~100ms spinner tick advances a shared
  braille spinner frame for working rows. Selection is preserved by agent **ID**
  across refreshes and the list re-sorts live (waiting agents surface to top).
- **fzf is kept** for the static `pickSession` / `pickOSWindow` pickers — they
  have no live-refresh need, so there is no reason to rewrite them.

## Consequences

- Two picker mechanisms coexist on purpose: self-owned TUI for the live
  goto-agent picker, `fzf` for the static session/os-window pickers.
- The fzf-specific agent plumbing is removed: `pickAgent`,
  `formatAgentChoices`, `parseAgentSelection`, `agentPreviewCommand`, and the
  hidden `__preview-agent` subcommand (`RenderAgentPreview` is now called
  in-process via a `Cmd`).
- The launcher migration is covered by its existing `model_test.go`, which must
  keep passing unchanged as proof the extraction preserved behavior.

## Alternatives considered

- **Keep fzf, drive it via `reload()`.** Rejected — `reload` re-reads the whole
  list on a binding; it cannot animate a spinner per frame or transition a single
  row's status in place, which is the entire point.
- **Extract the widget but leave the launcher on its private copy.** Rejected — a
  shared component with one consumer is a guessed abstraction; migrating the
  launcher is what forces the seams to be right.
- **Callback-configured picker that owns the loop.** Rejected in favour of an
  embedded widget — consumer-specific features (preview, history) don't fit a
  fixed callback set and leak into the shared model.
