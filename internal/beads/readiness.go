package beads

import "sort"

// Readiness classifies an issue's actionability. It must be derived
// authoritatively from bd ready set membership, never inferred from
// DependencyCount — a closed blocker still counts toward that count. See
// CONTEXT.md's "readiness" entry.
type Readiness int

const (
	// Unblocked issues are members of the bd ready set: no active blockers.
	Unblocked Readiness = iota
	// Blocked issues are open/blocked-status, missing from the ready set, and
	// have at least one blocker.
	Blocked
	// Other covers everything else (closed, deferred, or any other status not
	// captured above).
	Other
)

// ClassifyReadiness derives issue's Readiness from readyIDs (the bd ready
// --json id set) and, for the blocked case, its status and blocker count.
func ClassifyReadiness(issue Issue, readyIDs map[string]bool) Readiness {
	if readyIDs[issue.ID] {
		return Unblocked
	}
	if (issue.Status == "open" || issue.Status == "blocked") && issue.DependencyCount > 0 {
		return Blocked
	}
	return Other
}

// SortIssues orders issues in place by readiness bucket (unblocked -> blocked
// -> other), then priority ascending (0 = highest per bd's convention), then
// most-recently-updated first.
func SortIssues(issues []Issue, readyIDs map[string]bool) {
	sort.SliceStable(issues, func(i, j int) bool {
		return issueLess(issues[i], issues[j], readyIDs)
	})
}

func issueLess(a, b Issue, readyIDs map[string]bool) bool {
	ra, rb := ClassifyReadiness(a, readyIDs), ClassifyReadiness(b, readyIDs)
	if ra != rb {
		return ra < rb
	}
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	return a.UpdatedAt.After(b.UpdatedAt)
}
