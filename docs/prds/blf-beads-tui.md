# PRD: `blf beads` — Beads issue browser TUI

> Canonical terms live in CONTEXT.md: **blf beads**, **readiness**, **issue preview**,
> **create mode**. Design was scoped in a /grill session; implementation plan lives at
> `docs/plans/blf-beads.md`.

## Problem Statement

When I'm working in a project tracked with Beads, I want to pick up the next piece of
work and hand it to an AI coding agent. Today that means dropping to the `bd` CLI,
running `bd ready` / `bd list`, reading dependency output to figure out what's actually
unblocked, and copy-pasting an issue id by hand. It's several commands, the blocked/
unblocked picture isn't obvious at a glance, and creating a quick follow-up ticket
pulls me out of flow. I want one fast, project-contextual TUI that shows me what's
actionable, lets me see *why* something is blocked, and gives an issue id to my agent
in a single keystroke.

## Solution

`blf beads` — an interactive TUI over the `bd` CLI. It fuzzy-lists the project's issues
(flat: epics, subtasks, and standalone), with a side-by-side **issue preview** showing
the selected issue's description plus two trees: its **subtasks** (for epics) and its
transitive **blocked-by** dependency chain. Rows are ordered actionable-first using
**readiness** derived from `bd ready`, with a color cue and a `↓N ↑M` blocker/dependent
badge so blocked-ness is scannable. Pressing Enter copies the issue id to the clipboard
(and prints it to stdout) and quits — a picker handoff straight into an agent prompt.
Secondary keys create, re-state, close/reopen, and edit issues, or open the full `bd`
dependency graph, all by shelling out to `bd`. Beads' JSON is the sole source of truth;
blf never reads the beads database directly.

## User Stories

1. As a developer, I want to run `blf beads` in my project and see its issues immediately, so that I can find work without composing `bd` queries.
2. As a developer, I want the list to show epics, subtasks, and standalone issues as flat rows, so that I can fuzzy-find any issue by typing its name.
3. As a developer, I want to type to fuzzy-filter the list instantly, so that I can narrow to an issue without shelling out per keystroke.
4. As a developer, I want unblocked issues sorted above blocked ones (priority within each), so that the work I can start now floats to the top.
5. As a developer, I want a visual cue (color/glyph) on each row for whether it's blocked or unblocked, so that I can scan readiness without reading text.
6. As a developer, I want a compact `↓N ↑M` badge showing how many issues block it and how many it blocks, so that I gauge an issue's entanglement at a glance.
7. As a developer, I want epic rows tagged as epics and subtask rows tagged with their parent, so that I understand each row's role in the hierarchy inline.
8. As a developer, I want a side preview of the selected issue's full description, so that I can read detail without opening an editor.
9. As a developer, I want the preview to show an epic's subtasks as a tree with a completion count, so that I see how much of the epic is done.
10. As a developer, I want the preview to show the transitive "blocked by" chain as a tree, so that I can trace what has to happen before this issue can start.
11. As a developer, I want repeated/cyclic nodes in the blocked-by tree collapsed to a back-reference, so that the tree stays finite and readable on diamond-shaped dependencies.
12. As a developer, I want a subtask's preview to show a one-line breadcrumb to its parent epic, so that I keep context on where it belongs.
13. As a developer, I want the preview to load lazily as I move the cursor (debounced, cached), so that scrolling the list stays fast and doesn't spawn a `bd` call per row.
14. As a developer, I want the preview header (title/status/priority) to appear instantly while the trees fill in, so that the pane never blank-flashes.
15. As a developer, I want to cycle a scope filter (actionable → ready-only → blocked-only → all incl. closed), so that I can focus on exactly the slice I care about.
16. As a developer, I want to press Enter to copy the selected issue id and quit, so that I can immediately paste it into my agent prompt.
17. As a developer, I want the id also printed to stdout on quit, so that `blf beads` composes in shell pipelines (e.g. `bd show $(blf beads)`).
18. As a developer, I want to create a new issue without leaving the TUI, so that I can capture a follow-up in flow.
19. As a developer, I want creation to repurpose the search input as a title field (create mode), so that I don't have to learn a separate form.
20. As a developer, I want creating while an epic is selected to default the new issue as that epic's child (toggleable), so that subtasks land under the right parent by default.
21. As a developer, I want to change an issue's status from the TUI via a quick picker, so that I can triage without switching to the shell.
22. As a developer, I want to close or reopen the selected issue with one key, so that I can mark work done as I go.
23. As a developer, I want to open the selected issue in `$EDITOR` for richer edits, so that I can fill in description/fields beyond a title.
24. As a developer, I want to open the full dependency graph via `bd graph`, so that I can see the whole DAG when the per-issue tree isn't enough.
25. As a developer, I want to refresh the list on demand, so that changes I made in another terminal show up.
26. As a developer, I want the list to re-fetch and keep my just-created/edited issue selected after a mutation, so that I stay oriented.
27. As a developer, I want `blf beads` to operate on the `.beads` database auto-discovered from my working directory, so that it's project-contextual like `bd`.
28. As a developer, I want a `-C/--dir` flag to point at another project, so that I can browse a different repo's issues.
29. As a developer, I want a clear error if `bd` isn't installed or no `.beads` db is found, so that I'm not staring at a broken/empty TUI.
30. As a developer, I want an in-TUI empty state with hints when there are no issues in scope, so that create and filter are still discoverable.
31. As a developer, I want the preview to hide on narrow terminals with a `tab` toggle, so that the tool stays usable in a small window.
32. As a developer, I want visible key hints, so that I can discover the available actions.
33. As a developer, I want `esc` to back out of create/status mode (restoring my search), so that I can cancel a triage action safely.

