# blf

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

## Commands

- `blf open <url>`: open a URL with the system default browser.
- `blf copy <text>`: copy text to the system clipboard.
- `blf tmux-links <open|copy>`: scan the current tmux pane for URLs and open a centered tmux menu.
- `blf tmux-targets`: open a same-size tmux popup that highlights one detected target at a time.
- `blf version`: print the current `blf` version.

`tmux-links` behavior:

- Captures the last 10,000 lines from the current pane.
- Uses tmux `-J` capture mode to join soft-wrapped lines, so wrapped URLs are preserved.
- Extracts and deduplicates `http://` and `https://` URLs.
- Shows up to 30 URLs in a centered menu titled `Open URL` or `Copy URL`.
- On failure, posts a tmux status message via `tmux display-message`.

`tmux-targets` behavior:

- Opens a popup at `80%` width/height and captures the visible viewport of the target pane.
- Popup title is `Select a target`.
- Condenses the viewport by folding target-free gaps to `...`, while keeping 1 line of context above and below each target.
- Detects targets including URLs, file refs (`path:line[:col]`), commit hashes, emails, host:port, UUIDs, issue refs, and branch/tag-like tokens.
- If a target text repeats, only the first occurrence is highlightable.
- Navigation: `j/k`, arrows, `h/l`, `gg`, `G`.
- Actions: `y` or `c` (copy + exit), `enter`/`o` (open if openable + exit), `q` (exit).
- Search: `/` enters fuzzy search on target text, `enter` locks filtered mode, `esc` clears search.
- `?` opens an in-popup help page.
- Bottom bar shows key help and in-popup notifications/errors.
- In search/filtered mode, targets switch to green highlighting and a rounded search box appears in the popup.
- Non-openable `enter`/`o` shows an in-popup notification and keeps the popup open.

tmux binding example:

```tmux
bind-key t run 'blf tmux-targets'
```
