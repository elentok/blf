# ADR 0009: `blf beads` is a plain view of `bd list`, not a readiness-scoped triage queue

**Status**: Accepted (supersedes the readiness/scope-cycle design from `docs/plans/blf-beads.md`)

## Context

The original `blf beads` design (fully implemented — see `docs/plans/blf-beads.md` task 2/5)
classified every issue as unblocked/blocked/other via a separate `bd ready --json` fetch, sorted
rows into readiness buckets, and offered a `ctrl+f` scope cycle (actionable → ready → blocked →
all). This turned the list into a triage queue rather than a direct view of the project's issues.

On reflection, that model didn't match the actual want: an interactive view of plain `bd list`
output, mirroring the tree the `bd` CLI itself prints. Readiness also isn't derivable from
`bd list --json` alone — it requires a second `bd ready` call and a separate classifier, adding a
fetch, a sort, and a mode the user no longer wants.

## Decision

Drop readiness (unblocked/blocked classification), the readiness-bucketed sort, the `↓N ↑M`
blocker/dependent badge, and the `ctrl+f` scope cycle entirely. `blf beads` now:

- Fetches the working set with one `bd list --json` call (`--all` when `-a`/`--all` is passed),
  mirroring `bd list` / `bd list --all` exactly.
- Reconstructs bd's own tree grouping client-side (group by `parent`, sort top-level and each
  sibling group by id) since `bd list --json` isn't itself tree-ordered.
- Highlights epic rows (bold + distinct color) instead of a readiness glyph.

## Consequences

- One fewer `bd` subprocess per load (no `bd ready` call) and one fewer async fetch to coordinate.
- No more "what's actionable right now" queue — a developer using `blf beads` to find unblocked
  work goes back to reading the tree/status icons, same as plain `bd list`.
- `internal/beads`'s `Readiness` type, `ClassifyReadiness`, `SortIssues`, and `Scope` (as an
  actionable/ready/blocked/all enum) are removed; `List` takes a plain `all bool`.
