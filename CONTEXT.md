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
One contributor of launcher results — math, unit/currency conversion, application launch, script, or command. Each inspects the raw query and optionally emits rows into a single ranked list; sources are _not_ mutually-exclusive modes, they self-select on query shape and coexist (ranking: **learned rank** > exact > prefix > source-weight > fuzzy score).
_Avoid_: "mode" (implies an exclusive switch you toggle between).

**script**:
A user-authored (or built-in) bash/osascript snippet, fuzzy-matched by name and executed as an external process (`exec.Cmd`) when picked. Configured/overridden via user config; built-ins ship in code.
_Contrast_: **command**, which runs in-process.

**command**:
A built-in, hardcoded **launcher source** entry (e.g. reload, cleanurl) that runs in-process — calling a Go function directly rather than spawning an external process. Not user-configurable, unlike **script**.
_Avoid_: confusing with a **script** — both are fuzzy-matched, name-triggered actions, but a command never shells out.

**learned rank**:
A per-(exact query text, result target) counter that overrides the default ranking tiers, letting the launcher remember that the user has repeatedly picked a non-top result for a specific query. Any Enter/pick on a result that wasn't first in the list increments the counter for (trimmed query text, Action.Type+Action.Target); the next time that exact query is typed, results with a nonzero counter sort above exact/prefix/source-weight/fuzzy-score matches, highest counter first. Applies to every action type (launch, run, open, copy), not just app launches. Persists permanently (no decay, no in-TUI clear) in a plain, human-editable state file alongside **launcher history**.
_Avoid_: confusing with **source weight** (the static, per-provider `Weight` field on a Result) — learned rank is per-user, per-query, and accumulated from behavior; source weight is a fixed constant a provider assigns to all of its rows.
_Avoid_: "selection preference" / "pick habit" (considered and rejected as ambiguous/imprecise for this glossary).

**source weight**:
The static `Weight` field a **launcher source** assigns to its own result rows, used as a ranking tiebreak below exact/prefix match. Fixed per provider, not learned from usage.
_Contrast_: **learned rank**, which is per-query and accumulated from the user's actual picks.

**directory source**:
The **launcher source** that surfaces filesystem directories — a fixed set of **built-in directories** plus **configured directories** — and opens the selected one in the OS file manager (Finder / Linux file manager) via `ActionOpen`.
_Avoid_: "folder provider" ("directory" is the term used throughout this codebase)

**built-in directory**:
One of the directory entries the **directory source** always includes without configuration: Home, Desktop, Downloads, Documents, iCloud. Silently omitted at query time if its path doesn't exist on the current machine (e.g. iCloud on Linux).

**config file**:
The single TOML file at `~/.config/blf/config.toml` (respecting `XDG_CONFIG_HOME`) holding all of blf's configuration, decoded into `Config` by `LoadConfig`. Currently only the `[launcher]` section is read; other sections are reserved for future commands. Lives in `internal/config`. Edited directly via `blf config edit`, which seeds it with active defaults (real `defaultConfig()` values, not commented-out placeholders) if it doesn't exist yet, then hands off to `$EDITOR`.
_Avoid_: "user config" / "settings file" (the codebase and CLI consistently say "config file").

**configured directory**:
A directory entry the user adds via `[[launcher.directory]]` in the **config file**. If its name matches a **built-in directory**'s name, it replaces that built-in's path (not a duplicate row); otherwise it's added alongside the built-ins.

