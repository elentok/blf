package beads

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Adapter is the read/write front-end over the bd CLI. Dir, if set, is
// threaded as `bd -C <dir>` so the adapter targets a project other than the
// working directory.
type Adapter struct {
	Runner Runner
	Dir    string

	// mu serializes bd invocations: bd's underlying db contends under
	// concurrent access, so two bd processes racing each other end up
	// slower (and much noisier) than just running them back to back. The
	// TUI fires bd calls from several independent goroutines (initial
	// load, preview fetch, mutations), so this is enforced centrally here
	// rather than at each call site.
	mu sync.Mutex
}

// New returns an Adapter that shells out to the real bd binary, optionally
// scoped to dir via -C.
func New(dir string) *Adapter {
	return &Adapter{Runner: execRunner{}, Dir: dir}
}

// run threads the adapter's -C dir (if any) in front of args and executes.
func (a *Adapter) run(args ...string) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	full := args
	if a.Dir != "" {
		full = append([]string{"-C", a.Dir}, args...)
	}
	return a.Runner.Run(full)
}

// List returns the issues decoded from `bd list --json`, mirroring bd
// list's own default filter (closed issues excluded) unless all is true, in
// which case it mirrors `bd list --all` (closed included). Bd's
// unavailability or a missing database surfaces as this call's own error
// (bd's stderr is descriptive on both counts), so callers don't need a
// separate up-front check.
func (a *Adapter) List(all bool) ([]Issue, error) {
	// bd list defaults to --limit 50; --limit 0 disables that cap so a
	// large project's issue list doesn't get silently truncated.
	args := []string{"list", "--json", "--limit", "0"}
	if all {
		args = append(args, "--all")
	}

	out, err := a.run(args...)
	if err != nil {
		return nil, err
	}
	return decodeIssues(out)
}

// Show returns the full detail of a single issue via `bd show <id> --json`.
func (a *Adapter) Show(id string) (Issue, error) {
	out, err := a.run("show", id, "--json")
	if err != nil {
		return Issue{}, err
	}
	issues, err := decodeIssues(out)
	if err != nil {
		return Issue{}, err
	}
	if len(issues) == 0 {
		return Issue{}, fmt.Errorf("beads: issue %q not found", id)
	}
	return issues[0], nil
}

// DepDirection selects which way DepTree walks from its root: DepDown
// (blockers/ancestors) or DepUp (dependents/children).
type DepDirection string

const (
	// DepDown walks toward what root depends on (its blockers), plus the
	// parent-child edge to root's own parent.
	DepDown DepDirection = "down"
	// DepUp walks toward what depends on root: its subtasks (parent-child)
	// and anything it blocks.
	DepUp DepDirection = "up"
)

// DepTree returns the full recursive dependency tree rooted at id in a
// single `bd dep tree` invocation, via `bd dep tree <id> --json
// --show-all-paths` (--show-all-paths keeps every edge into a
// diamond-shared or cyclic node instead of bd's default dedup, so
// BuildSubtasksTree/BuildBlockedByTree can still detect and collapse those
// themselves). One call replaces what would otherwise be one bd subprocess
// spawn per node in the tree.
func (a *Adapter) DepTree(id string, direction DepDirection) ([]DepTreeNode, error) {
	args := []string{"dep", "tree", id, "--json", "--show-all-paths"}
	if direction == DepUp {
		args = append(args, "--direction", "up")
	}
	out, err := a.run(args...)
	if err != nil {
		return nil, err
	}
	var nodes []DepTreeNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		return nil, fmt.Errorf("beads: decoding dep tree for %q: %w", id, err)
	}
	return nodes, nil
}

func decodeIssues(data []byte) ([]Issue, error) {
	var issues []Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("beads: decoding issues: %w", err)
	}
	return issues, nil
}

// decodeIssueList decodes a `bd <cmd> --json` result of the id-list form
// (update/close/reopen: a JSON array, even for a single id) into its sole
// issue.
func decodeIssueList(data []byte, id string) (Issue, error) {
	issues, err := decodeIssues(data)
	if err != nil {
		return Issue{}, err
	}
	if len(issues) == 0 {
		return Issue{}, fmt.Errorf("beads: no issue returned for %q", id)
	}
	return issues[0], nil
}

// CreateOptions holds the optional `bd create` flags the TUI's create mode
// sets.
type CreateOptions struct {
	Parent   string // --parent; empty omits the flag
	Priority string // --priority; empty omits the flag
	Type     string // --type; empty omits the flag
}

// Create creates a new issue via `bd create <title> --json`, returning the
// created issue. Unlike List/Show/UpdateStatus/Close/Reopen, `bd create
// --json` emits a single JSON object rather than an array.
func (a *Adapter) Create(title string, opts CreateOptions) (Issue, error) {
	args := []string{"create", title, "--json"}
	if opts.Parent != "" {
		args = append(args, "--parent", opts.Parent)
	}
	if opts.Priority != "" {
		args = append(args, "--priority", opts.Priority)
	}
	if opts.Type != "" {
		args = append(args, "--type", opts.Type)
	}

	out, err := a.run(args...)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, fmt.Errorf("beads: decoding created issue: %w", err)
	}
	return issue, nil
}

// StatusChoices are the workflow statuses offered by the TUI's status-pick
// mode. bd also accepts "pinned"/"hooked", but those are operational-state
// markers rather than workflow steps, so they're left to the bd CLI directly
// per CONTEXT.md/the PRD's "core status only" scope.
var StatusChoices = []string{"open", "in_progress", "blocked", "deferred", "closed"}

// UpdateStatus sets id's status via `bd update <id> --status <status> --json`.
func (a *Adapter) UpdateStatus(id, status string) (Issue, error) {
	out, err := a.run("update", id, "--status", status, "--json")
	if err != nil {
		return Issue{}, err
	}
	return decodeIssueList(out, id)
}

// Close closes id via `bd close <id> --json`.
func (a *Adapter) Close(id string) (Issue, error) {
	out, err := a.run("close", id, "--json")
	if err != nil {
		return Issue{}, err
	}
	return decodeIssueList(out, id)
}

// Reopen reopens id via `bd reopen <id> --json`.
func (a *Adapter) Reopen(id string) (Issue, error) {
	out, err := a.run("reopen", id, "--json")
	if err != nil {
		return Issue{}, err
	}
	return decodeIssueList(out, id)
}

// GraphFormat selects bd graph's output format.
type GraphFormat string

const (
	// GraphCompact is the tree format, one line per issue, meant for paging
	// in the terminal.
	GraphCompact GraphFormat = "compact"
	// GraphHTML is self-contained interactive HTML, meant to be written to a
	// file and opened in a browser.
	GraphHTML GraphFormat = "html"
)

// Graph returns id's dependency graph rendered in format via `bd graph`,
// e.g. for the TUI's ctrl+g shell-out. The output is raw text/HTML, not
// JSON: bd graph doesn't support --json.
func (a *Adapter) Graph(id string, format GraphFormat) ([]byte, error) {
	args := []string{"graph", id}
	switch format {
	case GraphCompact, "":
		args = append(args, "--compact")
	case GraphHTML:
		args = append(args, "--html")
	default:
		return nil, fmt.Errorf("beads: unknown graph format %q", format)
	}
	return a.run(args...)
}
