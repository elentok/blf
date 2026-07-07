# Launcher commands: reload + cleanurl

## Problem Statement

The launcher (`blf launcher`) has two similar-looking but distinct ways to trigger a named
action: the ctrl+shift+r keymap (which reindexes the app cache in-process) and the **script**
mechanism (fuzzy-matched, name-triggered entries that shell out to an external process). Neither
is a good fit for exposing "reindex apps" as something you can type and select, and the existing
`cleanurl` script — which shells out to `blf clean-url --clipboard` just to reach Go code that's
already sitting in the same binary — pays for a subprocess spawn for no reason.

There's no first-class way to trigger reindexing except the keymap, and no way to make a
built-in, non-configurable action ("run this Go code by name") fuzzy-matchable in the launcher
without pretending it's a user script.

## Solution

Introduce **commands** — a new launcher source distinct from **scripts** (see `CONTEXT.md`'s
**script**/**command** glossary entries and `docs/adr/0007-launcher-commands-vs-scripts.md`).
A command is a built-in, hardcoded, fuzzy-matched launcher entry whose action calls Go code
directly (no `exec.Cmd`), as opposed to a script, which is user-configurable and always shells
out to an external process.

Ship two commands: `reload` (reindex the app cache — identical behavior to today's ctrl+shift+r
keymap, now also reachable by typing "reload" and hitting Enter) and `cleanurl` (migrated off
the scripts mechanism into a command, since its logic already lives in this binary). `cleanurl`
gains a new completion UX: on success it hides the launcher and shows a system notification
(new capability); on failure it stays open with a status-bar error, consistent with existing
script-error handling.

## User Stories

1. As a launcher user, I want to type "reload" and hit Enter, so that I can reindex my app
   cache without memorizing the ctrl+shift+r keymap.
2. As a launcher user, I want ctrl+shift+r to keep working exactly as it does today, so that
   my existing muscle memory isn't broken by this change.
3. As a launcher user, I want reload's completion feedback to look the same whether I triggered
   it via keymap or by typing "reload", so that the two trigger paths feel like the same feature.
4. As a launcher user, I want to type "cleanurl" and hit Enter, so that I can clean the URL
   currently on my clipboard.
5. As a launcher user, I want the launcher to disappear and show me a system notification when
   cleanurl succeeds, so that I get quick confirmation without the launcher lingering on screen.
6. As a launcher user, I want the launcher to stay open with a clear status-bar error when
   cleanurl fails (e.g. clipboard doesn't contain a URL), so that I can see what went wrong and
   retry.
7. As a maintainer, I want built-in actions that call existing Go code to be implemented as
   commands (not scripts), so that we don't spawn subprocesses to reach code already in-process.
8. As a maintainer, I want scripts to remain the mechanism for genuinely external/user-authored
   bash or osascript snippets, so that the two mechanisms don't blur together.
9. As a maintainer, I want `cleanurl`'s core logic (URL cleaning, clipboard I/O) extracted into
   a package importable from both `cmd` and `internal/launcher`, so that neither the CLI nor the
   launcher command has to duplicate that logic, and so we avoid an import cycle (`cmd` already
   imports `internal/launcher/*`).
10. As a maintainer, I want `platform.ShowNotification` to work on both mac and Linux, so that
    command-completion notifications aren't a mac-only capability going forward.
11. As a maintainer, I want new commands added later (beyond reload/cleanurl) to follow the same
    `Command{Name, Icon, Run}` shape, so that extending the builtin list stays a one-line addition.

## Implementation Decisions

- **New `internal/cleanurl` package.** Owns `cleanURL(string) string` (pure, migrated verbatim
  from `cmd/clean_url.go`) plus `RunClipboard() error`, which reads the clipboard, cleans the
  URL, writes it back, and owns all clipboard I/O for this flow. Clipboard access goes through
  `internal/platform` (`ReadClipboardText`/`CopyText`), matching how `cmd`'s `deps` struct
  already wires those same platform functions — no need to route clipboard I/O through
  `launcher.Config`.
- **`cmd/clean_url.go` becomes a thin wrapper**: `runCleanURL`'s clipboard branch delegates to
  `internal/cleanurl.RunClipboard`; the CLI subcommand's plain-argument path (clean a URL passed
  as an arg, print to stdout) can keep calling `internal/cleanurl.cleanURL` directly. No CLI
  contract change (`blf clean-url [url]` / `blf clean-url --clipboard` behave identically from
  the outside).
- **`internal/launcher/scripts`**: remove the `cleanurl` entry from `Builtins`. The `scripts`
  package itself (execution model, `Merge`, `FilterForPlatform`) is unchanged — it remains the
  mechanism for external/user-authored bash/osascript snippets.
- **New `internal/launcher/commands` package**: `Command{Name string, Icon string, Run func()
  tea.Cmd}`. A hardcoded builtin list (not user-configurable, unlike scripts): `reload` (wraps
  the existing `ReindexCmd(homeDir, cachePath)` — same function the ctrl+shift+r keymap already
  calls) and `cleanurl` (wraps a new `CleanURLCmd()` that calls `cleanurl.RunClipboard` inside a
  `tea.Cmd` closure, returning a new `CleanURLDoneMsg{Err error}`).
- **New `ActionType` and `IconRole` enum values**: `ActionCommand` (in `provider.go`'s
  `ActionType` const block, alongside `ActionCopy`/`ActionLaunch`/`ActionRun`/etc.) and
  `IconRoleCommand` (in the `IconRole` const block).
- **New `CommandsProvider`** (`internal/launcher`, new file): mirrors `ScriptsProvider`'s shape
  — fuzzy `Query(input string) []Result` matching by command name, `Find(name string)` to look
  up a command for dispatch, `LookupResult(action Action) (Result, bool)` satisfying
  `TargetLookupProvider` (for launcher-history re-display). Emits
  `Result{Action: Action{Type: ActionCommand, Target: c.Name}, Icon: IconRoleCommand, ...}`.
- **`cmd/launcher.go` wiring**: construct `CommandsProvider` (needs `homeDir`/`cachePath`,
  closed over for the `reload` command's `Run`), pass it into the launcher config alongside the
  existing `ScriptsProvider`/`AppsProvider` construction.
- **`model.go` Enter-handling**: new `ActionCommand` case, mirroring the existing `ActionRun`
  case — look up the command by name via `CommandsProvider.Find`, record history/learned-rank
  the same way `ActionRun` does, return the command's `Run()` result.
- **`model.go` message handling**:
  - `reload`'s completion is unchanged — it still produces `AppsReindexedMsg`, handled exactly
    as it is today (clears status, updates `AppsProvider`'s index silently on success). This is
    deliberate: reload's completion UX must be identical regardless of trigger path (keymap vs.
    command pick).
  - New `CleanURLDoneMsg` case: on `Err == nil`, call `m.resetAndHide()` (existing helper, same
    one used for other successful sync actions) and `platform.ShowNotification(title, message)`.
    On `Err != nil`, set `m.status = "cleanurl error: ..."` and leave the launcher open —
    consistent with how script errors are already surfaced.
- **New `platform.ShowNotification(title, message string) error`** in `internal/platform`:
  switches on `runtime.GOOS` (same pattern `scripts.go` already uses for platform-specific
  behavior) — `osascript -e 'display notification ...'` on mac, `notify-send` on Linux.
- **Terminology**: `CONTEXT.md` already updated with **script** and **command** glossary
  entries, and the **launcher source** entry now lists "script, or command." No further
  glossary changes anticipated.
- **ctrl+shift+r keymap**: unchanged — still calls `ReindexCmd` directly, same as today. It does
  not route through the new `Command`/`CommandsProvider` machinery; the two trigger paths
  (keymap, command pick) both bottom out in the same `ReindexCmd` function, but the keymap's own
  code path in `model.go` is left as-is (not refactored to dispatch through
  `CommandsProvider.Find("reload")`).

## Testing Decisions

Good tests here exercise externally observable behavior — inputs to a function/`Update()` call
and the resulting output/state/message — not internal call sequencing.

- **`internal/cleanurl`**: migrate `TestCleanURL`'s table-driven cases from
  `cmd/clean_url_test.go` to `internal/cleanurl` (pure function, no I/O). Add tests for
  `RunClipboard` using the same injectable-function pattern `cmd/clean_url_test.go` already uses
  for `deps.readClipboard`/`deps.copyText` (success path: reads, cleans, writes back; error path:
  clipboard read failure propagates). Prior art: `cmd/clean_url_test.go`'s `TestCleanURL` and
  `TestRunCleanURL`.
- **`internal/launcher/commands` + `CommandsProvider`**: test fuzzy `Query` matching and `Find`
  lookup behavior for the builtin list, following `internal/launcher/scripts/scripts_test.go`'s
  style for `Merge`/`FilterForPlatform` (table-driven, asserting on the resulting slice/lookup
  result — not on internal execution).
- **`model.go` `ActionCommand`/`CleanURLDoneMsg` handling**: test via `Update()` calls, following
  `model_test.go`'s existing style (construct a `Model`, send a message, assert on resulting
  `status`, whether `resetAndHide` was triggered, whether history/learned-rank was recorded) —
  same pattern already used for `AppsReindexedMsg` and script-completion messages.
- **`platform.ShowNotification`**: test command/argument construction (e.g. the `osascript`/
  `notify-send` invocation shape for a given `runtime.GOOS`) via an injectable exec-runner seam,
  rather than actually invoking the OS notification system — mirrors how `scripts.go` doesn't
  unit-test actual process output, only the constructed `exec.Cmd`. Low-value beyond basic
  argument-shape coverage; keep this test light.

## Out of Scope

- Making `commands` user-configurable (unlike scripts, they are a fixed, hardcoded list — no
  config-file surface).
- Adding any commands beyond `reload` and `cleanurl` in this epic.
- Changing the ctrl+shift+r keymap's own code path to dispatch through
  `CommandsProvider`/`Command.Run` — it keeps calling `ReindexCmd` directly.
- Notification styling/richness (icons, actions, sound) beyond a plain title+message.
- Any change to `blf clean-url`'s CLI contract/flags.

## Further Notes

Full interactive design discussion (including the scripts-vs-commands trade-off, the import-cycle
constraint that motivated extracting `internal/cleanurl`, and the notification/error-UX decisions)
happened via `/grill` and is captured in `docs/adr/0007-launcher-commands-vs-scripts.md` and the
**script**/**command** entries in `CONTEXT.md`. A prior, less granular implementation checklist
exists at `docs/plans/launcher-reload-command.md` — this PRD supersedes it as the source of truth
for beads task breakdown.