## Implementation Decisions

**Package layout** — one `internal/beads` package with four separable concerns:

- **CLI adapter** (deep module): the only code aware of `bd`. Exposes typed operations —
  `List(scope)`, `Ready()`, `Show(id)`, `Children(id)`, `DepList(id)`, `Create(title, opts)`,
  `UpdateStatus(id, status)`, `Close(id)`, `Reopen(id)`, `Graph(id, format)` — plus a
  presence/db-discovery check. Builds `bd ... --json` argument vectors and decodes results
  into an `Issue` struct (id, title, description, status, priority, issue_type, labels,
  parent, dependency_count, dependent_count, timestamps). The command runner is injected
  (an exec-runner interface) so tests drive it without a real `bd`. All `bd` invocations pass
  through here; the `-C` directory is threaded as `bd -C <dir>`.
- **Readiness & sort** (pure): a classifier turning the `bd ready` id set + issue list into
  a per-issue readiness (unblocked / blocked / other), and a comparator implementing the
  readiness-bucketed-then-priority-then-updated ordering. No `bd`, no bubbletea.
- **Tree builder** (pure): builds the **subtasks tree** (parent→children, nested, with an
  epic completion count) and the transitive **blocked-by tree** (rooted at the issue,
  expanding the dependency direction), collapsing already-seen nodes and cycles to a
  back-reference marker. Consumes data fetched by the adapter; emits a renderable tree
  structure independent of styling.
- **TUI model** (bubbletea): embeds `internal/fuzzyfinder`, owns the flat list + row
  rendering (status/readiness icon, epic/parent tags, `↓N ↑M` badge), the side-by-side
  layout with narrow-terminal fallback, the lazy/debounced/cached preview fetch, the scope
  filter cycle, and the create/status **mode-flip** states. Follows the existing
  `internal/claudehistory` and `internal/launcher` patterns (consumer sees KeyMsg first,
  forwards the rest to the widget; suspend/exec/return for `$EDITOR` and `bd graph`).

**Data flow** — fetch the working set once per scope (`bd list --json` + `bd ready --json`),
fuzzy-match client-side via `fuzzyfinder.Find`. Re-fetch only on scope-cycle, a mutation, or
manual refresh; no background polling. Preview detail is fetched separately per selected id
(lazy, ~100ms debounce, cached; cache entry invalidated on mutation of that id).

