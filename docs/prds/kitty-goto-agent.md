# PRD: Kitty goto-agent

## Problem Statement

I run several AI coding agents (Claude, Codex, OpenCode) at once, each in its own
Kitty window scattered across tabs, OS windows, and sessions. I have no fast way
to see which agents are open, which are still working versus done, or to jump
straight to one. Today I rely on an nvim send-to-agent feature whose detection
sometimes misses an agent entirely, so I can't trust it to find them all.

## Solution

A `blf kitty goto-agent` command that opens a picker listing every **agent
window** across all OS windows and sessions, each annotated with its **agent
status** (working or idle), and focuses the one I choose. A companion
`blf kitty list-agents [--json]` exposes the same data non-interactively so it
can become the single source of truth that other tools (e.g. the nvim
send-to-agent feature) call instead of reimplementing detection.

## User Stories

1. As a developer juggling several agents, I want to see all open agent windows
   in one picker, so that I don't have to hunt through tabs and OS windows.
2. As a developer, I want the picker to include agents from every session and OS
   window, so that I can reach an agent no matter where I left it.
3. As a developer, I want each agent row to show its working directory, so that I
   can identify the project at a glance.
4. As a developer, I want each row to also show the window title (the agent's
   task summary), so that I can tell two agents in the same repo apart.
5. As a developer, I want the agent type shown but de-emphasised, so that I see
   it without it competing with the directory and title.
6. As a developer, I want each row to show whether the agent is working or idle,
   so that I can decide whether to wait or jump in.
7. As a developer, I want working agents listed first, then by most recently
   focused, so that the most relevant agents are at the top.
8. As a developer, I want to select an agent and have Kitty focus that exact
   window (pulling its tab and OS window forward), so that I land precisely where
   the agent is.
9. As a developer, I want a preview of the highlighted agent's current screen, so
   that I can confirm which one it is before jumping.
10. As a developer, I want to bind `blf kitty goto-agent` to a Kitty shortcut
    that opens it in a new tab/overlay, so that I can invoke it from anywhere.
11. As a developer invoking the picker, I don't want the picker window itself (or
    the window I'm currently in) to appear as a target, so that the list only
    shows places worth jumping to.
12. As a developer with no agents running, I want a clear "No agent windows"
    message, so that I'm not confronted with an empty picker.
13. As a developer, I want agent detection to never misclassify a plain shell as
    an agent just because a path contains an agent's name, so that the list is
    trustworthy.
14. As a developer, I want detection to find an agent even when it runs behind a
    shell wrapper (e.g. `/bin/sh /usr/bin/command claude`), so that no real agent
    is missed.
15. As a tool author, I want `blf kitty list-agents --json` to return a stable,
    machine-readable list, so that I can drive the nvim send-to-agent feature
    from it.
16. As a developer, I want `blf kitty list-agents` (no flag) to print a readable
    list, so that I can sanity-check detection from the shell.

## Implementation Decisions

- **New commands** under the existing `kitty` cobra group:
  - `goto-agent` — interactive picker that focuses the chosen agent window.
  - `list-agents` — non-interactive listing; `--json` for machine output.
  - `__preview-agent <id>` — hidden subcommand for the fzf preview pane,
    mirroring `__preview-session`.
- **Deep detection module** (`internal/kitty/agents.go`) built on the existing
  `ListOSWindows` / `ParseOSWindows` and the `Window` model. Pure functions so it
  is testable without invoking Kitty:
  - `detectAgentName(Window) (name string, ok bool)` — identity by whole command
    **word** matching. Primary signal: first token of `last_reported_cmdline`.
    Backup: a foreground process command-word / argv0 basename. Known agents:
    `claude`, `codex`, `opencode`, `cursor-agent`. Substring matching over the
    full cmdline is explicitly forbidden (a path such as `/tmp/claude-501/…` must
    not match).
  - `detectStatus(title) Status` — title-only. A leading braille-spinner rune
    (U+2800–U+28FF) means `working`; anything else means `idle`. No screen
    scraping. Known limitation: OpenCode has no title status signal and therefore
    always reports `idle`.
  - `ListAgents(d) ([]Agent, error)` — orchestrates listing, detection, dropping
    the currently-focused window, and sorting (working-first, then most-recently
    focused via `last_focused_at`).
  - `Agent` struct fields: window id, agent name, status, dir (cwd basename),
    title, session, last-focused-at.
- **Row formatting** (pure): `<status>  <dir>  <title>  <agent(dim)>` where dir is
  the window cwd basename and the agent name is dimmed. The Kitty window id rides
  in a hidden, tab-delimited field consumed by fzf (`--with-nth` / `--delimiter`
  pattern from the sessions picker). Status renders as a glyph: `●` working,
  `○` idle (dim).
- **Picker** (`pickAgent`) mirrors `pickSession`: `fzf --layout=reverse --ansi`,
  the shared navigation binds, a preview window driven by `__preview-agent {id}`.
  Empty agent list short-circuits to a `No agent windows` message in the
  sessions-picker style.
- **Focus**: `kitten @ focus-window --match id:N` (focuses the window plus its
  tab and OS window). This differs from `goto-os-window`, which focuses a tab.
- **Current-window handling**: drop whichever window is currently focused. In the
  intended new-tab/overlay launch the focused window is the picker itself, so this
  is a no-op and all agents show; run inline from an agent, it drops that one.
- **JSON contract** (`list-agents --json`): an array of objects, one per agent,
  with fields mirroring the `Agent` struct (window id, agent, status, dir, title,
  session). Field names are the stable contract for external callers.
- No `--fast` flag — title-only detection is already the only path, so there is
  no slower path to opt out of.

## Testing Decisions

- Tests assert **external behavior**, not internals: given parsed Kitty windows
  (or stubbed `RunCommand` output), assert the produced agent list, statuses,
  ordering, formatted rows, and JSON — never private helper internals.
- **Prior art**: `internal/kitty/targets_test.go` and `windows_test.go` — table
  tests that feed JSON / stub `Deps.RunCommand` and assert outputs.
- **Detection core** (confirmed): identity matching including the `claude-501`
  path false-positive (must NOT match) and the `/bin/sh /usr/bin/command claude`
  wrapper (must match); status braille-prefix parsing including the
  OpenCode-always-idle case; drop-focused and working-first+recency sorting.
- **Row formatting** (confirmed): row layout with the hidden id field, status
  glyphs, dimmed agent, and selection parsing round-trip.
- **list-agents --json** (confirmed): JSON shape/field-name contract for external
  callers, driven by stubbed window data.
- The `pickAgent` fzf shell-out is not unit-tested directly, consistent with how
  `pickSession` is left untested.

## Out of Scope

- Detecting a "waiting/blocked" status — deliberately collapsed into `idle`.
- Reliable OpenCode working-status detection (needs `kitty @ get-text`); accepted
  as always-idle for now.
- Any screen-scraping detection path.
- Migrating the nvim `find-agent.lua` to call `list-agents --json` (the JSON
  contract is provided so this can happen later, but the migration itself is not
  part of this work).
- A `--session` filter flag for the picker (can be added later if the list gets
  noisy).
- Sending text to an agent — that remains the nvim feature's responsibility.

## Further Notes

- Glossary terms live in `CONTEXT.md`: **agent window**, **agent status**,
  **goto-agent**, **list-agents**.
- The braille-spinner working signal and the OpenCode title gap were confirmed
  against live `kitty @ ls` output during design; Claude and Codex prefix the
  OSC title with a braille rune while working, OpenCode keeps a static title.
