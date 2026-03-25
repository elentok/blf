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

`tmux-links` behavior:

- Captures the last 10,000 lines from the current pane.
- Uses tmux `-J` capture mode to join soft-wrapped lines, so wrapped URLs are preserved.
- Extracts and deduplicates `http://` and `https://` URLs.
- Shows up to 30 URLs in a centered menu titled `Open URL` or `Copy URL`.
- On failure, posts a tmux status message via `tmux display-message`.

`tmux-targets` behavior:

- Captures the visible viewport of the current pane and opens a popup with matching width/height.
- Detects targets including URLs, file refs (`path:line[:col]`), commit hashes, emails, host:port, UUIDs, issue refs, and branch/tag-like tokens.
- Navigation: `j/k`, arrows, `h/l`, `gg`, `G`.
- Actions: `y` (copy + exit), `enter`/`o` (open if openable + exit), `q` (exit).
- Non-openable `enter`/`o` shows `tmux display-message -d 5000` and keeps the popup open.
