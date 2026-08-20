# Launcher AI helpers

## Problem Statement

I regularly want a quick answer from a model, or a quick grammar/spelling/writing pass over
something I just wrote — a Slack message, a commit body, a paragraph in a doc. Today that means
leaving what I'm doing, finding a terminal or a browser tab, pasting the text, waiting, reading the
answer, copying it back. The context switch costs more than the task.

The launcher is already the thing I hit for "do a small thing right now" (Cmd+2, type, Enter, gone).
It has no way to reach a model.

## Solution

Two new built-in launcher **commands**, `ai` and `improve`.

Picking either flips the launcher into **ai prompt mode**: the input line's prompt changes, the
footer shows the keys, and the result list is replaced by a full-viewport preview reading
`Press enter to use the clipboard:` followed by the clipboard's contents. I either type my
prompt/text, or leave the input empty to send the clipboard as previewed. Enter fires the request in
the background and the launcher immediately resets and hides — I'm back to what I was doing within a
second.

When the request finishes, a system notification shows the response and the response is copied to
the clipboard. Every run is appended to the **runs store**, and the newest successful runs appear at
the top of the launcher's recent items, where Enter copies the response to the clipboard again.

`improve` is the same machinery with a fixed instruction wrapped around the input: fix the grammar,
spelling, punctuation and flow, change nothing else.

Requests go to a cheap, fast model (`haiku` by default) invoked with no tools, no MCP servers and no
user settings — minimum context, minimum latency.

## User Stories

1. As a launcher user, I want to type `ai` and pick it, so that I can ask a model a question without
   leaving my current window.
2. As a launcher user, I want the launcher to flip into a prompt mode rather than opening a separate
   window, so that the interaction stays inside the tool I already have open.
3. As a launcher user, I want the prompt line to visibly change when I'm in ai prompt mode, so that
   I never confuse it with a normal launcher query.
4. As a launcher user, I want the footer to tell me which keys do what while I'm in ai prompt mode,
   so that I don't have to remember the mode's bindings.
5. As a launcher user, I want that key legend to survive background activity, so that an apps
   reindex or a config error finishing in the background doesn't wipe the footer while I'm typing.
6. As a launcher user, I want to leave the input empty to send whatever is on my clipboard, so that
   I don't have to retype or paste text I have already copied.
7. As a launcher user, I want the whole viewport to show `Press enter to use the clipboard:` and the
   clipboard's contents while the input is empty, so that I can read exactly what I'm about to send
   rather than a teaser of it.
8. As a launcher user, I want the text I saw in that preview to be exactly the text that gets sent,
   so that the preview can never lie to me.
9. As a launcher user, I want that preview to disappear as soon as I start typing, so that the
   screen reflects what will actually be sent.
10. As a launcher user, I want the arrow and navigation keys to do nothing while the preview is
    showing, so that I can't highlight or select preview lines as if they were results.
11. As a launcher user, I want a clear inline error if I press Enter with both an empty input and an
    empty clipboard, so that I'm not left wondering whether anything was sent.
12. As a launcher user, I want that error to leave me in the mode with my cursor still in the input,
    so that I can fix it immediately instead of starting over.
13. As a launcher user, I want the error to clear as soon as I type, so that a stale error doesn't
    sit on screen while I'm fixing it.
14. As a launcher user, I want Esc in ai prompt mode to return me to the normal launcher rather than
    hiding the terminal, so that a mistaken pick costs me one key, not a re-open.
15. As a launcher user, I want the launcher to hide immediately when I press Enter, so that I can go
    back to my work while the model is thinking.
16. As a launcher user, I want the response delivered as a system notification, so that I find out
    it's ready without watching for it.
17. As a launcher user, I want the notification body to hold the full response including its line
    breaks, so that short answers need no further action at all.
18. As a launcher user, I want the notification title to name the helper that produced it (`ai` or
    `improve`), so that I know which of my two helpers is reporting back.
19. As a launcher user, I want the response copied to my clipboard automatically, so that I can
    paste it straight into whatever I was writing.
20. As a launcher user, I want the clipboard to hold the untruncated response even when the
    notification truncates it for display, so that a long answer isn't lost to a display limit.
