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

**launcher**:
An always-running, full-screen TUI (`blf launcher`) that lives inside Kitty's quick-access terminal (instance-group `quick`). A single long-lived process that Cmd+2 toggles into view — staying resident is what makes it open blazingly fast (no spawn, no reload on the hot path). Presents one input box over a single ranked result list fed by multiple **launcher sources**.
_Avoid_: "spawned per keypress" (the process is persistent; Cmd+2 only toggles visibility).

**launcher source** (a.k.a. **provider**):
One contributor of launcher results — math, unit/currency conversion, application launch, or script. Each inspects the raw query and optionally emits rows into a single ranked list; sources are _not_ mutually-exclusive modes, they self-select on query shape and coexist (ranking: exact > prefix > source-weight > fuzzy score).
_Avoid_: "mode" (implies an exclusive switch you toggle between).

**computational query**:
A launcher input that resolves to a value — a math expression containing an operator/function (`1+2`, `sqrt(2)`) or a `<number><unit>` conversion (`10cm`, `123$`). A confident computational parse **suppresses** the fuzzy app/script list.
_Contrast_: a **name-like query** (letters that don't resolve to a unit/function) is fuzzy-matched against apps + scripts. A bare number (`1`) is treated as name-like (falls through to fuzzy, e.g. `1`→1Password), except a large number also contributes a comma-formatted row above the matches.

**launcher history**:
The persisted list of launcher queries you executed (pressed Enter on) or explicitly saved (Ctrl+S). Recalled via Ctrl+P/Ctrl+N or shown as the default list when the input is empty. Recalling **populates the input and recomputes** — it never blindly re-fires the original action.
_Avoid_: recording every keystroke-query (only executed/saved queries are history).

**fuzzy finder**:
The shared embedded TUI widget (built on bubbletea v2 + lipgloss) that owns a query input box, a ranked/scrollable/fuzzy-highlighted result list, selection cursor, and the border/footer chrome. Consumers (the **launcher**, the **goto-agent** picker) embed it and own their own item type, ranking, per-row rendering, preview pane, actions, and any live-refresh tickers. Lives in `internal/fuzzyfinder`.
_Avoid_: conflating it with the fzf-based `pick*` shell-outs (`pickSession`, `pickOSWindow`) — those launch the external `fzf` process; the **fuzzy finder** is self-owned Go, which is what lets it refresh live (spinners, status changes).

## Relationships

- `blf copy <text>` copies **content** (a string) to the clipboard — the content comes from the arguments, or from stdin when the sole argument is `-` (`blf copy -`).
- `blf copy-ref <file>...` copies **File references** to the clipboard — a reference/handle, not the bytes.
- `blf clean-url` reads a URL (from an argument or the clipboard) and prints (and, in `--clipboard` mode, writes back) the **clean URL**.
- Pressing Enter on a selected **launcher** result performs its action — launch an app, run a script, or copy a computed value to the clipboard — and records a **launcher history** entry; a successful action then hides the quick terminal.

## Flagged ambiguities

- "file ref" is also used informally in the `tmux-targets` feature to mean a `path:line[:col]` token detected _inside text_ (a location pointer). That is a different concept from the clipboard **File reference** defined here. Left as-is for now; rename to "file location" later if the overlap causes confusion.
