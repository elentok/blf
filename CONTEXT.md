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

**history hint**:
The dimmed-italic computed echo shown beside a **launcher history** row, rendered as the row's subtitle (`= <result>`). Only **computational queries** that resolve to a value get one: a math expression shows `= <value>` (e.g. `10+20` → `= 30`); a currency query shows a single joined line of the configured target currencies, using each currency's symbol where one exists and its ISO code otherwise (e.g. `1$` → `= 2.987 ₪, 0.79 £, 0.92 €`). Computed live from the current rates each time the history list is shown, never persisted. _Excluded_: non-currency unit conversions (too many targets to join meaningfully), bare numbers, and anything that doesn't resolve.
_Avoid_: treating it as a re-fire of the original action (it is display-only; the stored query is still what gets recalled/executed).

**fuzzy finder**:
The shared embedded TUI widget (built on bubbletea v2 + lipgloss) that owns a query input box, a ranked/scrollable/fuzzy-highlighted result list, selection cursor, and the border/footer chrome. Consumers (the **launcher**, the **goto-agent** picker) embed it and own their own item type, ranking, per-row rendering, preview pane, actions, and any live-refresh tickers. Lives in `internal/fuzzyfinder`.
_Avoid_: conflating it with the fzf-based `pick*` shell-outs (`pickSession`, `pickOSWindow`) — those launch the external `fzf` process; the **fuzzy finder** is self-owned Go, which is what lets it refresh live (spinners, status changes).

**claude history**:
An interactive TUI (`blf claude history`) for browsing the Claude Code transcript store under `$CLAUDE_CONFIG_DIR`/`~/.claude/projects`. A single bubbletea program with three in-TUI pages (**project** list → **conversation** list → **live grep**) plus an external editor handoff for the **conversation export**. Lives under the `blf claude` command group alongside `claude statusline` (an alias of the existing `claude-statusline`).

**project** (claude history):
One directory under the projects root, representing all Claude Code **conversations** that ran in a single working directory. Identified by the real `cwd` read from a transcript line — never by reverse-engineering the lossy dash-encoded directory name. Displayed as `basename(cwd)` with the `~`-collapsed path; **worktree** projects (cwd under `/.claude/worktrees/`) are labelled distinctly so they don't collide with their parent project.
_Avoid_: deriving the name from the directory string (it collapses both `/` and `.` to `-` and can't be reversed).

**conversation** (a.k.a. **session**):
One `.jsonl` transcript file — a single Claude Code session. Its title is the recorded `aiTitle`, falling back to the last prompt, then the first user message, then the sessionId.

**live grep** (claude history):
The claude history page that runs `rg` over the raw transcript files on each (debounced) keystroke and renders each match as a decoded, human-readable snippet with the query highlighted, plus a preview pane showing the matched message in context. Scope is contextual: global when opened from the **project** list, single-**project** when opened from the **conversation** list, toggleable in-page.
_Avoid_: "search" (too generic — this is incremental ripgrep over JSON transcripts with decoded rendering).

**conversation export**:
The rendering of a **conversation**'s `.jsonl` into Markdown (chronological role-headed turns; thinking folded, tool results truncated; system/meta noise stripped), opened in `$EDITOR` from inside the TUI. The entry point for previewing a conversation's full content.

## Relationships

- `blf copy <text>` copies **content** (a string) to the clipboard — the content comes from the arguments, or from stdin when the sole argument is `-` (`blf copy -`).
- `blf copy-ref <file>...` copies **File references** to the clipboard — a reference/handle, not the bytes.
- `blf clean-url` reads a URL (from an argument or the clipboard) and prints (and, in `--clipboard` mode, writes back) the **clean URL**.
- Pressing Enter on a selected **launcher** result performs its action — launch an app, run a script, or copy a computed value to the clipboard — and records a **launcher history** entry; a successful action then hides the quick terminal.
- A **project** contains one or more **conversations**; selecting a project lists its conversations, and selecting a conversation produces its **conversation export** in `$EDITOR`.
- **claude history** and the **launcher**/**goto-agent** picker all embed the same **fuzzy finder** widget; claude history runs several instances behind a page state machine, with **live grep** supplying its own `rg`-ranked items rather than the widget's fuzzy ranking.

## Flagged ambiguities

- "file ref" is also used informally in the `tmux-targets` feature to mean a `path:line[:col]` token detected _inside text_ (a location pointer). That is a different concept from the clipboard **File reference** defined here. Left as-is for now; rename to "file location" later if the overlap causes confusion.
