# Claude History

## Problem Statement

I run many Claude Code sessions across many working directories. Every session is recorded as a
`.jsonl` transcript under `~/.claude/projects`, but that store is opaque: the directory names are a
lossy dash-encoding of the original path, the transcripts are dense JSON, and there is no way to
browse, find, or re-read past conversations. When I remember "I solved this before in some Claude
session" I have no practical way to get back to it.

## Solution

A new interactive TUI, `blf claude history`, for browsing the Claude Code transcript store. It opens
on a **project** list, drills into a **conversation** list, and can run an incremental **live grep**
across transcripts. Selecting a conversation produces a readable Markdown **conversation export**
opened in `$EDITOR`. It reuses the existing embedded **fuzzy finder** widget so it looks and feels
like the **launcher** and **goto-agent** picker.

## User Stories

1. As a Claude Code user, I want to run `blf claude history` and immediately see a list of my
   projects, so that I can start browsing my session history with one command.
2. As a user, I want each project shown by its real working-directory name (e.g. `blf`) rather than
   the dash-encoded directory string, so that I can recognise it at a glance.
3. As a user, I want the full project path shown dimmed next to the name (with `$HOME` collapsed to
   `~`), so that I can disambiguate projects with the same basename.
4. As a user with git worktrees, I want worktree projects labelled distinctly from their parent
   project, so that `blf` and its worktree don't look identical.
5. As a user, I want the project list sorted with my most recently active projects first, so that
   what I'm working on now is at the top.
6. As a user, I want to fuzzy-filter the project list by typing, so that I can jump to a project in
   a large list quickly.
7. As a user, I want to pick a project and see all its conversations, so that I can find a specific
   past session.
8. As a user, I want each conversation shown by its title, so that I can tell sessions apart without
   opening them.
9. As a user, I want a sensible title even when a session has no recorded AI title, so that every
   row is meaningful (falling back to the last prompt, then the first user message, then the id).
10. As a user, I want slash-command noise stripped from titles, so that a title reads "generate a
    logo" rather than "/grill generate a logo".
11. As a user, I want each conversation's last-accessed time shown as a relative time with the exact
    timestamp dimmed beside it, so that I can judge recency at a glance and precisely when needed.
12. As a user, I want conversations sorted most-recently-accessed first, so that my latest sessions
    are at the top.
13. As a user, I want to fuzzy-filter the conversation list by typing, so that I can find a session
    by title.
14. As a user, I want to pick a conversation and have it open as readable Markdown in my editor, so
    that I can re-read the whole session comfortably.
15. As a user, I want the exported Markdown to show turns with clear role headings, so that I can
    follow who said what.
16. As a user, I want assistant thinking folded away (collapsible) in the export, so that it's
    available but doesn't drown out the conversation.
17. As a user, I want tool calls shown compactly and tool results truncated, so that huge tool
    outputs don't bury the discussion.
18. As a user, I want system/meta/hook noise stripped from the export, so that I only see the real
    human and assistant turns.
19. As a user, I want subagent (sidechain) turns included and marked in the export, so that I can
    see delegated work in context.
20. As a user, after I close the editor I want to be returned to the conversation list, so that I
    can keep browsing without re-launching.
21. As a user, I want to press Ctrl-F from the project list to grep across all projects, so that I
    can find a phrase anywhere in my history.
22. As a user, I want to press Ctrl-F from the conversation list to grep within just that project,
    so that I can search a single project's sessions.
23. As a user, I want grep to update as I type (incrementally), so that I get fast feedback while
    refining my query.
24. As a user, I want grep matches rendered as readable decoded snippets (not raw escaped JSON),
    with my query highlighted, so that results are legible.
25. As a user, I want each grep result to show which project and conversation it came from, so that
    I know where a match lives.
26. As a user, I want a preview pane showing the matched message in context, so that I can judge
    relevance before opening the full conversation.
27. As a user, I want to toggle grep scope between global and the current project in-page (Ctrl-G),
    so that I can widen or narrow without leaving grep.
28. As a user, I want grep results ordered newest-conversation-first, so that recent matches surface.
29. As a user, I want to press Enter on a grep result to open that conversation in my editor, so
    that I can read the full context of a match.
30. As a user using nvim, I want the editor opened with a search for the matched text, so that I
    land near the relevant part of the conversation.
