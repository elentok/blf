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
- `blf tmux-targets`: open an interactive popup to navigate and act on detected targets.
- `blf kitty list-os-windows`: print kitty OS windows and their tab titles, highlighting the active and last-focused rows.
- `blf kitty goto-os-window [id]`: focus a kitty OS window directly by id, or pick one with `fzf`.
- `blf kitty targets`: open an interactive Kitty overlay to navigate and act on detected targets from the current window.
- `blf kitty new-session`: prompt for a session name in a Kitty overlay, reuse an existing live session with the same name, otherwise create or recreate `~/.local/share/kitty/sessions/<name>.kitty-session` and switch to it.
- `blf kitty sessions`: open a Kitty overlay, list session files from `~/.local/share/kitty/sessions/`, preview their tab/window structure, and switch to the selected session.
- `blf kitty doctor`: print Kitty session-debugging info including environment, session directory contents, and session match counts.
- `blf npm-scripts`: print `package.json` scripts in declaration order with aligned green names.
- `blf querystring <querystring|-> [key]` (alias: `blf qs`): parse and print query string params.
- `blf cal [date]`: print previous, current, and next month calendars with week numbers.
- `blf sum [-e|--echo]`: sum the first space-delimited value from each stdin line.
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
bind-key t run 'blf tmux-targets'
```

`kitty targets` behavior:

- Captures the visible viewport of the current Kitty window and detects the same targets as `tmux-targets`.
- Opens a Kitty overlay only when at least one target is found.
- Reuses the shared targets UI for navigation, search, copy, open, and resume-command actions.
- Sends AI resume commands back to the original Kitty window with `kitty @ send-text`.
- If no targets are found, prints an error and also attempts to show a Kitty error notification.

kitty binding example:

```conf
map kitty_mod+e>o launch --copy-env --type=background --cwd=current fish -c "blf kitty targets"
```

`kitty sessions` behavior:

- Uses `~/.local/share/kitty/sessions/` as the session-file directory.
- `new-session` opens an overlay prompt for the session name; if a session with that name is still live it switches to it, otherwise it writes or rewrites the session file and switches to it.
- `sessions` opens an overlay and uses `fzf` to pick from all session files, even if they currently have `0 tabs`.
- The picker no longer probes Kitty for tab counts; `fzf` preview renders the session file as a simple tab/window tree instead.
- `new-session` still treats a same-name file with `0 tabs` as inactive and rewrites it before switching.

kitty session binding examples:

```conf
tab_bar_filter session:~ or session:^$
map kitty_mod+e>n launch --copy-env --type=background --cwd=current fish -c "blf kitty new-session"
map kitty_mod+e>j launch --copy-env --type=background --cwd=current fish -c "blf kitty sessions"
```
