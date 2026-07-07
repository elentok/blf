package beads

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoDatabase is returned when no .beads database is discoverable from the
// adapter's working directory.
var ErrNoDatabase = errors.New("no beads database found")

// Scope selects which issues List returns.
type Scope string

const (
	// ScopeActionable is bd list's default: open issues, closed excluded.
	ScopeActionable Scope = "actionable"
	// ScopeReady restricts to issues with no active blockers.
	ScopeReady Scope = "ready"
	// ScopeBlocked restricts to issues with the "blocked" status.
	ScopeBlocked Scope = "blocked"
	// ScopeAll includes every issue, closed included.
	ScopeAll Scope = "all"
)

// Adapter is the read/write front-end over the bd CLI. Dir, if set, is
// threaded as `bd -C <dir>` so the adapter targets a project other than the
// working directory.
type Adapter struct {
	Runner Runner
	Dir    string
}

// New returns an Adapter that shells out to the real bd binary, optionally
// scoped to dir via -C.
func New(dir string) *Adapter {
	return &Adapter{Runner: execRunner{}, Dir: dir}
}

// run threads the adapter's -C dir (if any) in front of args and executes.
func (a *Adapter) run(args ...string) ([]byte, error) {
	full := args
	if a.Dir != "" {
		full = append([]string{"-C", a.Dir}, args...)
	}
	return a.Runner.Run(full)
}

// Check confirms bd is installed and a .beads database is discoverable,
// returning a clear error otherwise.
func (a *Adapter) Check() error {
	if _, err := a.run("where"); err != nil {
		if errors.Is(err, ErrBdNotFound) {
			return err
		}
		return fmt.Errorf("%w: %s", ErrNoDatabase, err)
	}
	return nil
}

// List returns the issues in scope, decoded from `bd list --json`.
func (a *Adapter) List(scope Scope) ([]Issue, error) {
	args := []string{"list", "--json"}
	switch scope {
	case ScopeActionable, "":
		// bd list's default already excludes closed issues.
	case ScopeReady:
		args = append(args, "--ready")
	case ScopeBlocked:
		args = append(args, "--status", "blocked")
	case ScopeAll:
		args = append(args, "--all")
	default:
		return nil, fmt.Errorf("beads: unknown scope %q", scope)
	}

	out, err := a.run(args...)
	if err != nil {
		return nil, err
	}
	return decodeIssues(out)
}

// Ready returns the set of issue ids with no active blockers, per
// `bd ready --json`. Readiness must come from this set membership, never
// from DependencyCount (a closed blocker still counts toward the count).
func (a *Adapter) Ready() (map[string]bool, error) {
	out, err := a.run("ready", "--json")
	if err != nil {
		return nil, err
	}
	issues, err := decodeIssues(out)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(issues))
	for _, issue := range issues {
		set[issue.ID] = true
	}
	return set, nil
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

// Children returns id's child issues via `bd children <id> --json`.
func (a *Adapter) Children(id string) ([]Issue, error) {
	out, err := a.run("children", id, "--json")
	if err != nil {
		return nil, err
	}
	return decodeIssues(out)
}

// DepList returns what id depends on (its blockers) via
// `bd dep list <id> --json`.
func (a *Adapter) DepList(id string) ([]Dependency, error) {
	out, err := a.run("dep", "list", id, "--json")
	if err != nil {
		return nil, err
	}
	var deps []Dependency
	if err := json.Unmarshal(out, &deps); err != nil {
		return nil, fmt.Errorf("beads: decoding dep list for %q: %w", id, err)
	}
	return deps, nil
}

func decodeIssues(data []byte) ([]Issue, error) {
	var issues []Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, fmt.Errorf("beads: decoding issues: %w", err)
	}
	return issues, nil
}
