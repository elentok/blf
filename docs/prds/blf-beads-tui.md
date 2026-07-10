# PRD: `blf beads` — Beads issue browser TUI

> Canonical terms live in CONTEXT.md: **blf beads**, **issue tree**, **issue preview**,
> **create mode**. Design was scoped in a /grill session; implementation plan lives at
> `docs/plans/blf-beads.md`. An earlier readiness/scope-cycle design (unblocked/blocked
> sort, `ctrl+f` scope filter) was reversed — see ADR 0009.

## Problem Statement

When I'm working in a project tracked with Beads, I want to pick up the next piece of
work and hand it to an AI coding agent. Today that means dropping to the `bd` CLI,
running `bd list`, and copy-pasting an issue id by hand. It's several commands and
creating a quick follow-up ticket pulls me out of flow. I want one fast,
project-contextual TUI that's an interactive view of `bd list` — fuzzy-searchable,
with epics visually obvious — and gives an issue id to my agent in a single keystroke.

## Solution

`blf beads` — an interactive TUI that is a direct view of `bd list` / `bd list --all`
(`-a`/`--all`) over the `bd` CLI. It renders the project's issues as bd's own **issue
tree** (client-side reconstructed grouping by parent, id-sorted, since `bd list --json`
isn't itself tree-ordered), with a side-by-side **issue preview** showing the selected
issue's description plus two trees: its **subtasks** (for epics) and its transitive
**blocked-by** dependency chain. Epic rows render bold in a distinct color so they read
as epics at a glance. Fuzzy-filtering keeps a non-matching ancestor of a match visible
(dimmed) so the tree context isn't lost. Pressing Enter copies the issue id to the
clipboard (and prints it to stdout) and quits — a picker handoff straight into an agent
prompt. Secondary keys create, re-state, close/reopen, and edit issues, or open the full
`bd` dependency graph, all by shelling out to `bd`. Beads' JSON is the sole source of
truth; blf never reads the beads database directly.

## User Stories

1. As a developer, I want to run `blf beads` in my project and see its issues immediately, so that I can find work without composing `bd` queries.
2. As a developer, I want the list to show epics and their subtasks nested the way `bd list` prints them in the terminal, so that the hierarchy is visible at a glance.
3. As a developer, I want to type to fuzzy-filter the list instantly, so that I can narrow to an issue without shelling out per keystroke.
4. As a developer, I want a matching descendant's non-matching ancestor to stay visible (dimmed) while filtering, so that I don't lose track of which epic a matched subtask belongs to.
5. As a developer, I want epic rows to render bold in a distinct color, so that I can tell epics apart from tasks/subtasks at a glance.
6. As a developer, I want a side preview of the selected issue's full description, so that I can read detail without opening an editor.
7. As a developer, I want the preview to show an epic's subtasks as a tree with a completion count, so that I see how much of the epic is done.
8. As a developer, I want the preview to show the transitive "blocked by" chain as a tree, so that I can trace what has to happen before this issue can start.
9. As a developer, I want repeated/cyclic nodes in the blocked-by tree collapsed to a back-reference, so that the tree stays finite and readable on diamond-shaped dependencies.
10. As a developer, I want a subtask's preview to show a one-line breadcrumb to its parent epic, so that I keep context on where it belongs.
11. As a developer, I want the preview to load lazily as I move the cursor (debounced, cached), so that scrolling the list stays fast and doesn't spawn a `bd` call per row.
12. As a developer, I want the preview header (title/status/priority) to appear instantly while the trees fill in, so that the pane never blank-flashes.
13. As a developer, I want a `-a`/`--all` flag mirroring `bd list --all`, so that I can include closed issues when I need to.
14. As a developer, I want to press Enter to copy the selected issue id and quit, so that I can immediately paste it into my agent prompt.
15. As a developer, I want the id also printed to stdout on quit, so that `blf beads` composes in shell pipelines (e.g. `bd show $(blf beads)`).
16. As a developer, I want to create a new issue without leaving the TUI, so that I can capture a follow-up in flow.
17. As a developer, I want creation to repurpose the search input as a title field (create mode), so that I don't have to learn a separate form.
18. As a developer, I want creating while an epic is selected to default the new issue as that epic's child (toggleable), so that subtasks land under the right parent by default.
19. As a developer, I want to change an issue's status from the TUI via a quick picker, so that I can triage without switching to the shell.
20. As a developer, I want to close or reopen the selected issue with one key, so that I can mark work done as I go.
21. As a developer, I want to open the selected issue in `$EDITOR` for richer edits, so that I can fill in description/fields beyond a title.
22. As a developer, I want to open the full dependency graph via `bd graph`, so that I can see the whole DAG when the per-issue tree isn't enough.
23. As a developer, I want to refresh the list on demand, so that changes I made in another terminal show up.
24. As a developer, I want the list to re-fetch and keep my just-created/edited issue selected after a mutation, so that I stay oriented.
25. As a developer, I want `blf beads` to operate on the `.beads` database auto-discovered from my working directory, so that it's project-contextual like `bd`.
26. As a developer, I want a `-C/--dir` flag to point at another project, so that I can browse a different repo's issues.
27. As a developer, I want a clear error if `bd` isn't installed or no `.beads` db is found, so that I'm not staring at a broken/empty TUI.
28. As a developer, I want an in-TUI empty state with hints when there are no issues found, so that create is still discoverable.
29. As a developer, I want the preview to hide on narrow terminals with a `tab` toggle, so that the tool stays usable in a small window.
30. As a developer, I want visible key hints, so that I can discover the available actions.
31. As a developer, I want `esc` to back out of create/status mode (restoring my search), so that I can cancel a triage action safely.

## Implementation Decisions

**Package layout** — one `internal/beads` package with four separable concerns:

- **CLI adapter** (deep module): the only code aware of `bd`. Exposes typed operations —
  `List(all)`, `Show(id)`, `Children(id)`, `DepList(id)`, `Create(title, opts)`,
  `UpdateStatus(id, status)`, `Close(id)`, `Reopen(id)`, `Graph(id, format)` — plus a
  presence/db-discovery check. Builds `bd ... --json` argument vectors and decodes results
  into an `Issue` struct (id, title, description, status, priority, issue_type, labels,
  parent, dependency_count, dependent_count, timestamps). The command runner is injected
  (an exec-runner interface) so tests drive it without a real `bd`. All `bd` invocations pass
  through here; the `-C` directory is threaded as `bd -C <dir>`.
- **Issue tree** (pure): groups the flat `bd list --json` array by each issue's `parent`
  field and sorts top-level items and each sibling group by id ascending, reproducing what
  `bd list` prints in the terminal (which is *not* the JSON array's own order). Also applies
  the fuzzy match set to prune non-matching subtrees while keeping non-matching ancestors of
  a match (dimmed). No `bd`, no bubbletea.
- **Tree builder** (pure): builds the **subtasks tree** (parent→children, nested, with an
  epic completion count) and the transitive **blocked-by tree** (rooted at the issue,
  expanding the dependency direction), collapsing already-seen nodes and cycles to a
  back-reference marker. Consumes data fetched by the adapter; emits a renderable tree
  structure independent of styling. (Distinct from the **issue tree** above — see
  CONTEXT.md's "issue tree" entry for why they're not merged.)
- **TUI model** (bubbletea): embeds `internal/fuzzyfinder`, owns the **issue tree** row
  rendering (status icon, indent, epic bold+color style, dimmed-ancestor style), the
  side-by-side layout with narrow-terminal fallback, the lazy/debounced/cached preview
  fetch, and the create/status **mode-flip** states. Follows the existing
  `internal/claudehistory` and `internal/launcher` patterns (consumer sees KeyMsg first,
  forwards the rest to the widget; suspend/exec/return for `$EDITOR` and `bd graph`).

**Data flow** — fetch the working set once via `bd list --json` (`--all` when `-a`/`--all` is
passed), build the **issue tree** client-side, fuzzy-match client-side via `fuzzyfinder.Find`.
Re-fetch only on a mutation or manual refresh; no background polling, no scope cycle. Preview
detail is fetched separately per selected id (lazy, ~100ms debounce, cached; cache entry
invalidated on mutation of that id).

**Actions / keymap** — input stays focused; all actions are ctrl-chords; `esc` backs out of a
mode. `enter`=copy id + print stdout + quit; `ctrl+a`=create mode (`--parent` on epic, `ctrl+t`
toggle standalone); `ctrl+s`=status-pick mode → `bd update --status`; `ctrl+x`=close/reopen;
`ctrl+e`=edit in `$EDITOR`; `ctrl+g`=`bd graph`; `ctrl+r`=refresh; `tab`=toggle preview; nav via
up/down + ctrl+k/j + ctrl+p/n.

**Create/status modes** reuse the single fuzzy-finder input line (prompt changes, list stops
filtering) rather than a bespoke multi-field form; richer fields come from the `$EDITOR` edit
handoff. Status-pick offers the issue's valid target statuses.

**Command** — `blf beads`, a single interactive cobra command under the existing `cmd/` tree
(consistent with ADR 0001, cobra), with `-C/--dir` and `-a/--all`. Fail fast with a one-line
error when `bd` is absent or no `.beads` db is discoverable; otherwise enter the TUI
(empty-state line when nothing is found). Use `lipgloss` for all styling (no inline ANSI), per
project rules.

## Testing Decisions

Good tests here exercise **external behavior through each module's public interface**, not
internals: given a fake `bd` output, assert the decoded/derived result; given an issue set,
assert the built tree; given a dependency shape, assert the produced preview tree. Avoid
asserting private fields or exact lipgloss byte output.

- **CLI adapter** — inject a fake command runner; assert the exact `bd` argv built for each
  operation (incl. `-C` and `--all`) and that representative JSON fixtures decode into the
  right `Issue` values. Catches `bd` contract drift. Prior art: `cmd/*_test.go` table tests and
  the exec-indirection style already used around `claude`/editor handoffs.
- **Issue tree** — table tests: parent grouping and id sort against `bd list`'s own terminal
  output, orphan-parent handling, and fuzzy-match pruning/dimming (matching subtree kept,
  non-matching sibling subtree dropped, non-matching ancestor of a match dimmed not dropped).
- **Tree builder** — table tests over crafted dependency/hierarchy graphs, explicitly covering a
  diamond (shared blocker rendered once with a back-reference) and a cycle (broken, not infinite),
  plus epic completion counts. Prior art: `internal/fuzzyfinder/*_test.go` pure-logic tests.
- **TUI model** — lighter bubbletea `Update` tests following `internal/launcher/model_test.go` /
  `internal/fuzzyfinder/model_test.go`: entering/leaving create and status modes, `esc` restoring
  the prior query, Enter yielding the selected id, epic rows styled bold. Use a stub adapter so
  no real `bd` runs.

## Out of Scope

- Reading the Beads/dolt database directly — everything goes through the `bd` CLI.
- Readiness (unblocked/blocked) scoring, sorting, or a scope-cycle filter — reversed; see ADR 0009.
- Creating dependency links ("new blocked-by") from the TUI — v1 creates issues only.
- Live polling / auto-refresh — refresh is manual or mutation-driven.
- Rendering the full DAG ourselves — `ctrl+g` shells out to `bd graph`.
- Non-interactive subcommands (`blf beads list`, etc.) — management lives in the TUI.
- Custom operational-state dimensions (`bd set-state` patrol/mode/health) — core status only.
- Nav that skips dimmed ancestor rows — they're normal, selectable rows; dimming is visual only.

## Further Notes

- Mirror the suspend/exec/return handoff `internal/claudehistory` uses for `$EDITOR`/`claude
  --resume` when shelling to `$EDITOR` and `bd graph`.
- The clipboard write should reuse blf's existing cross-platform clipboard path (as `blf copy`
  uses), not a macOS-only call.
- A possible ADR — "shell out to `bd` (JSON) as source of truth vs. reading the dolt db / a Go
  beads lib" — is noted in the plan; write it once the CLI boundary is proven in implementation.
