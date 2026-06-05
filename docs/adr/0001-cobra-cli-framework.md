# ADR 0001: Migrate CLI dispatch to Cobra

**Status**: Accepted

## Context

`blf` started with a hand-rolled switch dispatcher in `cmd/cmd.go`. Flag parsing was manual per-command (looping `[]string`). This was fine for a small number of commands, but as the binary grows it creates recurring costs: every new command needs its own help text maintenance, flag parsing boilerplate, and has no shell completions.

## Decision

Migrate CLI dispatch to [Cobra](https://github.com/spf13/cobra) for structured subcommand routing, automatic `--help`, and shell completion generation (`fish`, `bash`).

### Specific decisions made

**Deps injection via closures.** Each command is constructed by a `newXxxCmd(d deps) *cobra.Command` function that captures `deps` in a closure. This preserves the existing testable deps pattern without introducing a global or a method receiver.

**SilenceErrors + SilenceUsage.** Both are set on the root command. `main.go` already prints errors; cobra's default double-printing is noisy. Usage is always available via `--help`.

**SetOut/SetErr wired to deps.** `root.SetOut(d.stdout)` and `root.SetErr(d.stderr)` ensure cobra's help/usage output routes through `deps`, keeping tests that capture output working correctly.

**`version` kept as a subcommand.** `blf version` is documented and in muscle memory. `root.Version` also adds `--version` / `-v`. Both coexist.

**`tmux-targets` flags migrated into cobra.** Cobra owns parsing of `--popup` (bool) and `--target` (string). `tmuxtargets.Execute` signature changes from `Execute([]string) error` to `Execute(popup bool, target string) error`.

**`kitty` sub-subcommands fully migrated.** Internal kitty functions (`Targets`, `DeleteSession`, `GotoOSWindow`, etc.) change from `f(args []string, d Deps)` to typed signatures. Cobra handles all flag/arg parsing; internal functions receive typed values. `--overlay` (used by `LaunchOverlay` for self-invocation) becomes a declared cobra flag.

## Alternatives considered

**Keep hand-rolled dispatch, add help/completions manually.** Rejected — the maintenance burden grows with each new command and completions would still need a separate maintained file.

**`urfave/cli`.** Similar feature set but less ecosystem traction and weaker completion support than Cobra.

**Passthrough for `tmux-targets` / `kitty` internals.** Cobra reconstructing `[]string` from parsed flags to pass to internal functions that re-parse them was considered and rejected as redundant. Typed signatures are cleaner.