21. As a launcher user, I want to type `improve` and pick it, so that I can clean up my writing with
    one action.
22. As a launcher user, I want `improve` to read the clipboard when I leave the input empty, so that
    the copy-improve-paste loop is three keystrokes.
23. As a launcher user, I want `improve` to also accept typed text, so that I can fix a short phrase
    without copying it first.
24. As a launcher user, I want `improve`'s instruction to tell the model to preserve my voice, tone,
    register and formatting, so that the result still reads as something I wrote.
25. As a launcher user, I want `improve`'s instruction to forbid adding, removing or reinterpreting
    content, so that I can paste its output back without re-reading it word by word.
26. As a launcher user, I want `improve`'s instruction to forbid translating, so that writing in a
    non-English language stays in that language.
27. As a launcher user, I want `improve`'s instruction to demand only the corrected text, so that I
    don't have to strip a preamble before pasting.
28. As a launcher user, I want my text delimited from the instruction, so that something I wrote
    can't be read as an instruction to the model.
29. As a launcher user, I want to fire several requests at once, so that a slow one doesn't block a
    quick one.
30. As a launcher user, I want a failed request to notify me with the error, so that I don't sit
    waiting for a response that will never arrive.
31. As a launcher user, I want a failed request to leave my clipboard untouched, so that a failure
    never destroys what I had copied.
32. As a launcher user, I want a request that hangs to be killed after a timeout and reported as a
    timeout, so that a stuck process doesn't linger forever.
33. As a launcher user, I want a missing `claude` binary reported like any other failure, so that a
    PATH problem shows up as a notification rather than as silence.
34. As a launcher user, I want every run recorded to the runs store, so that a notification I miss
    is not a response I lost.
35. As a launcher user, I want failed runs recorded too, with their status, so that I can see what
    went wrong after the fact.
36. As a launcher user, I want the store to be plain JSON-lines, so that I can read it with `jq`
    without any blf-specific tooling.
37. As a launcher user, I want the newest successful runs shown above my normal recent items, so
    that I can get back to a response through the tool I'm already in.
38. As a launcher user, I want AI runs kept out of the 30-entry launcher history, so that using the
    helpers never evicts my apps, scripts and calculations from recent items.
39. As a launcher user, I want an AI row to show my input as its title, so that I can recognise the
    run I'm looking for.
40. As a launcher user, I want an AI row to show the first line of the response as its subtitle, so
    that I can often identify it without opening anything.
41. As a launcher user, I want pressing Enter on an AI row to copy that response to the clipboard
    and hide the launcher, so that re-using an earlier answer costs no model call and no waiting.
42. As a launcher user, I want failed runs kept out of the AI rows, so that the list only offers
    rows that can actually do something.
43. As a launcher user, I want `ctrl+x` on an AI row to delete that run from the store, so that I
    can forget something sensitive I sent.
44. As a launcher user, I want the recent list refreshed when a run completes while my input is
    empty, so that the new row is there the next time I open the launcher.
45. As a launcher user, I want a run completing while I have a query typed to leave my list alone,
    so that the results don't move under my cursor mid-search.
46. As a launcher user, I want the runs store capped, so that it doesn't grow without bound over
    years of use.
47. As a launcher user, I want to configure which model is used, so that I can trade speed for
    quality on a given machine.
48. As a launcher user, I want to configure the timeout, so that I can allow slower models more
    room.
49. As a launcher user, I want an unparseable timeout in my config reported as a config error and
    the default used, so that a typo doesn't silently break the feature.
50. As a launcher user, I want both new keys present in the seeded config and in the config schema,
    so that `blf config edit` shows them and my editor can validate them.
51. As a launcher user, I want blf to invoke `claude` itself rather than a script in my dotfiles, so
    that blf works on a machine where my dotfiles aren't installed.
52. As a launcher user, I want the request sent with no tools, no MCP servers and no user settings,
    so that it's as fast and as cheap as it can be.
53. As a launcher user, I want long clipboard contents sent safely however big they are, so that a
    whole document doesn't hit an argument-length limit.
