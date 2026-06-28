# ADR 0004: Agent status via an agent-pushed Kitty user var, not screen scraping

**Status**: Accepted

## Context

`agent status` (shown by `goto-agent` / `list-agents`) needs a third state,
**waiting** — an agent blocked on user input or a permission prompt — alongside
the existing **working** / **idle**.

Today status is derived solely from the window's OSC title (braille spinner =
working, else idle). The title can distinguish working from not-working, but it
**cannot** tell "done and idle" from "blocked and waiting" — both states stop the
spinner. So a new signal is required.

The reference implementation for richer status is herdr, which has
Idle/Working/Blocked/Unknown. But herdr is a terminal **multiplexer**: it owns
the PTYs, so it continuously reads each pane's live screen text and matches it
against per-agent manifests of prompt-chrome patterns. blf does **not** own the
terminal — to copy that approach it would have to call `kitty @ get-text` per
agent window on every listing and maintain a pattern engine.

A cheaper-looking signal, Kitty's per-window `needs_attention` flag, was also
considered and rejected: it only sets when the program emits a terminal `BEL`
(opt-in, and Claude doesn't by default), and Kitty clears it the moment the
window gains focus — so it is both unreliable and focus-fragile.

## Decision

The agent **pushes its own status** as a Kitty per-window user var
`AGENT_STATE` (`working`/`waiting`/`idle`), and blf **reads it from the same
`kitty @ ls` payload** it already fetches to enumerate windows.

- **Writer**: `blf kitty set-agent-state <state>` wraps
  `kitty @ set-user-vars AGENT_STATE=<state>` (defaults to the calling window via
  `KITTY_WINDOW_ID`; no-ops silently outside Kitty). Agents invoke it from their
  event hooks. For Claude Code: `UserPromptSubmit`, `PreToolUse` and
  `PostToolUse` → working, `Notification` → waiting, `Stop` → idle. `PostToolUse`
  is what clears **waiting** after the user answers a question or permission
  prompt — the tool completing is the reliable re-engagement signal, since
  answering through the selector is not a `UserPromptSubmit`. After the final
  tool, `PostToolUse` sets working but `Stop` fires last and wins (idle).
- **Reader**: when `user_vars["AGENT_STATE"]` is present it is authoritative;
  otherwise blf falls back to today's OSC-title spinner detection.

blf owns the **vocabulary and the read**; each agent owns **when** it transitions.

### Consequences (the shape this forces)

- **Status detection stays a single free `ls` call** — zero per-window calls, no
  `get-text`, no pattern/manifest engine to maintain. The performance worry that
  motivated this design is avoided by construction.
- **`AGENT_STATE` + `set-agent-state` are now a contract.** External hooks depend
  on the var name and the state words; changing them breaks every wired agent.
- **Graceful degradation.** Agents without a wired hook never report **waiting**
  and fall back to working/idle; OpenCode (no title signal either) stays idle.
- **The agent is trusted to be honest** about its own state, and to fire the
  resume (`PreToolUse`/`PostToolUse`) hook — a missed transition leaves a stale
  var until the next event.
- **`set-agent-state` must print nothing to stdout.** A Claude Code
  `UserPromptSubmit` hook's stdout is injected into the model's context, so any
  confirmation output would silently pollute every prompt. The command writes
  only to Kitty's socket; success is mute (errors → stderr / non-zero exit).

## Alternatives considered

- **Screen-scrape via `kitty @ get-text` + per-agent patterns (herdr's model).**
  Rejected — N per-window calls per listing and a pattern engine to maintain, for
  an architecture (blf doesn't own the PTY) where screen text isn't free. Right
  for herdr, wrong for blf.
- **Infer waiting from Kitty's `needs_attention` bell flag.** Rejected — opt-in to
  a terminal bell the agent must emit, and cleared on focus; produces neither a
  reliable nor a focus-stable signal.
- **Derive waiting from the OSC title.** Rejected — impossible in principle; the
  title cannot distinguish done-idle from blocked-waiting.
