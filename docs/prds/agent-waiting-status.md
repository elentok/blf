# PRD: Agent `waiting` status for goto-agent / list-agents

## Problem Statement

When I run `blf kitty goto-agent` (or `list-agents`), an **agent window** only ever
shows as **working** or **idle**. But "idle" hides the case I most need to see: an
agent that has *stopped and is blocked on me* — a permission prompt or a question.
Those windows look identical to ones that are genuinely finished, so I can't tell at
a glance which agent is waiting for my input, and the one thing demanding my
attention is buried in the same bucket as everything that needs nothing.

## Solution

Add a third **agent status**, **waiting**, for an agent blocked on user input or
permission. The picker surfaces waiting agents first, with a distinct icon, so "what
needs me right now" is always at the top.

The status is **authored by the agent itself**: it sets a Kitty per-window user var
(`AGENT_STATE`) from its own event hooks, and blf reads that var back from the same
`kitty @ ls` payload it already fetches — no extra per-window calls, no screen
scraping. A one-shot, re-runnable `blf kitty setup-claude` wires Claude Code's hooks
to report state automatically. Agents without a wired hook keep working/idle exactly
as today.

## User Stories

1. As a developer juggling several agent windows, I want to see which agent is **waiting** on my input, so that I can unblock it without hunting through every window.
2. As a developer, I want **waiting** agents sorted to the top of the `goto-agent` picker, so that the most urgent window is one keystroke away.
3. As a developer, I want a **distinct icon** for **waiting** (vs working/idle), so that I can triage status at a glance.
4. As a developer, I want **waiting** to be visually distinct in color (yellow), so that it reads as "attention needed" against green working / dim idle.
5. As a developer, I want `list-agents --json` to expose the new `waiting` status, so that external callers (e.g. the nvim send-to-agent feature) can react to it.
6. As a Claude Code user, I want a single `blf kitty setup-claude` command to install the reporting hooks, so that I don't hand-edit `~/.claude/settings.json`.
7. As a Claude Code user, I want `setup-claude` to be **idempotent**, so that I can re-run it any time (after upgrades, or to repair drift) without creating duplicate hooks.
8. As a Claude Code user, I want `setup-claude` to **prune obsolete** `blf kitty set-agent-state` hooks before adding the current set, so that renamed/removed events self-heal.
9. As a Claude Code user, I want `setup-claude` to **preserve** my other settings (permissions, env, unrelated hooks), so that installing status reporting never clobbers my config.
10. As a Claude Code user, I want a `--dry-run` flag on `setup-claude`, so that I can preview the change before it writes.
11. As an agent, I want my window to report **working** when I start a turn, so that the picker shows me as busy.
12. As an agent, I want to report **waiting** when I need input or permission, so that the developer is alerted.
13. As an agent, I want to report **working** again after a permission prompt is approved, so that I don't appear stuck on **waiting** mid-turn.
14. As an agent, I want to report **idle** when my turn finishes, so that the picker shows me as available.
15. As a developer running an agent **outside Kitty**, I want `set-agent-state` to no-op silently, so that hooks never error in a non-Kitty terminal.
16. As a developer, I want `set-agent-state` to print **nothing to stdout** on success, so that a Claude `UserPromptSubmit` hook never leaks output into the model's context.
17. As a developer using an agent **without** a status hook, I want it to keep showing working/idle from the title spinner, so that the feature degrades gracefully.
18. As a developer, I want the **user var** to win over the title spinner when both are present, so that the agent's own report is authoritative.
19. As a developer, I want `goto-agent` and `list-agents` to stay a **single `kitty @ ls` call**, so that adding statuses doesn't slow the picker.
20. As a maintainer, I want the `AGENT_STATE` vocabulary owned in one place (blf), so that the writer and reader can't drift.

## Implementation Decisions

Grounded in **ADR 0004** (agent status via an agent-pushed Kitty user var, not screen
scraping) and the **CONTEXT.md** glossary (`agent status`, `agent-state user var`,
`set-agent-state`, `setup-claude`).

- **M1 — Status model (`internal/kitty`, modify).**
  - Add `StatusWaiting Status = "waiting"` alongside `working`/`idle`.
  - Add `UserVars map[string]string` to the window model and parse it from the
    `kitty @ ls` JSON field `user_vars`.
  - `statusForAgent` takes the window's user vars + title and applies precedence:
    if `AGENT_STATE` is a recognized value (`working`/`waiting`/`idle`) it is
    authoritative; otherwise fall back to today's title-spinner detection
    (working/idle). Unknown/empty var → fallback. The OSC title can never yield
    `waiting`.
- **M2 — `set-agent-state` writer (new command, thin).**
  - `blf kitty set-agent-state <working|waiting|idle>`; reject any other value with
    a non-zero exit + stderr message.
  - Runs `kitty @ set-user-vars AGENT_STATE=<state>` (targets the calling window via
    `KITTY_WINDOW_ID`).
  - No-op (exit 0, silent) when `KITTY_WINDOW_ID` is unset.
  - **Silent on stdout** in all success paths.