54. As a launcher user, I want `ai` and `improve` to behave like every other launcher command, so
    that there is nothing new to learn about finding or picking them.

## Implementation Decisions

### Vocabulary

Three new terms, to be added to `CONTEXT.md`'s glossary alongside **command** / **script**:

- **ai run** — one request: a kind (`ai` or `improve`), the resolved input, the response, and a
  status.
- **runs store** — the JSON-lines file of **ai run** records; both the durable log and the source of
  the AI rows in recent items.
- **ai prompt mode** — the transient input mode the launcher enters when `ai` or `improve` is
  picked.

`ai` and `improve` are **commands** in the CONTEXT.md sense — built-in, hardcoded, in-process, not
user-configurable (ADR 0007). They are not **scripts**: blf never shells out to a user-authored
snippet, and never depends on `~/.dotfiles/extra/scripts/haiku`.

**ai prompt mode** is a mode flip on the shared **fuzzy finder** widget, in the same family as
`blf beads`' create/status-pick modes. Note it is a deliberate exception to the glossary's "launcher
sources are not mutually-exclusive modes" rule: the *sources* remain non-modal; ai prompt mode is a
transient input mode layered above them, entered explicitly and exited with Esc.

### New package: `internal/launcher/ai`

Pure logic, no bubbletea import, sibling of `internal/launcher/scripts` and
`internal/launcher/history`. It owns:

- **Prompt construction.** `ai` passes the input through verbatim — matching the `haiku` script,
  keeping context minimal. `improve` wraps the input in a fixed instruction (below).
- **Invocation.** The subprocess call is an injected func, so tests assert the argv and the stdin
  payload and can fake stdout, non-zero exits, a missing binary and timeouts.
- **The runs store.** Append, cap, load, delete-by-id.

Input resolution (typed input, else the clipboard snapshot) lives in the launcher model, not here —
the snapshot is taken at mode entry, so it is model state by the time a run is dispatched.

### The claude invocation

`claude -p --model <model> --strict-mcp-config --mcp-config '{"mcpServers":{}}' --disallowed-tools
"Bash Read Write Edit Glob Grep Task WebFetch WebSearch TodoWrite NotebookEdit Agent Skill"
--settings '{}'`

- The flag set mirrors `~/.dotfiles/extra/scripts/haiku`. `--settings '{}'` and `--strict-mcp-config`
  are load-bearing for latency, not just safety — they stop global settings and MCP servers from
  loading.
- The flag list is blf's contract, not a user knob. It is not configurable.
- **The prompt is written to the process's stdin, never passed as an argv element.** A clipboard
  holding a whole document must not hit `ARG_MAX` or be mangled by quoting. This is the one
  deviation from the reference script, which passes the prompt as argv; that `claude -p` with no
  positional prompt consumes stdin as the prompt has been verified against the installed CLI.
- Timeout is enforced with a `context` deadline that kills the process; a timeout is reported as a
  distinct failure status, not a generic error.
- A `claude` binary missing from the resident launcher's `PATH` surfaces as an ordinary failed run —
  a `✗` notification carrying the exec error, and a `status` of failure in the store. This is the
  most likely first-run failure, because the resident process's environment need not match the
  user's interactive shell.

### The `improve` prompt

The input is delimited with XML-style tags — the documented convention for Claude models, and it
keeps arbitrary user text from being read as instructions:

```
<instructions>
Fix grammar, spelling and punctuation in the text below, and improve clarity and flow.
Preserve the author's voice, tone, register and formatting (markdown, line breaks, code).
Do not add, remove or reinterpret content. Do not translate.
Output only the corrected text, with no preamble, explanation or quoting.
</instructions>

<text>
...the resolved input...
</text>
```

The instruction's *effect* on the model is not blf's to guarantee; the testable commitment is that
the prompt is constructed with these instructions and this delimiting.

### Configuration

A new `[launcher.ai]` section in the **config file**, decoded by `internal/config`:

- `model` — string, default `"haiku"`
- `timeout` — string, default `"120s"`, parsed with `time.ParseDuration`

