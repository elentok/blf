# Changelog

All notable changes to this project are documented in this file.

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
