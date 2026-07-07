// Package beads is the sole code in blf aware of the external `bd` (Beads)
// CLI. It shells out to `bd ... --json`, decodes the result into typed
// values, and never reads the Beads/dolt database directly.
package beads

import "time"

// Issue is a single Beads issue as decoded from `bd ... --json`.
type Issue struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	Priority        int        `json:"priority"`
	IssueType       string     `json:"issue_type"`
	Labels          []string   `json:"labels"`
	Parent          string     `json:"parent"`
	DependencyCount int        `json:"dependency_count"`
	DependentCount  int        `json:"dependent_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at"`
	ClosedAt        *time.Time `json:"closed_at"`
}

// Dependency is one row of `bd dep list`/`bd show`'s dependency edges: the
// related issue plus the edge type (e.g. "blocks", "parent-child").
type Dependency struct {
	Issue
	DependencyType string `json:"dependency_type"`
}
