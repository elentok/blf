# Changelog

All notable changes to this project are documented in this file.

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
