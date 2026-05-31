# blf

A collection of blazingly-fast misc CLI utilities, dispatched from a single `blf` binary.

## Language

**File reference**:
A clipboard entry that GUI apps interpret as one or more actual files, so pasting drops/attaches the files themselves (not their path text). On macOS this is a file-URL clipboard flavor; on Linux it is the `text/uri-list` MIME type.
_Avoid_: "file path on clipboard" (that's plain text), "copy files" (implies copying bytes / a `cp`-style filesystem op).

**copy-ref**:
The command that puts one or more **File references** on the clipboard: `blf copy-ref <file>...`.
_Avoid_: `copy-files` (reads like a filesystem copy or copying file contents).

## Relationships

- `blf copy <text>` copies **content** (a string) to the clipboard.
- `blf copy-ref <file>...` copies **File references** to the clipboard — a reference/handle, not the bytes.

## Flagged ambiguities

- "file ref" is also used informally in the `tmux-targets` feature to mean a `path:line[:col]` token detected *inside text* (a location pointer). That is a different concept from the clipboard **File reference** defined here. Left as-is for now; rename to "file location" later if the overlap causes confusion.
