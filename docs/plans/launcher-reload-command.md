# Launcher reload/commands plan

Design settled via `/grill`. See `docs/adr/0007-launcher-commands-vs-scripts.md` and
`CONTEXT.md`'s **script**/**command** entries for the reasoning.

## Extract cleanurl core logic

- [ ] Create `internal/cleanurl` package: move `cleanURL`, `redirectWrappers`,
      `trackingParams` from `cmd/clean_url.go`
- [ ] Add `RunClipboard() error` to `internal/cleanurl` — owns clipboard I/O via
      `platform.ReadClipboardText`/`platform.CopyText` (mirrors today's `runCleanURL` clipboard
      branch)
- [ ] Update `cmd/clean_url.go`'s `runCleanURL`/cobra command to call into `internal/cleanurl`
      for the clipboard path (non-clipboard/plain-arg path can stay as-is or also delegate)
- [ ] Remove `cleanurl` from `scripts.Builtins` (`internal/launcher/scripts/scripts.go`)

## Add notification support

- [ ] Add `platform.ShowNotification(title, message string) error` to `internal/platform`
      — `osascript -e 'display notification ...'` on mac, `notify-send` on Linux (switch on
      `runtime.GOOS`, matching `scripts.go`'s pattern)

## Add ActionType / IconRole

- [ ] Add `ActionCommand` to the `ActionType` enum (`internal/launcher/provider.go`)
- [ ] Add `IconRoleCommand` to the `IconRole` enum (`internal/launcher/provider.go`)

## Add commands package + provider

- [ ] Create `internal/launcher/commands` package: `Command{Name, Icon string; Run func() tea.Cmd}`
- [ ] Builtin list: `reload` (wraps `ReindexCmd(homeDir, cachePath)`, same as the ctrl+shift+r
      keymap) and `cleanurl` (wraps a new `CleanURLCmd()` that calls `cleanurl.RunClipboard`
      and emits a new `CleanURLDoneMsg{Err error}`)
- [ ] `CommandsProvider` in `internal/launcher` (new file, e.g. `commandsprovider.go`): fuzzy
      `Query`, `Find(name)`, `LookupResult(action)` — mirrors `ScriptsProvider`

## Wire into model + launcher construction

- [ ] `cmd/launcher.go`: construct `CommandsProvider` (needs `homeDir`/`cachePath` for `reload`),
      pass into launcher config
- [ ] `model.go`: `ActionCommand` case in Enter-handling, mirrors `ActionRun` — looks up by name,
      returns `Run()`'s `tea.Cmd`
- [ ] `model.go`: handle `CleanURLDoneMsg` — on success, `resetAndHide()` +
      `platform.ShowNotification(...)`; on failure, set `status = "cleanurl error: ..."`,
      launcher stays open (consistent with existing script-error handling)
- [ ] `reload`'s existing `AppsReindexedMsg` handling stays untouched — identical completion UX
      whether triggered via keymap or command-list pick

## Docs

- [x] `CONTEXT.md`: add **script** and **command** glossary entries, extend **launcher source**
- [x] `docs/adr/0007-launcher-commands-vs-scripts.md`
- [ ] `CHANGELOG.md` entry once implemented
