# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

- Added `blf kitty list-agents [--json]` to list open AI agent windows (`claude`, `codex`, `opencode`, `cursor-agent`) across all OS windows and sessions with their working/idle status. Detection matches whole command words (so a path like `/private/tmp/claude-501/…` is never a false positive, while an agent behind a shell wrapper is still found); status comes from the window title's braille spinner. `--json` emits a stable `{ id, agent, status, dir, title, session }` contract for other tools.

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
