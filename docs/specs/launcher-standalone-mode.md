# Launcher standalone mode

## Problem Statement

The launcher assumes it is always running inside a Kitty or Ghostty quick terminal: on `esc` (and
every other path that resets the launcher — a successful `cleanurl`, an AI run completing, etc.) it
calls `HideTerminal`, which shells out to hide/toggle the quick terminal window. When the launcher is
run standalone — in a regular terminal window, for development, debugging, or just preferring a
persistent window — that hide command still fires. Depending on environment (e.g. a stray
`GHOSTTY_QUICK_TERMINAL` env var inherited from a parent process, or `kitten quick-access-terminal`
targeting the shared `quick` instance group) this can toggle or hide an unrelated real quick-terminal
window instead of doing nothing, which is surprising and unwanted when the user deliberately launched
the app outside that context.

## Solution

A new `--standalone` flag on `blf launcher` disables quick-terminal integration entirely: no hide
command is wired up, so every reset path (`esc`, `cleanurl`, AI-run completion, etc.) just resets the
launcher's in-memory state and leaves the terminal window alone, exactly like today's `HideTerminal ==
nil` case that the tests already cover (`TestResetAndHideNilHideTerminal`).

Without `--standalone`, behavior is unchanged: `launcherHideTerminal` is wired up as it is today.

## User Stories

1. As a user running `blf launcher --standalone` in a regular terminal window, I want `esc` to just
   clear the query, so that pressing escape doesn't fire an OS-level command against some other
   window.
2. As a user running `blf launcher --standalone`, I want every other reset path (successful
   `cleanurl`, an AI run finishing while idle, etc.) to also skip the hide command, so that
   standalone mode is consistent regardless of which action triggered the reset.
3. As a user running `blf launcher` without `--standalone`, I want today's Kitty/Ghostty
   quick-terminal hide behavior unchanged, so that the new flag is opt-in and doesn't regress the
   primary use case.

## Implementation Decisions

- **Seam**: `newLauncherCmd` in `cmd/launcher.go` is the seam. It gains a new `bool` flag,
  `--standalone` (default `false`), alongside the existing `--no-border` flag, with help text along
  the lines of `"run standalone (outside a Kitty/Ghostty quick terminal): don't hide/toggle the
  terminal on reset"`.
- When `--standalone` is set, the `ModelConfig.HideTerminal` field passed to `launcher.NewModel` is
  left `nil` instead of being set to `launcherHideTerminal(d)`. `launcherHideTerminal(d)` is simply
  not called in that case — no new function or wrapper needed, since `Model.resetAndHide` already
  treats a `nil` `HideTerminal` as "nothing to hide" (see `TestResetAndHideNilHideTerminal`).
- No change to `launcherHideTerminal`, `Model`, or `resetAndHide`.
- **Seam**: the `standalone` decision itself is pulled out of `RunE`'s inline `ModelConfig`
  construction into a small standalone function, e.g. `launcherHideTerminalFor(d deps, standalone
  bool) func() error`, that returns `nil` when `standalone` is `true` and `launcherHideTerminal(d)`
  otherwise. `RunE` calls `HideTerminal: launcherHideTerminalFor(d, standalone)` instead of the
  current `HideTerminal: launcherHideTerminal(d)`. This gives tests a seam to call directly without
  going through the blocking `tea.NewProgram(m).Run()` in `RunE`.

## Testing Decisions

- Only test external behavior: what `ModelConfig.HideTerminal` ends up being, not internals.
- `cmd/launcher_test.go` already tests `launcherHideTerminal` directly (`TestLauncherHideTerminal`);
  add an equivalent table test for the new `launcherHideTerminalFor(d, standalone)` asserting it
  returns `nil` when `standalone` is `true`, and returns `launcherHideTerminal(d)`'s behavior
  (non-nil, delegating to the same Ghostty/Kitty selection) when `standalone` is `false`.
- No new test needed in `internal/launcher` — `resetAndHide`'s nil-`HideTerminal` behavior is already
  covered by `TestResetAndHideNilHideTerminal`.

## Out of Scope

- No change to how the launcher detects Ghostty vs Kitty when `--standalone` is *not* set.
- No config-file setting for this — CLI flag only, matching `--no-border`.
- No change to `esc`'s AI-prompt-mode behavior (`exitAIPromptMode`) — that path never calls
  `HideTerminal` today and isn't affected either way.
- No attempt to auto-detect "am I running inside a quick terminal" and imply `--standalone`
  automatically — this is an explicit, user-set flag.

## Further Notes

Scope was settled by grilling before writing this spec — no map/tickets were needed since the
destination had no open decisions left, only implementation detail. The fix is a single conditional at
the `cmd/launcher.go` call site: `resetAndHide` already no-ops cleanly when `HideTerminal` is `nil`.
