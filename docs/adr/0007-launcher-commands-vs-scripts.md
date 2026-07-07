# ADR 0007: Built-in launcher actions are commands (in-process), not scripts (subprocess)

**Status**: Accepted

## Context

The launcher already had a **script** mechanism (`internal/launcher/scripts`): fuzzy-matched,
name-triggered entries whose `Body` runs as an external process (`bash -c` or `osascript`).
Built-in scripts like `cleanurl` were just thin wrappers that shelled out to the `blf` CLI
itself (`blf clean-url --clipboard`) to reach logic that already lives in this same Go binary.

We wanted to add `reload` (reindex the app cache — previously only reachable via the
ctrl+shift+r keymap, which calls `apps.Reindex` in-process) as a fuzzy-matched, name-triggered
launcher entry alongside the keymap. The obvious move was to reuse the scripts mechanism, since
it already does "fuzzy-matched, name-triggered, runs something."

## Decision

Built-in actions that call Go code already living in this binary are **commands**
(`internal/launcher/commands`), not scripts. A command's `Run` calls a Go function directly —
no `exec.Cmd`, no subprocess. Scripts remain for genuinely external bash/osascript snippets,
user-configurable and overridable; commands are a small hardcoded, non-configurable list.

As part of this, `cleanurl` moved from a built-in script (`blf clean-url --clipboard`) to a
command, and its core logic (URL cleaning + clipboard I/O) was extracted from `cmd/clean_url.go`
into `internal/cleanurl` so both the CLI subcommand and the new launcher command can call it
directly without an import cycle (`cmd` already imports `internal/launcher/*`).

## Alternatives considered

- **Add `reload` as a script that shells out to a new `blf reindex-apps` CLI subcommand**
  (matching how `cleanurl` used to work). Rejected — spawns a whole OS process just to call a
  function (`apps.Reindex`) that's already reachable in-process, and adds a CLI subcommand whose
  only purpose is to be a shim for the launcher.

## Consequences

- Two similar-looking mechanisms now coexist (scripts vs. commands), distinguished by execution
  model, not by how they're triggered or displayed — see `CONTEXT.md`'s **script**/**command**
  glossary entries.
- New built-ins default to being commands when their logic is already Go code in this repo;
  scripts stay reserved for actual external processes / user-authored snippets.