- **M3 — Claude hooks reconciler (new, deep) + `setup-claude` command.**
  - The reconciler is a **pure function**: input = parsed settings (generic
    `map[string]any` to preserve unknown keys by value); output = updated settings.
  - It removes from every hook event any entry whose `command` matches
    `blf kitty set-agent-state` (narrow match — never touches other `blf kitty …`
    hooks), drops any event key left with an empty array, then inserts the canonical
    managed groups.
  - Canonical hook set:

    ```
    UserPromptSubmit → blf kitty set-agent-state working
    PreToolUse (matcher "*") → blf kitty set-agent-state working
    Notification → blf kitty set-agent-state waiting
    Stop → blf kitty set-agent-state idle
    ```

    (`PreToolUse` is the resume-from-waiting recovery; the non-tool events take no
    matcher.)
  - `setup-claude` command: read global `~/.claude/settings.json` (treat
    missing/empty as `{}`), run the reconciler, write back with 2-space indent
    (accepting key re-sort/reformat as a one-time churn). `--dry-run` prints the
    resulting diff instead of writing. Writes only the global user settings, never
    project-local or `settings.local.json`.
- **M4 — Display + ordering (`internal/kitty`, modify).**
  - Three-way sort: `waiting` → `working` → `idle`, each tier by `LastFocusedAt`
    descending.
  - Distinct glyphs/styles: working `U+F08FF` (green, `"2"`), waiting `U+F0F3`
    (yellow, `"3"`), idle `U+F49E` (dim/faint).
- **JSON contract.** `status` gains the `"waiting"` value; existing `working`/`idle`
  consumers are unaffected.

## Testing Decisions

Good tests here assert **external behavior** through the modules' public seams, not
internals. Prior art: `internal/kitty/agents_test.go` already uses table tests
(`TestStatusForAgent`, `TestCleanTitle`), a stub `Deps` (`RunCommand`, `ReadFile`,
`WriteFile`, `LookupEnv`, …), sort assertions (`TestListAgentsDropsCurrentWindowAndSorts`),
glyph assertions (`TestStatusGlyphDiffersByStatus`), and a JSON-contract test
(`TestListAgentsCommandJSONContract`). Reuse that style.

All four modules are tested:

- **M1 (status precedence).** Table tests over `statusForAgent`: user var `waiting`
  wins regardless of title; user var `working`/`idle` wins over title; unknown/empty
  var falls back to title spinner; a window with no user vars behaves exactly as
  today. Extend the JSON-contract test to cover a `waiting` agent.
- **M2 (`set-agent-state` writer).** With a stub `Deps`: a valid state issues the
  expected `kitty @ set-user-vars AGENT_STATE=<state>` `RunCommand` call; an invalid
  state exits non-zero and calls nothing; `KITTY_WINDOW_ID` unset → no `RunCommand`,
  exit 0; stdout is empty on every success path.
- **M3 (hooks reconciler).** Pure-function table tests (no filesystem): empty/`{}`
  input gets the canonical set; running twice is a no-op (idempotent); a stale
  `blf kitty set-agent-state` hook under a renamed event is pruned and its emptied
  event key removed; unrelated hooks, `permissions`, and `env` survive untouched; an
  unrelated `blf kitty other-cmd` hook is left alone. A thin `setup-claude` test with
  stub `ReadFile`/`WriteFile` covers missing-file → `{}` and `--dry-run` (no write).
- **M4 (icons + ordering).** Sort test asserting `waiting → working → idle` then
  recency; glyph test asserting all three statuses render distinct glyphs.

## Out of Scope

- Setup commands for **codex** and **opencode** (each has its own hook/notify
  mechanism; they can call the same `blf kitty set-agent-state` later). This PRD ships
  `setup-claude` only.
- Any **screen-scraping** / `kitty @ get-text` status inference, or a manifest/pattern
  engine (explicitly rejected in ADR 0004).
- Using `needs_attention` / terminal bell as a signal (rejected in ADR 0004).
- A `Done` status distinct from `idle`.
- OpenCode gaining a `waiting` signal via the title (it has no title signal).
- Auto-running `setup-claude` on install, or managing a backup/rollback of
  `settings.json` beyond `--dry-run`.

## Further Notes

- The writer and reader share one vocabulary owned by blf; `AGENT_STATE` + the
  `set-agent-state` state words are now an external contract that hooks depend on
  (see ADR 0004) — renames must bump `setup-claude` to also prune the old token.
- Verification after build: run `blf kitty setup-claude`, start a Claude window, hit a
  permission prompt, and confirm the agent shows **waiting** (yellow `U+F0F3`) and
  sorts to the top of `goto-agent`; approve it and confirm it flips back to working.
