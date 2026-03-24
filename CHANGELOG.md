# Changelog

All notable changes to this project are documented in this file.

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
