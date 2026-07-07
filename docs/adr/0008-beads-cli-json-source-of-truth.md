# ADR 0008: blf beads shells out to bd JSON instead of reading the Beads database directly

**Status**: Accepted

## Context

`blf beads` needs to browse, preview, and mutate Beads issues for whatever project the user runs it in.
The project already has a stable external interface for that data: the `bd` CLI, including JSON output
for list/show/ready/children/dep/update/create-style operations and the same workspace auto-discovery
users already rely on in the shell.

The alternative would be to read the `.beads` backing store directly, either by talking to Dolt
ourselves or by depending on a Go Beads library. That would avoid process spawns, but it would also
duplicate Beads' own workspace-discovery, status semantics, graph traversal surface, and mutation
contracts inside blf.

## Decision

`blf beads` treats `bd` as the sole source of truth and shells out to it for every Beads read/write.
The adapter layer owns all `bd` argument construction and JSON decoding; higher layers only see typed
issues, readiness sets, and mutation methods.

This applies both to reads (`bd list`, `bd ready`, `bd show`, `bd children`, `bd dep list`) and to
mutations / shell-outs (`bd create`, `bd update --status`, `bd close`, `bd reopen`, `bd edit`,
`bd graph`). `blf` never opens or queries the `.beads` database directly.

## Alternatives considered

- **Read the `.beads` / Dolt store directly**
  Rejected because it would couple blf to Beads' storage shape rather than its supported command
  interface, and would force blf to reimplement project discovery, mutation semantics, and query
  behavior that `bd` already defines.

- **Depend on a Beads Go library**
  Rejected for now because the CLI boundary is already sufficient, keeps the integration narrow, and
  avoids taking on a new internal API contract whose stability is less obvious than the user-facing CLI.

## Consequences

- `blf beads` inherits Beads' own workspace discovery and semantics automatically, including "run here
  and operate on this project's issues" behavior via `bd` and `bd -C`.
- Adapter tests can lock behavior at the CLI boundary by asserting exact argv and representative JSON
  decoding, which is simpler than mocking a database layer.
- Every data fetch or mutation pays a subprocess cost. For this feature that trade-off is acceptable:
  the TUI fetches the working set once per scope, does fuzzy matching client-side, and only re-fetches
  on mutation, manual refresh, or lazy preview loads.