**Readiness** — authoritative from `bd ready` set membership, never inferred from
`dependency_count` (a closed blocker still counts). Blocked = open/blocked-status issue not
in the ready set that has blockers.

**Actions / keymap** — input stays focused; all actions are ctrl-chords; `esc` backs out of a
mode. `enter`=copy id + print stdout + quit; `ctrl+a`=create mode (`--parent` on epic, `ctrl+t`
toggle standalone); `ctrl+s`=status-pick mode → `bd update --status`; `ctrl+x`=close/reopen;
`ctrl+e`=edit in `$EDITOR`; `ctrl+g`=`bd graph`; `ctrl+r`=refresh; `ctrl+f`=scope cycle;
`tab`=toggle preview; nav via up/down + ctrl+k/j + ctrl+p/n.

**Create/status modes** reuse the single fuzzy-finder input line (prompt changes, list stops
filtering) rather than a bespoke multi-field form; richer fields come from the `$EDITOR` edit
handoff. Status-pick offers the issue's valid target statuses.

**Command** — `blf beads`, a single interactive cobra command under the existing `cmd/` tree
(consistent with ADR 0001, cobra), with `-C/--dir`. Fail fast with a one-line error when `bd`
is absent or no `.beads` db is discoverable; otherwise enter the TUI (empty-state line when the
scope has no issues). Use `lipgloss` for all styling (no inline ANSI), per project rules.

## Testing Decisions

Good tests here exercise **external behavior through each module's public interface**, not
internals: given a fake `bd` output, assert the decoded/derived result; given an issue set,
assert ordering; given a dependency shape, assert the produced tree. Avoid asserting private
fields or exact lipgloss byte output.

- **CLI adapter** — inject a fake command runner; assert the exact `bd` argv built for each
  operation (incl. `-C` and scope flags) and that representative JSON fixtures decode into the
  right `Issue` values. Catches `bd` contract drift. Prior art: `cmd/*_test.go` table tests and
  the exec-indirection style already used around `claude`/editor handoffs.
- **Readiness & sort** — table tests: readiness classification from a ready-set, and comparator
  output for mixed readiness/priority/updated inputs (unblocked-before-blocked, priority within).
- **Tree builder** — table tests over crafted dependency/hierarchy graphs, explicitly covering a
  diamond (shared blocker rendered once with a back-reference) and a cycle (broken, not infinite),
  plus epic completion counts. Prior art: `internal/fuzzyfinder/*_test.go` pure-logic tests.
- **TUI model** — lighter bubbletea `Update` tests following `internal/launcher/model_test.go` /
  `internal/fuzzyfinder/model_test.go`: entering/leaving create and status modes, `esc` restoring
  the prior query, Enter yielding the selected id, scope-cycle changing the requested scope. Use a
  stub adapter so no real `bd` runs.

## Out of Scope

- Reading the Beads/dolt database directly — everything goes through the `bd` CLI.
- A "roots only" list mode (hiding subtasks) — deferred unless flat density proves annoying.
- Creating dependency links ("new blocked-by") from the TUI — v1 creates issues only.
- Live polling / auto-refresh — refresh is manual or mutation-driven.
- Rendering the full DAG ourselves — `ctrl+g` shells out to `bd graph`.
- Non-interactive subcommands (`blf beads list`, etc.) — management lives in the TUI.
- Custom operational-state dimensions (`bd set-state` patrol/mode/health) — core status only.

## Further Notes

- Mirror the suspend/exec/return handoff `internal/claudehistory` uses for `$EDITOR`/`claude
  --resume` when shelling to `$EDITOR` and `bd graph`.
- The clipboard write should reuse blf's existing cross-platform clipboard path (as `blf copy`
  uses), not a macOS-only call.
- A possible ADR — "shell out to `bd` (JSON) as source of truth vs. reading the dolt db / a Go
  beads lib" — is noted in the plan; write it once the CLI boundary is proven in implementation.