31. As a user, I want Esc to step back one level (conversations→projects, grep→where I opened it,
    projects→quit), so that navigation is predictable.
32. As a user, I want Ctrl-C to quit from anywhere, so that I can always get out.
33. As a user without ripgrep installed, I want a clear message telling me how to install it (with
    `brew install ripgrep` on macOS) instead of a crash, so that I understand what to do.
34. As a user, I want `blf claude statusline` to work as an alias of the existing
    `claude-statusline`, so that the new `claude` group is internally consistent.
35. As a user, I want the existing `claude-statusline` command to keep working unchanged, so that my
    Claude Code statusline config is not broken.
36. As a user, I want a per-page footer showing the relevant keybindings, so that I can discover
    what I can do on each page.
37. As a user, I want empty states (no projects, no conversations, no matches) shown as a clear
    message rather than a blank screen, so that I know nothing is wrong.

## Implementation Decisions

### Command surface
- New `blf claude` cobra parent group with two subcommands: `history` (this feature) and
  `statusline` (a thin alias delegating to the existing `claude-statusline` implementation).
- The existing flat `claude-statusline` command stays registered and unchanged (it is invoked by
  Claude Code's settings; renaming it would be a breaking change).

### Architecture
- A single bubbletea (v2) program with a `page` state machine: project list → conversation list →
  live grep. Each page embeds its own instance of the existing `internal/fuzzyfinder` widget. This
  mirrors `internal/kitty/agentpicker.go`, which is the reference for embedding the widget.
- The conversation export is **not** an in-TUI page: it is an external `$EDITOR` handoff via
  `tea.ExecProcess`, which suspends the TUI, runs the editor, and resumes back into the page the
  user came from.
- Esc pops one level up the page stack; on the project list Esc quits. Ctrl-C quits everywhere.

### Projects (source of truth)
- The projects root is `$CLAUDE_CONFIG_DIR/projects` when `$CLAUDE_CONFIG_DIR` is set, else
  `~/.claude/projects`.
- A **project** is identified by the real `cwd`, read from the first transcript line in the dir that
  carries a `cwd` field — never by reverse-engineering the dash-encoded directory name (that encoding
  collapses both `/` and `.` to `-` and is not reversible). Fall back to a best-effort de-dashed
  directory name only if no transcript yields a `cwd`.
- Display: primary label `basename(cwd)`, dimmed subtitle = `cwd` with `$HOME` collapsed to `~`.
- Worktree projects (cwd under `/.claude/worktrees/` or `/worktrees/`) get a distinct label that
  identifies both the parent project and the worktree name.
- Sort by the newest `.jsonl` mtime in the directory (a `stat`, no parsing).

### Conversations
- A **conversation** is one `.jsonl` file. Its metadata is extracted cheaply (no full-file/turn
  count): title and last-accessed time.
- Title fallback chain: recorded `aiTitle` → last prompt (`lastPrompt`) → first user message → the
  sessionId. Leading slash-command tokens are stripped from the displayed title.
- Last-accessed = the last message `timestamp` (not file mtime). Displayed as relative time with the
  absolute timestamp dimmed beside it.
- Sort most-recently-accessed first.

### Export
- Render the transcript's records chronologically into Markdown: role headings for user/assistant
  turns; `text` rendered as-is; `thinking` wrapped in a collapsible `<details>`; `tool_use` rendered
  compactly with its input as a fenced JSON block; `tool_result` fenced and truncated to a line
  budget with a truncation marker; sidechain (subagent) turns included and marked in chronological
  position; pure system/meta/hook lines and `<command-*>`/`<local-command-stdout>` wrapper cruft
  stripped.
- The Markdown is written to a temp directory and regenerated on each open (not persisted alongside
  the project, no long-term cache).
- Editor resolution: `$EDITOR`, falling back to `nvim`, then `vi`.

### Live grep
- Engine: spawn `rg` per (debounced ~100ms) keystroke with a minimum query length (~2–3 chars),
  searching the raw `.jsonl` files with line numbers. A match is always contained in one line = one
  message record, so the matched JSON line is parsed and decoded for display.
- Scope is contextual to where Ctrl-F was pressed: from the project list → global (all projects);
  from the conversation list → that single project. In-page Ctrl-G toggles global ↔ current-project.
- Rendering: single-line list rows = `<project> · <conversation title> · …decoded snippet…` with the
  query highlighted via `fuzzyfinder.Highlight`, truncated to width. A bottom preview pane shows the
  full matched message plus a little surrounding context. The grep page gives the finder widget a
  sub-region via `SetSize` and renders the preview below it.
- Items are supplied pre-filtered/ordered by `rg` (newest-conversation-first); the widget's own
  fuzzy ranking is bypassed — only its input box, list and selection are reused.
- On Enter: export the conversation and open `$EDITOR` (same handoff), then resume into the grep
  page. For nvim specifically, pass a search pattern for the matched text so the cursor lands near
  the match; other editors open at the top.
- If `rg` is not on `PATH`, the grep page shows a clear inline message (including `brew install
  ripgrep` on macOS) and the other pages still work.

### Modules
- `internal/claude` (parsing/export, the deep testable core): project enumeration (`ListProjects`),
  transcript parsing + cheap conversation metadata (`ConversationMeta`), Markdown export, and grep
  match decoding (raw JSON line → readable highlighted snippet + context).
- `internal/claudehistory` (the TUI): the page state-machine model, the embedded finders, the grep
  runner (rg wrapper with scope/debounce), the preview pane, and the editor handoff.
- `cmd/claude.go`: the `blf claude` group + `history`/`statusline` wiring.

### Keybindings
- Global: Ctrl-C quit; Esc pop one level (quit on the project list).
- Navigation: up/down + ctrl-j/k + ctrl-n/p (provided by the widget).
- Enter: descend / activate. Ctrl-F: open live grep. Ctrl-G: toggle grep scope.
- Each page renders a footer with its relevant hints.

## Testing Decisions

Good tests here assert **external behavior** through public interfaces, not internals: feed fixture
transcripts / directory trees in and assert on the returned values or rendered strings. Prior art:
`internal/claude/statusline_test.go` (pure function fed a JSON string, golden-style substring
assertions) and `internal/kitty/agentpicker_test.go` (TUI driven by `tea.KeyPressMsg`, assertions on
`View()`).

All modules are tested:
- **Projects** (`ListProjects`): fixture directory trees → assert real-cwd resolution, worktree
  labelling, `~`-collapsed paths, mtime ordering, and the de-dashed fallback when no cwd is present.
- **Transcript + metadata** (`ConversationMeta`): fixture `.jsonl` → assert the title fallback chain,
  slash-noise stripping, and last-timestamp extraction.
- **Export**: fixture transcript → golden Markdown assertions covering role headings, folded
  thinking, compact tool_use, truncated tool_result, sidechain marking, and noise stripping.
- **Grep decode**: raw matched JSON lines → assert the decoded snippet, highlight ranges, and that
  escaped/encoded JSON is rendered legibly.
- **History model** (TUI): drive with key presses (like agentpicker_test) → assert page transitions
  projects→conversations→grep, Esc back-stack behavior, and Ctrl-F/Ctrl-G effects on `View()`.
- **Grep runner**: run against fixture transcript dirs → assert scope selection (global vs single
  project) and result decoding/ordering; assert the graceful message path when `rg` is absent.

## Out of Scope

- Editing, deleting, or otherwise mutating transcripts — this is read-only.
- An in-TUI conversation renderer/pager (preview beyond grep context) — the full read happens in
  `$EDITOR`.
- A persistent metadata cache — deferred unless reading is measured slow.
- Turn-count / token / duration metrics in the conversation list — explicitly dropped.
- Precise cursor placement in the exported Markdown for non-nvim editors.
- A non-interactive / scriptable export subcommand.
- Searching anything other than the local transcript store (no remote/cloud sessions).

## Further Notes

- `claude history` deliberately reuses the same `internal/fuzzyfinder` widget as the launcher and
  goto-agent picker so the three feel identical; the novelty is running several instances behind a
  page state machine and, for grep, feeding it rg-ranked items instead of the widget's fuzzy ranking.
- The cwd-from-transcript decision is recorded in `CONTEXT.md` rather than an ADR (it is surprising
  and a real trade-off, but cheaply reversible, so it does not meet the ADR bar).
- Glossary terms (`claude history`, `project`, `conversation`, `live grep`, `conversation export`)
  are defined in `CONTEXT.md`; use that vocabulary in code and commits.
