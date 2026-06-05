# PRD: Migrate CLI dispatch to Cobra

## Problem Statement

As `blf` grows with more commands, the hand-rolled switch dispatcher in the `cmd` package
accumulates recurring costs: every new command requires manually maintaining help text, writing
bespoke flag-parsing loops, and shell completions are entirely absent. The current structure does
not scale.

## Solution

Migrate CLI dispatch to the Cobra framework. Cobra provides structured subcommand routing,
automatic `--help` generation, and built-in shell completion generation for fish and bash. The
`deps` injection pattern is preserved via closures. Internal packages (`tmuxtargets`, `kitty`) have
their string-slice arg APIs replaced with typed function signatures so arg parsing lives in exactly
one place (cobra), not two.

## User Stories

1. As a `blf` user, I want `blf --help` to show a structured list of all subcommands with
   descriptions, so that I can discover commands without reading the README.
2. As a `blf` user, I want `blf <command> --help` to show flags and usage for that specific
   command, so that I don't need to run the command incorrectly to find out the usage.
3. As a `blf` user, I want `blf kitty --help` to list all kitty sub-subcommands, so that I can
   discover kitty functionality in one place.
4. As a `blf` user, I want `blf kitty <subcommand> --help` to show flags and positional args for
   that subcommand, so that I know how to use it without reading source code.
5. As a `blf` user, I want `blf completion fish` to emit a fish completion script, so that I can
   get tab-completion in fish by sourcing it.
6. As a `blf` user, I want `blf completion bash` to emit a bash completion script, so that I can
   get tab-completion in bash.
7. As a `blf` user, I want `blf --version` and `blf -v` to print the version, so that I can verify
   which binary is installed.
8. As a `blf` user, I want `blf version` to still work as a subcommand, so that existing scripts
   and muscle memory are not broken.
9. As a `blf` user, I want `blf qs` to continue working as an alias for `blf querystring`, so that
   existing usage is preserved.
10. As a `blf` user, I want unknown commands or missing required args to print a helpful error
    message (not a full usage dump), so that the output is not overwhelming.
11. As a `blf` contributor, I want to add a new command by writing a single `newXxxCmd(d deps)
    *cobra.Command` function, so that adding commands does not require touching a central switch
    statement or maintaining a separate help string.
12. As a `blf` contributor, I want internal packages (`tmuxtargets`, `kitty`) to accept typed
    parameters instead of raw string slices, so that call sites are clear and arg parsing is not
    duplicated.

## Implementation Decisions

### Deps injection via closures

Each cobra command is constructed by a function `newXxxCmd(d deps) *cobra.Command` that captures
`deps` in a closure. The `execute(args []string, d deps) error` function (called by tests) builds
the root command, wires output, sets args, and calls `Execute()`. No global state.

### Root command configuration

- `SilenceErrors = true` — `main.go` already prints errors; cobra must not double-print.
- `SilenceUsage = true` — usage is available via `--help`; it must not auto-print on every error.
- `root.SetOut(d.stdout)` and `root.SetErr(d.stderr)` — all cobra output (help, completions) routes
  through `deps` so tests that capture output continue to work.
- `root.Version = version` — wires `--version` / `-v` automatically.

### `tmuxtargets` typed signature

`Execute([]string) error` becomes `Execute(popup bool, target string) error`. The `--popup` /
`--target` flags are declared on the cobra `tmux-targets` command; `RunE` passes parsed values
directly. Internal `parsePopupArgs` is removed. The `deps.runTargets` field type changes
accordingly.

### `kitty` typed signatures

All kitty sub-subcommand functions that currently take `args []string` are changed to typed
parameters:

- `GotoOSWindow(id string, d Deps)` — `id` is optional; empty string means "pick interactively"
- `DeleteSession(overlay bool, d Deps)` — `overlay` switches between launcher and overlay mode
- `Targets(overlay bool, target string, d Deps)` — `target` is optional window ID; `overlay` tells
  it to resolve against the overlay parent
- `PreviewSession(path string, d Deps)`
- `DeleteSessionFile(path string, d Deps)`
- `EditSessionFile(path string, d Deps)`
- `NewSession`, `SessionsCommand`, `ListSessionChoices` — drop the unused `[]string` param entirely

Internal arg-parsing logic (`resolveTargetMatch`, `parsePopupArgs`, switch/case on `args`) is
removed from `internal/kitty`; cobra owns that layer.

`--overlay` is declared as a cobra flag on the `kitty targets` and `kitty delete-session`
subcommands. It is passed by `LaunchOverlay` when kitty re-invokes `blf` as an overlay window — it
is a real cobra flag, not a hidden internal detail.

### `qs` alias

`querystring` cobra command declares `Aliases: []string{"qs"}`.

### kitty sub-subcommand structure

`newKittyCmd(d deps)` returns a cobra command with sub-subcommands added via `AddCommand`. Each
sub-subcommand is its own `newKittyXxxCmd(d deps)` closure.

## Testing Decisions

Good tests verify external behavior through the public API, not internal parsing logic. Since arg
parsing moves into cobra, tests should not reach inside cobra internals — they should call the
package's entry point with args and assert on outputs or side-effects via `deps` stubs.

### `internal/tmuxtargets`

All existing tests are updated to call `Execute(popup bool, target string)` directly with typed
values instead of string slices. The `parsePopupArgs` unit tests are removed (the function no
longer exists).

### `internal/kitty`

All existing tests are updated to call typed function signatures. Tests that previously constructed
`[]string{"--overlay"}` args now pass `overlay: true` directly. Behavior assertions (output
content, command calls) remain unchanged.

### `cmd` package

Existing routing tests in `cmd_test.go` continue to work unchanged — they call
`execute(args, deps)` which now builds the cobra tree internally. New tests may be added for
cobra-specific behavior (e.g. `--help` output routing to `d.stdout`, `--version` output).

## Out of Scope

- Custom `ValidArgs` / `RegisterFlagCompletionFunc` for rich per-argument completions (e.g.
  completing kitty window IDs). The cobra `completion` subcommand will work for command and flag
  names; value completions are a follow-up.
- Migrating `tmux-targets`'s internal `runTopLevel` / `runPopupMode` to export a cleaner API
  beyond the typed `Execute` signature.
- Any new commands beyond what currently exists.

## Further Notes

See `docs/adr/0001-cobra-cli-framework.md` for the full record of design trade-offs explored
during planning.

The migration should be done in three sequential tasks to keep each diff reviewable:
1. `internal/tmuxtargets` signature refactor
2. `internal/kitty` signature refactor
3. `cmd` cobra migration (depends on 1 and 2)
