# blf

Blazingly fast misc CLI utilities.

## Commands

- `blf open <url>`: open a URL with the system default browser.
- `blf copy <text>`: copy text to the system clipboard.
- `blf tmux-links <open|copy>`: scan the current tmux pane for URLs and open a centered tmux menu.

`tmux-links` behavior:

- Captures the last 10,000 lines from the current pane.
- Uses tmux `-J` capture mode to join soft-wrapped lines, so wrapped URLs are preserved.
- Extracts and deduplicates `http://` and `https://` URLs.
- Shows up to 30 URLs in a centered menu titled `Open URL` or `Copy URL`.
