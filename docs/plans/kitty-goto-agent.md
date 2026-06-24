# Kitty goto-agent

Add `blf kitty goto-agent` (interactive picker that focuses an agent window) and
`blf kitty list-agents [--json]` (non-interactive listing, future source of truth
for the nvim send-to-agent feature).

See `CONTEXT.md` for the glossary: **agent window**, **agent status**,
**goto-agent**, **list-agents**.

## Design decisions (locked)

- **Scope**: all agent windows across every OS window and session.
- **Identity**: whole command-**word** matching. Primary = first token of
  `last_reported_cmdline`; backup = a foreground process command-word / argv0
  basename. Never substring-match the full cmdline (a path like
  `/tmp/claude-501/…` must not count as a Claude agent). Known agents:
  `claude`, `codex`, `opencode`, `cursor-agent`.
- **Status**: working vs idle only. Title-only detection — leading braille
  spinner char (U+2800–U+28FF) = working, else idle. No screen scraping, no
  "waiting"/"blocked" state.
  - Known limitation: OpenCode has no title status signal, so it always reads
    idle. Revisit later (would require `kitty @ get-text` for opencode windows).
- **Picker**: `fzf`, same patterns as the sessions picker. Row layout:
  `<status>  <dir>  <title>  <agent(dim)>` where dir = window cwd basename.
  Sort working-first, then most-recently-focused (`last_focused_at`).
  Kitty window id rides in a hidden tab-delimited field.
- **Preview**: lazy fzf preview showing a screen snapshot of the highlighted
  agent (`kitty @ get-text --match id:N --extent screen`).
- **Focus**: `kitten @ focus-window --match id:N` (focuses window + its tab +
  OS window).
- **Current window**: drop whichever window is currently focused. In the
  intended new-tab/overlay launch the focused window is the picker itself, so
  this is a no-op; if run inline from an agent it correctly drops that one.
- **Empty result**: `No agent windows` (sessions-picker style).
- No `--fast` flag (title-only is already the only path).

## Tasks

- [x] Add `internal/kitty/agents.go`: `Agent` struct (window id, agent name,
      status, dir, title, session, last_focused_at) + `ListAgents(d Deps)
      ([]Agent, error)` built on the existing `ListOSWindows` / `ParseOSWindows`.
- [x] Implement identity detection (word/basename matching) with unit tests
      covering the `claude-501` path false-positive and the
      `/bin/sh /usr/bin/command claude` wrapper.
- [x] Implement title-only status detection (braille-prefix → working) with
      unit tests, including the OpenCode-always-idle case.
- [x] Implement sorting (working-first, then recency) + row formatting with the
      hidden id field; unit-test the formatter.
- [x] Add `list-agents` command (`ListAgents` + `--json`) wired in `cmd/kitty.go`.
- [x] Add `goto-agent` command: build the agent list, run the fzf picker with
      screen-snapshot preview, focus via `focus-window`, handle the empty case.
- [x] Add a hidden preview subcommand for the fzf preview pane (mirrors
      `__preview-session`), or reuse `kitty @ get-text` directly.
- [x] Update CHANGELOG.md and README.
- [ ] (Optional, later) Migrate nvim `find-agent.lua` to call
      `blf kitty list-agents --json`.
