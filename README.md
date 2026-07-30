# blf

<p align="center">
  <img src="docs/logo-web.png" alt="blf — blazingly fast" width="300">
</p>

Blazingly fast misc CLI utilities.

## Install

### Go

```bash
go install github.com/elentok/blf@latest
```

### Homebrew

```bash
brew tap elentok/stuff
brew install blf
```

## Shell Completions

### fish

```bash
blf completion fish > ~/.config/fish/completions/blf.fish
```

### bash

```bash
# Linux
blf completion bash > /etc/bash_completion.d/blf

# macOS
blf completion bash > $(brew --prefix)/etc/bash_completion.d/blf
```

### zsh

```bash
# Linux
blf completion zsh > "${fpath[1]}/_blf"

# macOS
blf completion zsh > $(brew --prefix)/share/zsh/site-functions/_blf
```

Start a new shell for completions to take effect.

## Commands

- `blf open <url>`: open a URL with the system default browser.
- `blf copy <text>`: copy text to the system clipboard. Use `blf copy -` to read the text from stdin (e.g. `echo hello | blf copy -`); trailing newlines are trimmed.
- `blf copy-ref <file>...`: copy one or more files to the clipboard as references (paste the actual files in GUI apps). Resolves relative and `~` paths, accepts directories, and validates all paths before copying (macOS via `osascript`, Linux/Wayland via `wl-copy`).
- `blf tmux links <open|copy>`: scan the current tmux pane for URLs and open a centered tmux menu.
- `blf tmux targets`: open an interactive popup to navigate and act on detected targets.
- `blf kitty list-os-windows`: print kitty OS windows and their tab titles, highlighting the active and last-focused rows.
- `blf kitty ls`: print a readable tree for `kitty @ ls`, including OS windows, tabs, windows, cmdlines, and foreground processes.
- `blf kitty goto-os-window [id]`: focus a kitty OS window directly by id, or pick one with `fzf`.
- `blf kitty targets`: open an interactive Kitty overlay to navigate and act on detected targets from the current window.
- `blf kitty list-agents [--json]`: list open AI agent windows (`claude`, `codex`, `opencode`, `cursor-agent`) across all OS windows and sessions, each with its working/waiting/idle status. Add `--json` for a machine-readable list (the source of truth for other tools).
- `blf kitty goto-agent`: pick an open AI agent window with `fzf` (showing each agent's status, directory, and title, with a live screen preview) and focus the selected window, pulling its tab and OS window forward.
- `blf kitty set-agent-state <working|waiting|idle> [--only-if-working]`: report the calling Kitty window's agent status by writing the `AGENT_STATE` user var. Meant to be called from an agent's event hooks; no-ops silently outside Kitty and prints nothing. `--only-if-working` writes only when the window is currently `working` (used by the `Notification` hook to ignore the ~60s idle nag).
- `blf kitty setup-claude [--dry-run]`: idempotently install the agent-state hooks into the global `~/.claude/settings.json` so Claude Code reports `working`/`waiting`/`idle`. `--dry-run` prints the diff without writing.
- `blf kitty new-session`: prompt for a session name, reuse an existing live session with the same name, otherwise create or recreate `~/.local/share/kitty/sessions/<name>.kitty-session` and switch to it.
- `blf kitty sessions`: list session files from `~/.local/share/kitty/sessions/`, preview their tab/window structure, and switch to the selected session.
- `blf kitty delete-session`: open a Kitty overlay, pick a session file, and delete it.
- `blf kitty doctor`: print Kitty session-debugging info including environment, session directory contents, and session match counts.
- `blf claude statusline [--silent] [--demo]`: render Claude status JSON from stdin as a compact status line. `--install` writes the command into `~/.claude/settings.json`'s `statusLine` entry.
- `blf claude history`: TUI for browsing Claude Code conversation history — list projects, drill into a project's conversations, grep across transcripts, export to markdown, and resume a session.
- `blf beads`: TUI for browsing and triaging Beads issues in the current project.
- `blf npm-scripts`: print `package.json` scripts in declaration order with aligned green names.
- `blf querystring <querystring|-> [key]` (alias: `blf qs`): parse and print query string params.
- `blf cal [date]`: print previous, current, and next month calendars with week numbers.
- `blf dim-path`: read file paths from stdin and dim the directory portion, leaving only the filename at full brightness. Designed for use with `fd | blf dim-path | fzf --ansi`. Respects `NO_COLOR`.
- `blf clean-url <url>` / `blf clean-url --clipboard`: unwrap redirect-wrapper URLs (e.g. Google search `/url?...&url=`) and strip tracking query params (`utm_*`, `gclid`, `fbclid`, etc.). Pass a URL to print the cleaned result, or use `--clipboard` to read, clean, and write the URL back to the clipboard.
- `blf sum [-e|--echo]`: sum the first space-delimited value from each stdin line.
- `blf version`: print the current `blf` version.
- `blf launcher`: Terminal launcher TUI (math, unit/currency conversion, app launch, scripts). Designed to run as a long-lived process inside Kitty's quick-access terminal; Cmd+2 toggles it into view.
- `blf launcher reindex`: rebuild the application index (`~/.cache/blf/apps.json`) manually. Run this on first use or after installing new apps.

`tmux links` behavior:

- Captures the last 10,000 lines from the current pane.
- Uses tmux `-J` capture mode to join soft-wrapped lines, so wrapped URLs are preserved.
- Extracts and deduplicates `http://` and `https://` URLs.
- Shows up to 30 URLs in a centered menu titled `Open URL` or `Copy URL`.
- On failure, posts a tmux status message via `tmux display-message`.

`tmux targets` behavior:

- Opens a popup at `80%` width/height and captures the visible viewport of the target pane.
- Popup title is `Select a target`.
- Condenses the viewport by folding target-free gaps to `...`, while keeping 1 line of context above and below each target (including top/bottom `...` when trimmed).
- Detects targets including URLs, AI agent resume commands (`codex resume <id>`, `opencode -s <id>`, `claude --resume <id>`, `agent --resume <id>`, `cursor-agent --resume <id>`), file refs (`path:line[:col]`), commit hashes, emails, host:port, UUIDs, issue refs, and branch/tag-like tokens.
- Schema-less URL matching requires a path (for example `github.com/elentok`), and bare domain-only strings are ignored.
- File detection requires a path separator (`/`), so `README.md` is ignored while `src/README.md` is detected.
- If a target text repeats, only the first occurrence is highlightable.
- Navigation: `j/k` or `up/down` move vertically only; `h/l` or `left/right` move horizontally only (no wrapping).
- Actions: `y` or `c` (copy + exit), `enter`/`o` (open URLs, or run AI resume commands in the active pane, then exit), `q` (exit).
- Search: `/` enters fuzzy search on target text, `enter` locks filtered mode, `esc` clears search.
- `?` opens an in-popup help page.
- Bottom bar shows key help and in-popup notifications/errors.
- In search/filtered mode, targets switch to green highlighting and a rounded search box appears in the popup.
- Non-openable `enter`/`o` shows an in-popup notification and keeps the popup open.

tmux binding example:

```tmux
bind-key t run 'blf tmux targets'
```

`kitty targets` behavior:

- Captures the visible viewport of the current Kitty window and detects the same targets as `tmux targets`.
- Runs directly inside the Kitty window or overlay where it was launched.
- When launched in an overlay, it reads and acts on the covered window via Kitty's `state:overlay_parent` match.
- Reuses the shared targets UI for navigation, search, copy, open, and resume-command actions.
- Sends AI resume commands back to the original Kitty window with `kitty @ send-text`.
- If no targets are found, prints an error and also attempts to show a Kitty error notification.

`kitty list-agents` behavior:

- Detects an agent window by whole command-word matching: the first token of the window's `last_reported_cmdline`, falling back to a foreground process's command word. A path that merely contains an agent's name (e.g. `/private/tmp/claude-501/…`) is never matched, and an agent launched behind a shell wrapper (e.g. `/bin/sh /usr/bin/command claude`) still is.
- Status is taken from the window's `AGENT_STATE` user var when present (`working`/`waiting`/`idle`, set by the agent via `set-agent-state`); this is authoritative. Otherwise it falls back to the window title: a leading braille-spinner rune means `working`, otherwise `idle` (the title can never yield `waiting`). OpenCode has no title status signal and, without the user var, always reads `idle`.
- Lists agents across every OS window and session, drops the currently-focused window, and sorts `waiting` first, then `working`, then by most recently focused.
- `--json` emits an array of `{ id, agent, status, dir, title, session }` objects; these field names are the stable contract for external callers.

`kitty goto-agent` behavior:

- Builds the same agent list as `list-agents` (waiting-first, dropping the currently-focused window) and presents it in a self-owned bubbletea TUI fuzzy picker.
- Type to fuzzy-filter by dir, title, or agent name; ↑/↓ (or ctrl-k/ctrl-j) to move; Enter to focus the selected agent; Esc to cancel; ? for help. Shows `No agent windows` inside the TUI when there are no agents open.
- Selecting an agent focuses its window with `kitten @ focus-window` (which pulls the window's tab and OS window forward).
- Runs directly in the current terminal; bind it to a Kitty mapping to launch it where you want (e.g. a new tab or overlay).

`kitty set-agent-state` / `setup-claude` behavior:

- `set-agent-state <working|waiting|idle>` validates the state, then runs `kitty @ set-user-vars AGENT_STATE=<state>` against the calling window (`KITTY_WINDOW_ID`). It no-ops silently when not run inside Kitty and prints nothing to stdout, so it is safe to wire into a Claude Code `UserPromptSubmit` hook (whose stdout is injected into the model's context). With `--only-if-working` it first reads the window's current `AGENT_STATE` (via `kitty @ ls --match id:<id>`) and writes only if it is `working`.
- `setup-claude` reconciles the canonical hook set into the global `~/.claude/settings.json`: `UserPromptSubmit`, `PreToolUse`, and `PostToolUse` → `working`, `Notification` → `waiting --only-if-working`, `Stop` → `idle`. `PostToolUse` is what clears `waiting` after you answer a question or permission prompt. `Notification` fires both for real input requests and as Claude Code's ~60s idle nag, so `--only-if-working` keeps the former (which only happens mid-task) and drops the latter (so a finished, idle agent is not flipped back to `waiting`). The reconcile is a narrow match on the `blf kitty set-agent-state` command, so it never touches unrelated hooks, and re-running it is a no-op. `--dry-run` prints the diff and writes nothing.

```conf
map kitty_mod+e>a launch --type=tab --cwd=current fish -c "blf kitty goto-agent"
```

`claude history` behavior:

- Opens on a fuzzy-filterable list of Claude Code projects (`~/.claude/projects`); type to filter, ↑/↓ to move, Enter to drill into a project's conversations, `ctrl+f` to jump straight to grep, Esc to quit.
- The conversations page lists each project's sessions by title (falling back to the session ID) with relative and absolute last-accessed times; Enter exports the conversation to markdown and opens it in `$EDITOR` (falling back to `nvim`/`vi`), `ctrl+r` resumes it with `claude --resume <session-id>`, `ctrl+y` copies the session ID to the clipboard, Esc goes back.
- `ctrl+f` opens transcript grep search (ripgrep-backed) with a live preview pane showing the matched conversation's title and session ID; `ctrl+g` toggles between project and global scope, Enter opens the matched conversation at that line, `ctrl+r` resumes its session, `ctrl+y` copies the session ID to the clipboard, Esc goes back.
- Filtering and search use the shared `fuzzyfinder` widget, including multi-word AND matching (e.g. `one two` matches rows containing both words in any order) and match highlighting.

`blf beads` behavior:

- Opens a project-contextual TUI over the local `.beads` database via the `bd` CLI; run it from a repo with Beads initialized, or pass `-C/--dir` to target another project.
- Lists issues flat (epics, subtasks, standalone) with client-side fuzzy filtering by id/title, readiness-first sorting, and a `↓N ↑M` badge for blockers/dependents.
- Shows a side preview for the selected issue with full description, an epic subtask tree, and the transitive blocked-by tree. Narrow terminals hide the preview by default; `tab` toggles it.
- Press `enter` to copy the selected issue id to the clipboard, print it to stdout, and quit.
- Press `?` for in-TUI key help. Main actions: `ctrl+a` create, `ctrl+s` status picker, `ctrl+x` close/reopen, `ctrl+e` edit in `$EDITOR`, `ctrl+g` open `bd graph`, `ctrl+r` refresh, `ctrl+f` cycle scope.

`kitty ls` behavior:

- Runs `kitty @ ls` and renders a readable tree grouped by OS window, tab, and window.
- Highlights active/last-focused state and includes per-window command line and foreground process when available.

`claude statusline` behavior:

- Reads JSON from stdin and renders model, tokens, context usage, and 5h/weekly usage in a single line.
- Context usage is shown as a progress bar with thresholds: `0-20%` green, `21-40%` orange, `41%+` red.
- Token counts over `1000` are compacted using `k` notation (`1234 -> 1.2k`).
- Missing/invalid fields render as `"<field> missing/invalid"` (or include raw invalid value), and malformed JSON prints an error and exits non-zero.
- `--silent` suppresses missing/invalid field segments.
- `--demo` ignores stdin and prints three sample lines (`10%`, `30%`, `60%`) for quick theme/style previews.
- `--install` idempotently writes `{"type": "command", "command": "blf claude statusline"}` into `~/.claude/settings.json`'s `statusLine` entry.

kitty binding example:

```conf
map kitty_mod+e>o launch --copy-env --type=overlay --cwd=current fish -c "blf kitty targets"
```

`kitty sessions` behavior:

- Uses `~/.local/share/kitty/sessions/` as the session-file directory.
- `new-session` runs directly in the current terminal; Kitty placement is controlled by your mapping. If a session with that name is still live it switches to it, otherwise it writes or rewrites the session file and switches to it.
- `sessions` runs directly in the current terminal; Kitty placement is controlled by your mapping. It uses `fzf` to pick from all session files, even if they currently have `0 tabs`.
- The picker no longer probes Kitty for tab counts; `fzf` preview renders the session file as a simple tab/window tree instead.
- `ctrl-d` inside the `sessions` picker deletes the selected session file and reloads the list in place.
- `ctrl-o` inside the `sessions` picker opens the selected session file in the editor from `$EDITOR`.
- `delete-session` opens the same session picker but deletes the selected session instead of switching to it.
- `new-session` still treats a same-name file with `0 tabs` as inactive and rewrites it before switching.

kitty session binding examples:

```conf
tab_bar_filter session:~ or session:^$
map kitty_mod+e>n launch --location=hsplit --bias=10 --cwd=current fish -c "blf kitty new-session"
map kitty_mod+e>j launch --location=before --bias=25 --cwd=current fish -c "blf kitty sessions"
```

These mappings are only examples. Because `blf kitty new-session` and `blf kitty sessions` now run directly, you can choose the presentation entirely in `kitty.conf`:

- use `--location=before --bias=25` for a left sidebar
- use `--location=hsplit --bias=10` for a small prompt below the current window
- use any other Kitty `launch` placement that fits your workflow

`blf launcher` behavior:

- Type a **math expression** (`1234*2`, `sqrt(2)*pi`, `200+10%`) → result appears immediately; Enter copies it to the clipboard.
- Type a **`<number><unit>`** (`10cm`, `123$`) → conversions to every other unit in that group appear; Enter copies the selected row.
- Type a **name** → fuzzy matches against installed applications, configured scripts, macOS System Settings panes, and directories (Home, Desktop, Downloads, Documents, iCloud, plus any configured), ranked into one list with match-position highlighting; Enter launches the app, runs the script, or opens the directory in the file manager.
- Computational input suppresses the fuzzy app/script list; name-like input shows it. A bare small number (`1`) searches apps; a large bare number (`1000000`) also shows a comma-formatted copy row.
- Empty input shows recent history items; Up/Down selects, Enter populates the input and recomputes without re-firing.
- **↑/↓, Ctrl+K/J, Ctrl+P/N** — navigate the result list.
- **Ctrl+R / Ctrl+F** — navigate backward/forward through history, populating the input each step.
- **Ctrl+S** — save the current input to history without acting; a transient "saved" confirmation appears for 1.5 s.
- **Ctrl+Shift+R** — rebuild the app index in the background; a brief loading indicator appears.
- **Esc** — clear input, reset to the empty state, and hide the quick terminal.
- **`?`** — toggle the key-binding help footer.
- After a successful Enter action the launcher resets and hides the quick terminal; it never exits.

Quick-terminal setup (Kitty):

1. Start the launcher manually the first time — open any Kitty terminal and run `blf launcher`.
2. Bind a key in `kitty.conf` to toggle the quick terminal into and out of view:

```conf
map cmd+2 kitten quick-access-terminal --instance-group quick
```

3. Press Cmd+2 to open the quick terminal; `blf launcher` is already running and responsive.

For a system-wide hotkey outside Kitty (using [skhd](https://github.com/koekeishiya/skhd)):

```sh
# ~/.skhdrc — requires allow_remote_control yes in kitty.conf
cmd - 2 : kitty @ kitten quick-access-terminal --instance-group quick
```

Quick-terminal setup (Ghostty, macOS):

1. Add a quick-terminal keybinding in `~/.config/ghostty/config`:

```conf
keybind = global:cmd+2=toggle_quick_terminal
```

2. Press Cmd+2, then run `blf launcher` once in the Ghostty quick terminal.
3. The launcher stays running between appearances. Esc and successful actions reset it and hide the quick terminal via Ghostty's AppleScript API; macOS may request Automation permission the first time.

Config (`~/.config/blf/config.toml`). Run `blf config edit` to open it in `$EDITOR` (creating it with
the encoded defaults first, if missing). This also (re)writes `~/.config/blf/config.schema.json` and
`~/.config/blf/taplo.toml`, and ensures the file starts with a `#:schema ./config.schema.json` line —
adding it if an existing `config.toml` predates this feature — so
[taplo](https://taplo.tamasfe.dev/) (and editors built on it, e.g. the Even Better TOML VS Code
extension) validate the file and offer completion:

```toml
[launcher]
script_weight    = 2.0  # scripts rank above apps (default 1.5)
app_weight       = 1.0  # default
directory_weight = 1.0  # default
snippets_weight  = 1.0  # default

# Built-in scripts include playpause and clean-url.
# [[launcher.script]] entries add to or override them.
[[launcher.script]]
name     = "Spotify: play/pause"
type     = "osascript"
platform = "mac"
body     = "tell application \"Spotify\" to playpause"
output   = "ignore"

[[launcher.script]]
name   = "clean clipboard URL"
type   = "bash"
body   = "blf clean-url --clipboard"
output = "ignore"

# [[launcher.unit_group]] entries add custom unit groups.
# Factor is relative to the group's base unit (first unit, factor = 1.0).
[[launcher.unit_group]]
name = "pressure"

[[launcher.unit_group.unit]]
name    = "pascal"
symbols = ["pa"]
factor  = 1.0

[[launcher.unit_group.unit]]
name    = "kilopascal"
symbols = ["kpa"]
factor  = 1000.0

[[launcher.unit_group.unit]]
name    = "bar"
symbols = ["bar"]
factor  = 100000.0

[[launcher.unit_group.unit]]
name    = "psi"
symbols = ["psi"]
factor  = 6894.76

[[launcher.unit_group.unit]]
name    = "atmosphere"
symbols = ["atm"]
factor  = 101325.0

# Built-in directories: Home, Desktop, Downloads, Documents, iCloud.
# [[launcher.directory]] entries add to or override them (matched by name);
# "~" is expanded, and entries whose path doesn't exist are hidden.
[[launcher.directory]]
name = "Projects"
path = "~/dev"

[[launcher.directory]]
name = "Desktop"       # overrides the built-in Desktop's path
path = "~/OtherDesktop"
```

Snippets (`~/.config/blf/snippets.toml`) — selecting one copies its value to the clipboard. Run
`blf config edit-snippets` to open the file in `$EDITOR` (creating it with a commented-out example
first, if missing). Like `config edit`, this (re)writes `~/.config/blf/snippets.schema.json` and
`~/.config/blf/taplo.toml`, and ensures the file starts with a `#:schema ./snippets.schema.json`
line:

```toml
[[snippet]]
name  = "shipping"
value = """
David Elentok
123 Main St
Springfield, IL 62704
"""

[[snippet]]
name  = "billing"
icon  = "󰆔"
value = "456 Other Ave"
```

Data paths:

- Config: `~/.config/blf/config.toml` (respects `$XDG_CONFIG_HOME`)
- Config schema: `~/.config/blf/config.schema.json` (regenerated on every `blf config edit`,
  respects `$XDG_CONFIG_HOME`)
- Snippets: `~/.config/blf/snippets.toml` (respects `$XDG_CONFIG_HOME`)
- Snippets schema: `~/.config/blf/snippets.schema.json` (regenerated on every `blf config
  edit-snippets`, respects `$XDG_CONFIG_HOME`)
- taplo config: `~/.config/blf/taplo.toml` (regenerated on every `blf config edit` or `edit-snippets`,
  respects `$XDG_CONFIG_HOME`)
- App index cache: `~/.cache/blf/apps.json` (respects `$XDG_CACHE_HOME`)
- Currency rate cache: `~/.cache/blf/currency.json`
- History: `~/.local/state/blf/launcher-history` (respects `$XDG_STATE_HOME`)
