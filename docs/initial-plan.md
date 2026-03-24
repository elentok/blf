# blf Initial Implementation Plan

## Goal

Create a Go CLI named `blf` with these initial commands:

- `blf tmux-links open`
- `blf tmux-links copy`
- `blf open <url>`
- `blf copy <text>`

Behavior:

1. Capture current tmux pane (`tmux capture-pane -pJ -S "-10000"`)
2. Extract URLs
3. Show them in `tmux display-menu`
4. On selection, either open the URL or copy it to clipboard

## Proposed Project Shape

Use a `gx`-style command dispatcher:

- `main.go` - thin entrypoint
- `cmd/cmd.go` - command parsing and usage text
- `internal/tmuxlinks/` - command logic
- `internal/platform/` - open/copy helpers

This keeps command wiring separate from business logic and makes unit testing easier.

## Cross-Platform Strategy (open + copy)

### Copy to clipboard

Use `github.com/atotto/clipboard`.

Why:

- Already used in `colr`
- Cross-platform support is sufficient for this use case
- Simple API (`WriteAll`)

### Open URL

Use `github.com/pkg/browser` as the default URL opener.

Why:

- Cross-platform wrapper around OS-specific open commands
- Cleaner than manually branching on `runtime.GOOS`

Fallback plan (if needed): use explicit commands (`open`, `xdg-open`, `rundll32`).

## Command Design

Public commands:

- `blf tmux-links <open|copy>`
- `blf open <url>`
- `blf copy <text>`

`tmux-links` menu callbacks should invoke these real commands directly:

- `blf open '<url>'`
- `blf copy '<url>'`

Keep command construction shell-safe by wrapping arguments with single-quote escaping when building tmux callback strings.

## Execution Flow

1. Validate command + args (`tmux-links <mode>`, `open <url>`, `copy <text>`)
2. For `blf open`/`blf copy`, execute action immediately and exit
3. For `tmux-links`, verify `tmux` exists and we are in a tmux client context
4. Run `tmux capture-pane -pJ -S "-10000"`
5. Extract URLs from pane text
6. Normalize and dedupe URLs while preserving order
7. Build `tmux display-menu` items (cap to first 30 for usability)
8. Each menu item callback invokes `blf open ...` or `blf copy ...`
9. Return non-zero on hard failures with actionable stderr messages

Notes for implementation:

- Add an inline code comment at the `capture-pane` call explaining the flags:
  - `-p`: print pane contents to stdout
  - `-J`: join soft-wrapped lines (fixes mid-URL terminal wrapping)
  - `-S -10000`: start capture 10,000 lines back for enough history
- This ensures future readers understand both what the args do and why they are required.

## URL Extraction Plan

Implement a small extractor package/function:

- Regex for `http://` and `https://`
- Trim common trailing punctuation (`)`, `]`, `}`, `.`, `,`, `;`, `:`)
- Optional basic validation via `net/url`
- Dedupe with `map[string]struct{}` + ordered output slice
- Intentionally ignore non-http schemes (`mailto:`, `file:`, etc.)

## tmux Menu Plan

Build argv directly (no shell pipeline), for example:

- `tmux display-menu -T "Open URL" ...`

Menu details:

- Cap to max 30 items
- Keep menu centered
- Dynamic title: `Open URL` for open mode, `Copy URL` for copy mode
- Show shortened labels (truncate long URLs for display)
- Keep full URL in callback command argument
- Include a disabled header or info row when possible
- If no URLs found, print a clear message to stderr and exit 1

## Error Handling and UX

- Friendly errors for:
  - not running inside tmux
  - tmux binary missing
  - pane capture failed
  - no URLs found
- open action failed
- copy action failed
- Keep stdout quiet on success (script-friendly)

## Testing Plan

### Unit tests

- URL extraction from realistic multiline input
- URL extraction from tmux soft-wrapped content joined via `-J` behavior assumptions
- Trailing punctuation trimming
- Deduplication order
- Shell-escaping helper for callback command arguments
- Menu label truncation

### Command tests

- `cmd.Execute` routing for `tmux-links open|copy`, `open`, and `copy`
- Invalid argument handling and usage text

### Integration-style tests (mocked exec)

- Inject command runner interface to fake `tmux` calls
- Assert generated `display-menu` arguments contain expected callbacks

## Implementation Steps

- [x] Initialize Go module and base CLI skeleton (`main.go`, `cmd/cmd.go`)
- [x] Add command parsing for `tmux-links`, `open`, and `copy`
- [x] Implement `open`/`copy` actions as reusable command handlers
- [x] Implement pane capture + URL extraction + dedupe
- [x] Implement `tmux display-menu` builder and invocation
- [x] Add unit tests for extractor, shell escaping, and command routing
- [x] Run `go test ./...` and fix any failures
- [x] Add short README usage section for `tmux-links`
