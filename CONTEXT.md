## blf

A collection of blazingly-fast misc CLI utilities, dispatched from a single `blf` binary.

## Language

**File reference**:
A clipboard entry that GUI apps interpret as one or more actual files, so pasting drops/attaches the files themselves (not their path text). On macOS this is a file-URL clipboard flavor; on Linux it is the `text/uri-list` MIME type.
_Avoid_: "file path on clipboard" (that's plain text), "copy files" (implies copying bytes / a `cp`-style filesystem op).

**copy-ref**:
The command that puts one or more **File references** on the clipboard: `blf copy-ref <file>...`.
_Avoid_: `copy-files` (reads like a filesystem copy or copying file contents).

**redirect wrapper**:
A URL whose query string embeds a destination URL inside a known parameter, so visiting it redirects to that destination (e.g. `https://www.google.com/url?...&url=<destination>&...`). **clean-url** recognizes these by host + param-name and unwraps them to recover the destination, repeating until no further wrapper matches (nested wrappers).
_Avoid_: "redirect link" (too vague — doesn't capture that the destination is recoverable from the query string itself).

**tracking param**:
A query-string parameter that serves analytics/attribution purposes only (e.g. `utm_source`, `fbclid`, `gclid`) and carries no meaning for the destination. **clean-url** strips these from a URL using one flat global list, regardless of host.
_Avoid_: "junk param" / "garbage param" (imprecise — some non-tracking params may also look superfluous but change behavior).

**clean-url**:
The command that produces a **clean URL** — the result of repeatedly unwrapping **redirect wrappers** and then stripping **tracking params**: `blf clean-url <url>` or `blf clean-url --clipboard` (reads from and writes back to the clipboard, also printing the result).

**agent window**:
A Kitty window currently running a known AI coding agent (`claude`, `codex`, `opencode`, `cursor-agent`). Identity is decided by matching whole command **words** — the first token of `last_reported_cmdline` (primary) or a foreground process's command-word/basename (backup) — never a substring of the full cmdline (a path like `/tmp/claude-501/…` must not count).
_Avoid_: "agent pane" (that's tmux's term; Kitty has windows), matching the agent name anywhere in the cmdline (substring matching produces false positives).

**agent status**:
Whether an **agent window** is currently **working** (actively processing) or **idle** (anything else — finished, or waiting on input). Derived solely from the window's OSC title: a leading braille-spinner char (U+2800–U+28FF) means working, otherwise idle. _Known gap_: OpenCode has no title signal, so it always reads idle.
_Avoid_: "waiting"/"blocked" as a distinct status (deliberately collapsed into idle for now).

**goto-agent**:
The command that opens a picker of all **agent windows** (across every OS window and session) with their **agent status**, and focuses the chosen one: `blf kitty goto-agent`.

**list-agents**:
The non-interactive counterpart that lists **agent windows** with **agent status**, optionally as JSON (`blf kitty list-agents [--json]`); intended as the shared source of truth for external callers (e.g. the nvim send-to-agent feature).

## Relationships

- `blf copy <text>` copies **content** (a string) to the clipboard — the content comes from the arguments, or from stdin when the sole argument is `-` (`blf copy -`).
- `blf copy-ref <file>...` copies **File references** to the clipboard — a reference/handle, not the bytes.
- `blf clean-url` reads a URL (from an argument or the clipboard) and prints (and, in `--clipboard` mode, writes back) the **clean URL**.

## Flagged ambiguities

- "file ref" is also used informally in the `tmux-targets` feature to mean a `path:line[:col]` token detected _inside text_ (a location pointer). That is a different concept from the clipboard **File reference** defined here. Left as-is for now; rename to "file location" later if the overlap causes confusion.
