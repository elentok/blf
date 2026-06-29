# Kitty goto-agent: live-refresh TUI fuzzy finder

Replace the `fzf`-based `goto-agent` picker with a self-owned bubbletea v2 +
lipgloss TUI so the picker can refresh live — an animated spinner on working
agents and in-place `working → waiting → idle` transitions while it is open.

Extract the launcher's reusable picker UI into a shared embedded widget
(`internal/fuzzyfinder`) and migrate the launcher onto it, with the agent picker
as the second consumer.

See `CONTEXT.md` (**fuzzy finder**, **goto-agent**, **agent window**, **agent
status**) and `docs/adr/0005-goto-agent-self-owned-tui.md`.

## Design decisions (locked)

- **Shared widget, not a refactor of two copies.** Extract the launcher's core
  (query input + ranked/scrollable/fuzzy-highlighted list + border/footer chrome)
  into `internal/fuzzyfinder`; migrate the launcher onto it so it has two real
  consumers.
- **Composition (embedded widget), not configuration.** Each consumer embeds the
  widget, keeps its own top-level `Update`/`View`, and composes its own chrome
  (agent preview pane, launcher history/actions) around it.
- **Boundary.** Widget = input + selection + scroll + chrome + row-layout via a
  selection-aware `renderRow(i, selected)` callback; **index-based / type-agnostic**
  (consumer keeps its own slice and calls `SetSelected(idx)`). Consumer = item
  type, ranking/filtering, per-row render (incl. highlight), preview, actions,
  live-refresh tickers.
- **Custom keybindings from the outside.** The consumer's `Update` sees every
  `KeyMsg` first, handles its own binds (launcher's `ctrl-x`/`ctrl-p`/…; future
  agent binds), then delegates input/nav keys to `widget.Update`. The widget must
  not assume it owns the whole keymap.
- **Refresh cadence (agent picker).** ~1s data tick re-runs `ListAgents` off the
  UI goroutine and sends fresh `[]Agent` back as a msg; ~100ms spinner tick
  advances a single shared frame counter.
- **Spinner.** Shared-counter braille spinner (U+2800 range) for `working` rows;
  static glyphs for `waiting`/`idle`.
- **Reorder + cursor.** Preserve selection by agent **ID** across refreshes;
  re-sort live (waiting → working → idle, then recency) so waiting agents jump to
  top with the cursor following. (Active query → fuzzy order instead.)
- **Preview pane.** Kept. Consumer renders widget left + preview right
  (`lipgloss.JoinHorizontal`). Fetch `get-text` via a `Cmd` on debounced
  (~80ms) selection-change, and refresh the selected agent's preview on the ~1s
  data tick.
- **Keys / exit.** type to filter; `up`/`down` + `ctrl-k`/`ctrl-j` to move;
  `enter` records selection + quits; `esc`/`ctrl-c` quit without focusing; `?`
  help. `GotoAgent` reads the selected id from the final model and runs
  `kitten @ focus-window --match id:N` after `p.Run()`.
- **Empty state in-TUI.** Drop the pre-launch short-circuit; render
  `No agent windows` inside the live TUI so a newly-spawned agent appears without
  relaunch.
- **Scope.** Remove fzf only from goto-agent. Leave `pickSession` /
  `pickOSWindow` on fzf.

## Tasks

- [x] Add `internal/fuzzyfinder` widget: query input (`textinput`), selection
      index, viewport scroll, border/input/separator/footer chrome and
      selected/normal/highlight styles, `renderRow(i, selected)` callback,
      index-based API (`Query()`, `Selected()`, `SetSelected(idx)`,
      `SetItemCount(n)`), input + nav keys only (`up`/`down`/`ctrl-j`/`ctrl-k`).
- [x] Add a reusable fuzzy match+highlight helper in `internal/fuzzyfinder`
      (wraps `github.com/sahilm/fuzzy`) that consumers may call from ranking.
- [x] Widget unit tests: nav keys, scroll-offset math, fuzzy-highlight ranges.
- [ ] Migrate `internal/launcher` onto the widget (embed it; keep launcher's
      ranking, history, currency/apps/scripts, icons, actions, tickers in the
      launcher model). Existing `launcher/model_test.go` must pass unchanged.
- [x] Add the agent-picker bubbletea model in `internal/kitty` (e.g.
      `agentpicker.go`): embed the widget, hold `[]Agent`, fuzzy-filter on query,
      status-first+recency sort, per-row render (status glyph/spinner, dir, title,
      dim agent name).
- [ ] Implement live refresh: ~1s data tick (`Cmd` re-running `ListAgents` →
      msg) + ~100ms spinner tick; ID-stable selection across refresh; live
      re-sort.
- [ ] Implement the preview pane: left widget + right preview via
      `lipgloss.JoinHorizontal`; debounced (~80ms) selection-change fetch +
      per-data-tick refresh of the selected agent (`RenderAgentPreview` via a
      `Cmd`).
- [x] In-TUI empty state (`No agent windows`); remove the pre-launch
      short-circuit in `GotoAgent`.
- [x] Wire `GotoAgent`: run the program, read selected id from final model, run
      `kitten @ focus-window --match id:N`.
- [x] Agent-picker model tests: fuzzy filtering, status-first sort, empty state,
      selection by id, navigation.
- [x] Remove dead fzf agent plumbing: `pickAgent`, `formatAgentChoices`,
      `parseAgentSelection`, `agentPreviewCommand`, `agentPreviewWindow`, the
      hidden `__preview-agent` subcommand + `PreviewAgent`. Keep
      `RenderAgentPreview`, `FormatAgents`, `ListAgents`.
- [x] Update CHANGELOG.md and README.
