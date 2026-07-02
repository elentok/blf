# Changelog

All notable changes to this project are documented in this file.

## [v0.6.2] - 2026-07-02

- Added **learned ranking** to `launcher`: previously selected results for a given query are boosted above the usual fuzzy/history ranking on subsequent searches. Selections are tracked per (query, result) pair in a new persistent store (XDG state dir), and only non-first picks are recorded, so the effect only kicks in once a result has been chosen over the top suggestion.

## [v0.6.1] - 2026-07-02

- Added a **directory source** to `launcher`: fuzzy-matches filesystem directories (Home, Desktop, Downloads, Documents, iCloud by default) and opens the selected one in the file manager. Configurable via `[[launcher.directory]]` (`name`, `path`, `~` expanded) and `directory_weight`; user entries add to or override the built-ins by name, and entries whose path doesn't exist are hidden.
- Fixed `launcher`'s "open" action (`ActionOpen`, used by System Settings panes and now directories) to use the same cross-platform opener as `blf open` instead of a hardcoded macOS-only `open` call.
- `clean-url` now also recognizes Google's `q` redirect param (in addition to `url`), so `google.com/url?q=<dest>` links unwrap correctly.

## [v0.6.0] - 2026-06-30

- Added **`blf claude history`**: a TUI for browsing Claude Code conversation history. Lists projects, then conversations within a project (title, relative/absolute last-accessed time), with markdown export, a resume action that reopens a conversation's Claude session directly, and a `ctrl+y` binding to copy its session ID to the clipboard.
- Added transcript search to `claude history` (`ctrl+f`): greps across conversations with ripgrep, toggling between project and global scope, with a live preview pane (also showing the matched conversation's session ID), its own resume action, and a `ctrl+y` binding to copy the session ID to the clipboard.
- **`fuzzyfinder`** (the shared picker behind `launcher`, `goto-agent`, and `claude history`) now supports multi-word AND matching, so a query like `one two` matches rows containing both words in any order.
- Fixed `fuzzyfinder` list rows wrapping onto a second line when their content was wider than the available width (e.g. long conversation titles in `claude history`), which pushed the footer off the bottom of the frame; rows are now truncated with `…` to fit.
- **`launcher`** app scanning is now recursive and shows each app's parent folder as a subtitle, so apps nested in subfolders are found and easier to disambiguate.
- **`launcher` history hints**: math and currency entries in the history list now show their computed result as a dimmed-italic subtitle (`10+20 = 30`, `1$ = 2.987 ₪, 0.79 £, 0.92 €`). Currency lines use each currency's symbol where one exists (now including `₪` for ILS) and its ISO code otherwise, computed live from the current rates each time the list is shown. Non-currency unit conversions and bare numbers get no hint.
- The `₪` shekel symbol is now recognized as a currency unit, so `100₪` parses as a currency query.
- Currency amounts now drop trailing zeros (`2.987 ₪` instead of `2.9870 ₪`), keeping at least two decimals for amounts ≥ 1.
- Capped `launcher` history at 30 entries (was 500); existing histories are trimmed to the newest 30 on next load.
- Added a project logo to the README.

## [v0.5.3] - 2026-06-30

- Fixed `goto-agent` jumping to an agent in a different Kitty session: blf now switches to the target agent's session before focusing its window. Previously, focusing a cross-session window grafted its tab onto the active session, where it vanished on the next tab change.

## [v0.5.2] - 2026-06-29

- **`goto-agent` picker** now highlights the fuzzy-matched characters as you type, across all searchable fields (dir, title, and agent name) — previously matches weren't highlighted at all.
- Fixed active-row alignment in the `goto-agent` picker: the working spinner is padded to match the wider idle/waiting glyphs, and each status glyph now carries its trailing space so text no longer shifts between rows or sits flush against the directory.
- Extracted a shared `Highlight` primitive in `fuzzyfinder` so the launcher and `goto-agent` picker render match highlighting and the selection background identically.

## [v0.5.1] - 2026-06-29

- **`goto-agent` picker**: replaced the `fzf`-based picker with a self-owned bubbletea v2 TUI. The new picker renders inline (no subprocess), fuzzy-filters agents by dir/title/agent name as you type, shows a live split-pane preview of the selected agent's screen on the right, and auto-refreshes the agent list every second with a braille spinner for working agents. Selection is ID-stable across refreshes (cursor follows the same agent). The hidden `__preview-agent` subcommand has been removed.
- **`launcher` keybindings** updated to match fzf/goto-agent conventions:
  - **Ctrl+P / Ctrl+N** — now navigate the result list (↑/↓ aliases), consistent with goto-agent. Ctrl+K / Ctrl+J remain as additional aliases.
  - **Ctrl+R / Ctrl+F** — navigate history backward/forward (was Ctrl+P / Ctrl+N).
  - **Ctrl+Shift+R** — reindex apps (was Ctrl+R).

## [v0.5.0] - 2026-06-29

- Added a **`waiting`** agent status to `blf kitty list-agents` / `goto-agent`, alongside the existing `working` and `idle`. An agent reports its own state by pushing a Kitty per-window user var `AGENT_STATE` (`working`/`waiting`/`idle`), which blf reads from the same free `kitty @ ls` payload. When the var is present it is authoritative; otherwise blf falls back to the OSC-title spinner heuristic (which can only distinguish `working` from `idle`). The listing now sorts `waiting` first, then `working`, then `idle`.
- Added `blf kitty set-agent-state <working|waiting|idle> [--only-if-working]`: writes `AGENT_STATE` on the calling Kitty window (via `KITTY_WINDOW_ID`). No-ops silently outside Kitty and prints nothing to stdout (so it is safe to call from a Claude Code `UserPromptSubmit` hook, whose stdout is injected into the model's context). `--only-if-working` applies the write only when the window is currently `working`, so the `Notification` hook can ignore Claude Code's ~60s idle nag (which would otherwise flip a finished, idle agent back to `waiting`).
- Added `blf kitty setup-claude [--dry-run]`: idempotently installs the agent-state hooks into the global `~/.claude/settings.json` (`UserPromptSubmit`/`PreToolUse`/`PostToolUse` → working, `Notification` → `waiting --only-if-working`, `Stop` → idle). Reconciliation is a narrow match on the `blf kitty set-agent-state` command, so unrelated hooks (including other `blf kitty …` hooks) are never touched. `--dry-run` prints the resulting diff and writes nothing.
- Cleaned up the agent listing UI: rows now render as `<status> <dir>: <title> (<agent>)` with the title highlighted in blue, the `✳` prefix stripped from Claude titles, and a quiet (glyph-less) working state.

## [v0.4.18] - 2026-06-27

- Added `blf launcher`: a terminal launcher TUI designed to run as a long-lived process inside Kitty's quick-access terminal (toggle with Cmd+2). Provides:
  - **Math**: evaluates expressions (`1234*2`, `sqrt(2)*pi`, `200+10%`) with comma-formatted output; Enter copies the result.
  - **Unit conversion**: parses `<number><unit>` (`10cm`, `123$`) and shows conversions to every other unit in the group; Enter copies the selected row. Built-in groups: length, mass, temperature, speed, data, time, area, volume, plus currency.
  - **Currency**: live rates from open.er-api.com (fawazahmed0 fallback), cached at `~/.cache/blf/currency.json` with a ~12h TTL and stale-cache fallback on network failure.
  - **App launcher**: fuzzy search over an indexed list of installed applications with match highlighting; Enter launches the selected app.
  - **Scripts**: named bash/osascript actions defined in config; built-ins include `playpause` and `clean-url`; user scripts add to or override them.
  - **History**: every Enter action and Ctrl+S save is persisted to `~/.local/state/blf/launcher-history` (deduped, capped at 500). Ctrl+P/Ctrl+N navigate history; empty input shows recent entries.
  - Config at `~/.config/blf/config.toml` (`[launcher]` section); custom `unit_group` and `script` entries supported.
- Added `blf launcher reindex` to build or refresh the application index (`~/.cache/blf/apps.json`).

## [v0.4.17] - 2026-06-24

- Added `blf kitty list-agents [--json]` to list open AI agent windows (`claude`, `codex`, `opencode`, `cursor-agent`) across all OS windows and sessions with their working/idle status. Detection matches whole command words (so a path like `/private/tmp/claude-501/…` is never a false positive, while an agent behind a shell wrapper is still found); status comes from the window title's braille spinner. `--json` emits a stable `{ id, agent, status, dir, title, session }` contract for other tools.
- Added `blf kitty goto-agent` to pick an open AI agent window with `fzf` (status, directory, and title, with a live screen preview) and focus it, pulling its tab and OS window forward.

## [v0.4.16] - 2026-06-14

- `blf copy -` reads the text to copy from stdin (e.g. `echo hello | blf copy -`), trimming trailing newlines and erroring on empty input.

## [v0.4.15] - 2026-06-07

- Added `blf clean-url` to unwrap redirect-wrapper URLs (e.g. Google search `/url?...&url=`) and strip tracking query params (`utm_*`, `gclid`, `fbclid`, etc.). Run it as `blf clean-url <url>` to print the cleaned URL, or `blf clean-url --clipboard` to clean the URL on the clipboard in place.
- Fix `blf dim-path` to support `path/to/file/` (only dim `path/to/`)

## [v0.4.14] - 2026-06-06

- Added `blf dim-path` to dim the directory portion of file paths from stdin, for use with `fd | blf dim-path | fzf --ansi`.

## [v0.4.13] - 2026-06-06

- `blf claude-statusline`: token usage coloring is now based on absolute token count instead of context percentage. Green (🙂) below 75k, orange (🤔) from 75k–100k, red (🥵) at 100k+. The color is applied to the progress bar, percentage text, token count, and icon.

## [v0.4.12] - 2026-06-06

- Upgrade goreleaser
- Run go mod tidy to fix release issue

## [v0.4.11] - 2026-06-05 - failed to release

- Shell completions are now available via `blf completion fish|bash|zsh` (powered by Cobra).
- All commands and subcommands now support `--help` for usage and flag descriptions.
- Added `--version` / `-v` flag to the root command (equivalent to `blf version`).
- `blf claude-statusline`: changed the tokens icon.

## [v0.4.10] - 2026-06-01

- `blf copy-ref` on Linux no longer hangs. `wl-copy` forks a daemon child that inherits stderr and keeps it open until the next clipboard write, which stalled blf because the command runner captured stderr through a pipe and waited for it to close. The clipboard command now passes stderr straight through, so it returns as soon as the copy is handed off.

## [v0.4.9] - 2026-05-31

- `blf copy-ref` on macOS now reliably copies multiple files. It previously landed only a random subset on the clipboard (for example 2 of 3), because the pasteboard's lazy file-reference providers raced with the short-lived CLI process exiting; the references are now materialized before the process exits.

## [v0.4.8] - 2026-05-31

- Added `blf copy-ref <file>...` to copy one or more files to the clipboard as file references, so pasting into a GUI app drops/attaches the actual files. Resolves relative and `~` paths to absolute, accepts directories, validates every path up front (all-or-nothing), and confirms the count on success. Uses `osascript` on macOS and `wl-copy` (Wayland) on Linux.

## [v0.4.7] - 2026-05-18

- `blf claude-statusline` now hides `5h` and `weekly` values when they are missing or invalid (for example on Bedrock payloads), instead of rendering `missing/invalid` placeholders.

## [v0.4.6] - 2026-05-17

- Added `blf kitty ls` to render a readable tree from `kitty @ ls`, including OS windows, tabs, windows, session names, command lines, and foreground processes.
- Added `blf claude-statusline` to render Claude status JSON as a compact status line with model, token count, context usage, and 5h/weekly rate-limit usage.

## [v0.4.5] - 2026-04-25

- `blf kitty targets` now runs directly in the launched Kitty overlay instead of spawning its own nested overlay, and it targets the covered window via Kitty's `state:overlay_parent` match.
- `blf kitty targets` now lives in `internal/kitty`, so all `blf kitty ...` subcommands share one internal package and dependency model.

## [v0.4.4] - 2026-04-21

- `blf kitty sessions` now reads Kitty nightly session membership and `last_focused_at` data from `kitty @ ls`, hides the active session, sorts live sessions ahead of empty ones by recency, and dims empty sessions in the picker.
- `blf kitty doctor` now shows the same nightly-aware session data, including whether each session is live, empty, or active and its latest `last_focused_at` value.

## [v0.4.3] - 2026-04-21

- `blf kitty sessions` and `blf kitty new-session` now run directly in the current terminal, so their placement is controlled entirely by the Kitty `launch` mapping in `kitty.conf`.
- The `blf kitty sessions` preview is more compact: it no longer shows the session path, uses `Live session:` / `Empty session:` headers, and keeps the tab list closer to the header.

## [v0.4.2] - 2026-04-17

### Changed

- `blf kitty sessions` now maps `ctrl-o` to open the selected session file in the editor from `$EDITOR`.

## [v0.4.1] - 2026-04-17

### Changed

- `blf kitty new-session` no longer writes the session name into `new_tab`, so new Kitty sessions do not get a hardcoded tab title.

## [v0.4.0] - 2026-04-17

Add kitty session manager:

- Added `blf kitty new-session` to create or reuse named Kitty session files and switch to them from an overlay.
- Added `blf kitty sessions` to browse Kitty session files in an overlay with `fzf`.
  - previews sessions with live-tab awareness
  - supports `ctrl-d` to delete inactive session files and reload the picker in place (only sessions with no live tabs).
  - preview moves below the list automatically when the picker is narrower than `70` columns.
- Added `blf kitty doctor` to print Kitty session diagnostics including environment, session files, and session match counts.
- Added `blf kitty delete-session` to remove inactive Kitty session files from an overlay picker.
  - only remove sessions with no live tabs.

## [v0.3.0] - 2026-04-16

- Added `blf kitty targets` to open an interactive Kitty overlay for navigating
  and acting on detected targets from the current window.

- Target detection:
  - Recognizes env-var-prefixed file paths such as `$HOME/path/file.txt`.
  - "Cleans" the viewport by replaceing prompt glyphs such as `` with spaces.
  - Extracted the shared code between kitty and tmux targets detection.

## [v0.2.5] - 2026-04-16

- Added `blf sum` to sum the first space-delimited token from each stdin line, with optional input echoing.
- Added `blf kitty list-os-windows` and `blf kitty goto-os-window [id]` for listing and focusing kitty OS windows.

## [v0.2.4] - 2026-04-08

### Added

- Added `blf querystring` and `blf qs` to parse query string params from a raw query string, full URL, or stdin via `-`.
- Added `blf cal` to print previous, current, and next month calendars with week numbers.

## [v0.2.3] - 2026-04-04

### Added

- Added `blf npm-scripts` to print `package.json` scripts in declaration order with aligned green names.

## [v0.2.2] - 2026-04-01

### Added

- Added `tmux-targets` detection for AI resume commands:
  - `codex resume <id>`
  - `opencode -s <id>`
  - `claude --resume <id>`
  - `agent --resume <id>`
  - `cursor-agent --resume <id>`

### Changed

- `tmux-targets` now runs the selected AI resume command in the active tmux pane when pressing `enter` or `o`.
- URL targets keep their existing open behavior, while other non-openable targets still show the in-popup notification.

## [v0.2.1] - 2026-03-26

### Changed

- Tightened `tmux-targets` URL/path detection to avoid false positives:
  - Bare domain matching now requires a path segment (for example `github.com/elentok`).
  - File matching now requires a path separator (`/`), so bare filenames are ignored while path forms (for example `src/README.md`) are still detected.
- Condensed viewport rendering now adds `...` at the top and/or bottom when trimmed lines exist there.

## [v0.2.0] - 2026-03-26

### Added

- Added condensed `tmux-targets` viewport rendering that folds non-target gaps to `...` while preserving one line of context above and below each target.
- Added in-popup bottom help/status bar for `tmux-targets`.
- Added `?` mapping in `tmux-targets` to open an in-popup help page.

### Changed

- `tmux-targets` now opens as a centered `80%` x `80%` popup titled `Select a target`.
- `tmux-targets` directional navigation is now axis-constrained and non-wrapping (`j/k` vertical only, `h/l` horizontal only).
- `tmux-targets` notifications/errors now render in the popup bottom bar instead of using tmux status messages.
- Updated `tmux-targets` styling to use Catppuccin palette variables, including a darker bottom-bar background.
- Normalized specific powerline glyphs (``, ``, ``, ``) to spaces before rendering for cleaner target parsing/display.

## [v0.1.1] - 2026-03-25

- Fix search colors issue

## [v0.1.0] - 2026-03-25

### Added

- Added `blf tmux-targets` to open a same-size tmux popup over the current pane and navigate detected targets.
- Added target detection for URLs, domains, file paths (`path:line[:col]`), commit hashes, emails, host:port, UUIDs, issue refs, and branch/tag-like tokens.
- Added fuzzy search mode in `tmux-targets` (`/` to search, `enter` to lock filter, `esc` to clear), with in-popup search box and filtered navigation.
- Added `blf version` (`version`, `-v`, `--version`) with build-info fallback and ldflags override support.

### Changed

- Extracted shared tmux status messaging into `internal/tmuxutil` and reused it across `tmux-links` and `tmux-targets`.

## [v0.0.2] - 2026-03-24

### Changed

- The "no links found" case now shows a tmux message but exits successfully.

## [v0.0.1] - 2026-03-24

### Added

- Added `blf open <url>` to open URLs with the system default browser.
- Added `blf copy <text>` to copy text to the system clipboard.
- Added `blf tmux-links <open|copy>` to capture pane history, extract URLs, and show a centered tmux menu.

### Changed

- `tmux-links` now captures pane history with `tmux capture-pane -pJ -S -10000` so soft-wrapped URLs are reconstructed.
- `tmux-links` menu is capped at 30 entries and uses mode-specific titles: `Open URL` / `Copy URL`.
- `tmux-links` failures are surfaced with `tmux display-message -d 5000` for tmux key-binding workflows.