**computational query**:
A launcher input that resolves to a value — a math expression containing an operator/function (`1+2`, `sqrt(2)`) or a `<number><unit>` conversion (`10cm`, `123$`). A confident computational parse **suppresses** the fuzzy app/script list.
_Contrast_: a **name-like query** (letters that don't resolve to a unit/function) is fuzzy-matched against apps + scripts. A bare number (`1`) is treated as name-like (falls through to fuzzy, e.g. `1`→1Password), except a large number also contributes a comma-formatted row above the matches.

**launcher history**:
The persisted list of things you did via the launcher. For **launch**, **run**, and **open** actions, an entry is the picked result's label and action (e.g. "Kitty" → launch `/Applications/kitty.app`) — not the query text that found it, so different queries resolving to the same item coalesce into one entry. For copy actions (calc/unit/currency), an entry is still the raw query text (explicitly saved via Ctrl+S, since a copy result has no independent identity beyond its query). Shown as the default list when the input is empty (see ADR 0006).
_Avoid_: recording every keystroke-query (only executed/saved entries are history); assuming every entry stores a query — launch/run/open entries store the picked item, not the query.

**history direct-fire**:
Pressing Enter on a **launcher history** row for a launch/run/open entry immediately performs its stored action (e.g. picking "Kitty" launches Kitty) rather than populating the input and recomputing. This is the one place the launcher re-fires an action without the user re-confirming via search (ADR 0006).
_Contrast_: Ctrl+R/Ctrl+F still populate the input with the entry's label (or query, for copy entries) and recompute — a text-recall shortcut for further editing, not an execution shortcut.

**history hint**:
The dimmed-italic computed echo shown beside a **launcher history** row that still stores a query (copy entries only), rendered as the row's subtitle (`= <result>`). A math expression shows `= <value>` (e.g. `10+20` → `= 30`); a currency query shows a single joined line of the configured target currencies, using each currency's symbol where one exists and its ISO code otherwise (e.g. `1$` → `= 2.987 ₪, 0.79 £, 0.92 €`). Computed live from the current rates each time the history list is shown, never persisted. _Excluded_: non-currency unit conversions (too many targets to join meaningfully), bare numbers, and anything that doesn't resolve. Launch/run/open entries never have a hint — they have no stored query to compute one from.
_Avoid_: treating it as a re-fire of the original action (it is display-only; for copy entries the stored query is still what gets recalled/executed — see **history direct-fire** for the launch/run/open case, which does re-fire).

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

**resume** (claude history):
The ctrl+r action on the **conversation** list and **live grep** pages: suspends the TUI and runs `claude --resume <sessionId>` in the owning **project**'s `cwd`, returning to claude history when the resumed session exits. Same suspend/exec/return shape as the **conversation export**'s editor handoff, but execs `claude` itself instead of `$EDITOR`. Not offered on the **project** list, since a project row aggregates many conversations rather than naming one.

**blf beads**:
An interactive TUI (`blf beads`) for browsing and triaging **Beads** issues in the current project, and handing a chosen issue's id to an AI agent for implementation. Embeds the shared **fuzzy finder** widget with a side-by-side **issue preview**, treating the external `bd` CLI (its JSON output) as the sole source of truth — blf never reads the beads database directly. Its primary verb copies the selected id to the clipboard (and prints it to stdout) and quits; secondary verbs create/edit/re-state/close issues and open the dependency graph, all by shelling out to `bd`. Project-contextual: relies on `bd`'s auto-discovery of the `.beads` database from the working directory.
_Avoid_: "beads manager" (it is a browser/picker first; management is secondary), reimplementing beads' data layer (it is a thin front-end over `bd`).

**readiness** (blf beads):
Whether an issue is **unblocked** (actionable now) or **blocked** (waiting on an open dependency), derived authoritatively from membership in `bd ready`'s set — never guessed from a raw dependency count, since a closed blocker still counts. Drives the row indicator, the readiness-bucketed sort (unblocked before blocked, priority within each), and the scope filter.
_Avoid_: inferring blocked-ness from `dependency_count > 0` (counts include already-satisfied blockers).

**issue preview** (blf beads):
The side pane showing the selected issue's full detail, lazily fetched and cached per issue. Presents two **separate** sections that must never be merged, because they are different relationships: a **subtasks tree** (parent→child hierarchy, from an epic downward, with completion count) and a **blocked-by tree** (the transitive dependency chain the issue is waiting on, rooted at the issue and expanded in the "blocked by" direction, with diamonds/cycles collapsed to a back-reference marker).
_Avoid_: merging hierarchy and dependency edges into one tree (two distinct edge semantics), calling the dependency chain a "graph" here (it is rendered as a rooted tree; the full DAG is a separate `bd graph` shell-out).

**create mode** (blf beads):
A transient state that repurposes the always-focused **fuzzy finder** input as a title field instead of a search box (prompt changes, list stops filtering); confirming creates the issue via `bd create`, cancelling restores the prior search. When entered on an **epic** row it defaults the new issue to that epic's child, toggleable to standalone. The same mode-flip mechanism backs status changes (a status-pick variant), so no separate multi-field form is built.
_Avoid_: "create form" (there is no bespoke form — it reuses the one input line; richer fields come from the edit handoff or `bd`).

## Relationships

- `blf copy <text>` copies **content** (a string) to the clipboard — the content comes from the arguments, or from stdin when the sole argument is `-` (`blf copy -`).
- `blf copy-ref <file>...` copies **File references** to the clipboard — a reference/handle, not the bytes.
- `blf clean-url` reads a URL (from an argument or the clipboard) and prints (and, in `--clipboard` mode, writes back) the **clean URL**.
- Pressing Enter on a selected **launcher** result performs its action — launch an app, run a script, or copy a computed value to the clipboard — and records a **launcher history** entry; a successful action then hides the quick terminal.
- Pressing Enter on a **launcher history** row does not go through the normal result-ranking path: for launch/run/open entries it's a **history direct-fire** (immediate action, no recompute); for copy entries it still populates the input and recomputes, same as before.
- A **project** contains one or more **conversations**; selecting a project lists its conversations, and selecting a conversation produces its **conversation export** in `$EDITOR`.
- **claude history** and the **launcher**/**goto-agent** picker all embed the same **fuzzy finder** widget; claude history runs several instances behind a page state machine, with **live grep** supplying its own `rg`-ranked items rather than the widget's fuzzy ranking.
- **blf beads** also embeds the **fuzzy finder** widget: it loads the working set once per scope via `bd list --json` (+ `bd ready --json` for **readiness**), fuzzy-matches client-side, and re-fetches only on scope change, a mutation, or manual refresh. The selected row's **issue preview** is fetched separately (lazy, debounced, cached). Pressing Enter copies the issue id and quits (a picker handoff), unlike the launcher's Enter which performs an action and stays resident.

- The **directory source**'s result set is the union of **built-in directories** and **configured directories**, deduplicated by name (a **configured directory** wins on name collision).

- Picking a non-top **launcher** result increments its **learned rank** for that query; **launcher history** recording (the item+action, for launch/run/open — or the query, for copy) and **learned rank** recording (which result won for that query) both fire from the same Enter/pick event but are separate, independently-persisted stores.

## Flagged ambiguities

- "open" was ambiguous between the **launcher**'s `ActionOpen` (previously wired to a hardcoded macOS-only `open` binary call) and the standalone `blf open` command (already cross-platform via `platform.OpenURL`) — resolved for the **directory source**: `ActionOpen` is rewired to use `platform.OpenURL` too, so both paths share one cross-platform mechanism.
- "file ref" is also used informally in the `tmux-targets` feature to mean a `path:line[:col]` token detected _inside text_ (a location pointer). That is a different concept from the clipboard **File reference** defined here. Left as-is for now; rename to "file location" later if the overlap causes confusion.