`timeout` is a string field rather than a `time.Duration` because BurntSushi/toml cannot decode into
a `time.Duration` (it is not a `TextUnmarshaler`) and an integer would be read as raw nanoseconds. An
unparseable value falls back to the default and raises a config error through the existing
`ConfigErr` path.

Both keys are deliverables in two places: `DefaultConfig()` (so `blf config edit` seeds them as
active values, per the config file's documented convention) and `internal/config/config.schema.json`
(embedded and shipped to the user's config dir).

Nothing else is configurable.

### Entering the mode

`commands.Command.Run` returns a `tea.Cmd`, which cannot mutate the model. So `ai` and `improve`
each return a command that emits an "enter ai prompt mode" message carrying the kind; the launcher's
`Update` handles that message and flips the mode. This follows the existing `ReloadDoneMsg` /
`CleanURLDoneMsg` shape exactly and needs no change to the `Command` type.

Because they are ordinary commands, picking `ai` or `improve` records a normal history entry for the
command itself, exactly like `reload` and `cleanurl`. That is deliberate: an `ai` row in recent items
that re-enters the mode is useful, and history's move-to-front dedup keeps it to a single row. Note
this is the *command* being recorded, not the run.

### Mode behaviour

- On entry: the widget prompt becomes the kind (`ai `/`improve `), the footer shows the key legend,
  and the clipboard is read **once** into a snapshot held on the model.
- While the input is empty, the entire result viewport is replaced by `Press enter to use the
  clipboard:` followed by the snapshot's contents. This is **new plumbing, not reuse** — the
  `scriptOutput` path replaces `m.results` with ordinary *selectable* rows, and the fuzzy finder
  widget has no overlay API. It requires a mode/preview pointer visible to the `RenderRow` closure
  (bubbletea copies the model each Update, so the closure can only see data through pointers — the
  launcher already does this with `resultsRef`, and beads with `modeRef`), a `RenderRow` branch, and
  `SetItemCount` over the preview's lines.
- Navigation keys are suppressed and results are not recomputed while in mode, mirroring `blf beads`.
  With nav suppressed, no preview line can be highlighted or selected, so Enter never consults the
  selection while in mode.
- The first typed character removes the preview and restores the normal viewport.
- **The snapshot is what gets sent.** The clipboard is not re-read at Enter: with the full contents
  on screen, re-reading could send something other than what was shown.
- Enter with resolved input (typed text, else the snapshot): dispatch the run as a background
  `tea.Cmd`, then reset and hide the terminal via the normal path. Runs are unbounded in number and
  independent of each other.
- Enter with an empty input and an empty snapshot: show an inline error and stay in the mode, cursor
  intact.
- Esc: exit the mode back to the normal launcher, leaving the terminal visible.

`updateFooter` becomes **mode-aware**. Today it knows only `m.status`, `ConfigErr` and `"?: help"`,
and it is called from background message handlers (apps reindex, reload, the 30-minute refresh),
which would otherwise clobber the mode's key legend mid-typing. In mode it renders the key legend,
or the mode's inline error when one is set; the error is cleared on the next keystroke. The mode's
error does not go through `m.status`, whose `clearStatusMsg` prefix allow-list knows nothing about
modes, and which is still holding `"running…"` from the command dispatch that entered the mode.

### Completion

A run's completion message is handled in the launcher's `Update`, which:

1. appends the run to the runs store (success or failure) and rewrites the file,
2. on success, copies the response to the clipboard, then shows a notification titled `ai` /
   `improve` with the full response as its body,
3. on failure, shows a notification titled `✗ ai` / `✗ improve` with the error, and leaves the
   clipboard untouched,
4. recomputes the visible results **only when the input is empty** — so a newly-completed run is
   already on the list the next time the launcher is shown, without disturbing a query the user has
   typed.

Step 4 is required because the launcher is a resident process (ADR 0002): reshowing at the same size
emits no `WindowSizeMsg`, which is why `resetAndHide` pre-populates the list in the first place. A
run completing after the hide would otherwise be invisible until the user typed and deleted a
character.

Because every store write happens inside `Update`, and bubbletea serialises messages, concurrent
runs cannot race on the file. That serialisation is the only thing making unbounded concurrency
safe, so no run may write the store from its own goroutine.

The response is only whitespace-trimmed. No markdown-fence stripping: guessing would corrupt an
answer that legitimately is a code block. If `improve` fences its output in practice, the fix goes
in the prompt.

Multi-line notification bodies are supported by the existing `platform.ShowNotification` — verified
against `osascript`, which accepts both escaped and raw newlines inside a `display notification`
string, and whose escaper already doubles backslashes. No change to the platform helper is needed,
but the multi-line case is untested ground today (its only current caller passes a single-line
literal) and gains a test.

### The runs store

One JSON-lines file at `$XDG_STATE_HOME/blf/ai-runs.jsonl` (falling back to
`~/.local/state/blf/ai-runs.jsonl`), reusing `config.XDGStateDir` and the directory/file permission
conventions of `internal/launcher/history` (`0700` / `0600`).

Each record carries: id, timestamp, kind, resolved input, response, status. Ordered most-recent
first, capped at 200 records. The file is **rewritten whole** on every write, as `history.Save` does
— capping requires it, and it means a crash mid-write loses the file rather than corrupting one
line. A missing or unparseable file loads as empty rather than erroring, matching history.

The store is created in `cmd/launcher.go` and its handle passed to **both** the launcher model
(which writes it) and the ai provider (which reads it), mirroring how `History` and `HistoryPath`
are threaded today. It is loaded once at startup, not re-read per lookup.

The store durably holds whatever was on the clipboard, in plaintext — passwords, tokens, private
drafts included. This is accepted rather than mitigated with encryption, but it is not left without
an escape hatch: `ctrl+x` on an AI row deletes that run from the store, reusing the key that already
means "forget this" for history rows.

### Surfacing runs in recent items

**AI runs are not written to launcher history.** History is capped at 30 and dedups by
`(ActionType, Target)`; every run would be a unique target, so nothing would dedup and heavy helper
use would evict every app, script and calc row. ADR 0006 frames history as a disposable convenience
cache, and it stays that.

Instead:

- A new action type, `ActionAICopy`, is **appended** to the `ActionType` iota. Appending keeps
  `history.ActionTypeCopy == launcher.ActionCopy == 0` valid, so the existing hand-maintained mirror
  is not made worse.
- The empty-query branch of the launcher model, which today maps history entries to results, is
  extended to place **the 5 newest successful runs above** the history rows. The AI rows come from
  the ai provider reading the runs store; the history rows are unchanged.
- An AI row's title is the run's input truncated to ~60 characters; its subtitle is the first line
  of the response; icons distinguish `ai` from `improve`.
- Failed runs are excluded from the rows (they remain in the store).
- `Enter` on an AI row copies the stored response and resets/hides — the same synchronous path as
  `ActionCopy`, so it cannot strand the launcher the way an async `ActionCommand` dispatch would.
- `ctrl+x` on an AI row deletes the run from the store and refreshes the list; today's `ctrl+x`
  path, which requires an `ActionRecall` row with a non-nil history entry, gains this second case.

`ActionCommand` dispatch is left exactly as it is, resolving through `CommandsProvider`. No
cross-provider command-lookup interface is needed — that requirement existed only to support routing
`ai:<id>` targets through history, which this design removes.

The ai provider's `Query` contributes nothing for a non-empty query: `ai` and `improve` come from
the commands provider, and searching past runs is out of scope. Its job is supplying the AI rows for
the empty-query list, resolving a row to its stored response, and deleting a run.

### New injected surface

The mode needs to *read* the clipboard, which nothing injects today: `platform.CopyText` and
`platform.ShowNotification` are already threaded into the launcher, but `platform.ReadClipboardText`
is used only by `internal/cleanurl`, and neither `cmd`'s dependency struct nor the launcher's model
config has a read hook. Both gain one. Note `atotto/clipboard` shells out to `pbpaste` on macOS, so
the read is a subprocess call — it happens once, at mode entry, not per keystroke.

## Testing Decisions

A good test here asserts external behaviour: what argv and stdin the subprocess receives, what ends
up in the store, what the clipboard holds, what the model's chrome and dispatched commands are after
a key sequence. It does not assert on private helpers or on rendering minutiae. Table-driven, in a
`_test.go` beside the code, per repo convention.

**Seam 1 — `internal/launcher/ai`** (new, pure). Prior art:
`internal/launcher/history/history_test.go` and `internal/launcher/scripts/scripts_test.go`.

- prompt construction: `ai` passes input through unchanged; `improve` wraps it in the instruction and
  the text tags, and a text containing tag-like markup is still delimited correctly
- invocation, with a fake exec func: correct argv including the configured model and every hardening
  flag; the prompt arrives on stdin and never in argv; a large input is unaffected
- failure paths: non-zero exit surfaces stderr; a missing binary surfaces the exec error; a deadline
  produces the distinct timeout status
- response trimming: surrounding whitespace removed, an embedded code fence left alone
- store: append/load round-trip, most-recent-first ordering, whole-file rewrite, cap trimming at 200,
  delete-by-id, and a corrupt or missing file loading as empty rather than erroring

**Seam 2 — `internal/launcher` model and provider,** via the existing `model_test.go` and the
`*provider_test.go` family. Prior art for the mode half is `internal/beads/model_test.go`, which
already tests create/status mode flips against the same fuzzy finder widget.

- picking `ai` / `improve` emits the enter-mode message; handling it sets the prompt, the footer
  legend, and the clipboard snapshot
- the full-viewport preview shows while the input is empty and disappears on the first typed
  character
- navigation keys are swallowed and results are not recomputed while in mode
- Enter sends the snapshot, not a re-read clipboard: a clipboard that changes after mode entry does
  not change what is dispatched
- Esc exits the mode and leaves the launcher visible
- Enter with resolved input dispatches a run and triggers the reset-and-hide path
- Enter with empty input and empty snapshot shows the inline error and stays in the mode; the next
  keystroke clears it
- a background message arriving while in mode does not clobber the footer legend
- the completion message: on success writes the store, copies, and notifies with a multi-line body;
  on failure notifies and leaves the clipboard untouched; in neither case is a history entry written
- the completion message recomputes results when the input is empty and does not when a query is
  typed
- the empty-query list places the 5 newest successful runs above the history rows, excludes failed
  runs, and renders title and subtitle from the stored run
- Enter on an AI row copies the stored response and takes the reset-and-hide path
- `ctrl+x` on an AI row deletes the run from the store; `ctrl+x` on a history row still behaves as
  it does today
- `reload` and `cleanurl` dispatch unchanged

## Out of Scope

- **In-flight visibility.** No spinner, no "running" row, no live state if the launcher is reopened
  mid-request. It would need live refresh in a resident process that has an idle-cost ADR to
  respect, and the store already answers "what did it say?".
- **Re-running from an AI row.** Enter copies; it never re-fires the request.
- **Searching past runs.** AI rows appear only on the empty-query list; they are not fuzzy-matched.
- **A browsing UI for the runs store** beyond those rows.
- **Stripping markdown fences** from responses.
- **Multi-line typed input.** The mode reuses the single-line widget input; long prose arrives via
  the clipboard.
- **Per-run model selection**, prompt templates beyond `improve`, or user-defined AI commands.
- **Conversation state.** Every run is independent; there is no history, no follow-up, no context
  carried between runs.
- **Making the flag list configurable.**
- **Raising launcher history's 30-entry cap.** Deeper recents may be worth doing, but on its own
  merits — it is not a fix for AI runs, which this design keeps out of history entirely.
- **Encrypting or auto-purging the runs store.** `0600` and `ctrl+x` are the whole story.

## Further Notes

- `platform.ShowNotification` and `platform.CopyText` are already injected into the launcher.
  Clipboard *read* is not, and this feature adds it in two places.
- macOS truncates long notification bodies for display. That is accepted: the clipboard and the
  store both hold the untruncated response.
- The runs store's 200-record cap and history's 30-entry cap are independent and never interact,
  since AI runs are not written to history.
- The 5-row figure for AI rows in the empty-query list is a fixed constant, not configurable.
