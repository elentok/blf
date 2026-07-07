package beads

import (
	"testing"
	"time"
)

func TestClassifyReadiness(t *testing.T) {
	ready := map[string]bool{"abc-1": true}

	tests := []struct {
		name  string
		issue Issue
		want  Readiness
	}{
		{
			name:  "in ready set is unblocked regardless of status",
			issue: Issue{ID: "abc-1", Status: "open", DependencyCount: 3},
			want:  Unblocked,
		},
		{
			name:  "open with blockers not in ready set is blocked",
			issue: Issue{ID: "abc-2", Status: "open", DependencyCount: 1},
			want:  Blocked,
		},
		{
			name:  "blocked status with blockers not in ready set is blocked",
			issue: Issue{ID: "abc-3", Status: "blocked", DependencyCount: 2},
			want:  Blocked,
		},
		{
			name:  "open with no blockers and not in ready set is other",
			issue: Issue{ID: "abc-4", Status: "open", DependencyCount: 0},
			want:  Other,
		},
		{
			name:  "closed issue is other even with blockers",
			issue: Issue{ID: "abc-5", Status: "closed", DependencyCount: 1},
			want:  Other,
		},
		{
			name:  "deferred issue is other",
			issue: Issue{ID: "abc-6", Status: "deferred", DependencyCount: 1},
			want:  Other,
		},
		{
			// A closed blocker still counts toward DependencyCount, but
			// readiness must come from ready-set membership, not the count.
			name:  "in ready set despite nonzero dependency count from a closed blocker",
			issue: Issue{ID: "abc-1", Status: "blocked", DependencyCount: 1},
			want:  Unblocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyReadiness(tt.issue, ready)
			if got != tt.want {
				t.Errorf("ClassifyReadiness(%+v) = %v, want %v", tt.issue, got, tt.want)
			}
		})
	}
}

func TestSortIssuesBucketOrdering(t *testing.T) {
	ready := map[string]bool{"unblocked-1": true}

	issues := []Issue{
		{ID: "other-1", Status: "closed"},
		{ID: "blocked-1", Status: "open", DependencyCount: 1},
		{ID: "unblocked-1", Status: "open", DependencyCount: 1},
	}

	SortIssues(issues, ready)

	want := []string{"unblocked-1", "blocked-1", "other-1"}
	for i, id := range want {
		if issues[i].ID != id {
			t.Fatalf("SortIssues order = %v, want %v", issueIDs(issues), want)
		}
	}
}

func TestSortIssuesPriorityWithinBucket(t *testing.T) {
	ready := map[string]bool{}

	issues := []Issue{
		{ID: "low-pri", Status: "open", DependencyCount: 1, Priority: 3},
		{ID: "high-pri", Status: "open", DependencyCount: 1, Priority: 0},
		{ID: "mid-pri", Status: "open", DependencyCount: 1, Priority: 1},
	}

	SortIssues(issues, ready)

	want := []string{"high-pri", "mid-pri", "low-pri"}
	for i, id := range want {
		if issues[i].ID != id {
			t.Fatalf("SortIssues order = %v, want %v", issueIDs(issues), want)
		}
	}
}

func TestSortIssuesUpdatedAtWithinPriority(t *testing.T) {
	ready := map[string]bool{}
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	issues := []Issue{
		{ID: "older", Status: "open", DependencyCount: 1, Priority: 1, UpdatedAt: older},
		{ID: "newer", Status: "open", DependencyCount: 1, Priority: 1, UpdatedAt: newer},
	}

	SortIssues(issues, ready)

	want := []string{"newer", "older"}
	for i, id := range want {
		if issues[i].ID != id {
			t.Fatalf("SortIssues order = %v, want %v", issueIDs(issues), want)
		}
	}
}

func TestSortIssuesStableOnFullTies(t *testing.T) {
	ready := map[string]bool{}

	issues := []Issue{
		{ID: "first", Status: "in_progress"},
		{ID: "second", Status: "in_progress"},
		{ID: "third", Status: "in_progress"},
	}

	SortIssues(issues, ready)

	want := []string{"first", "second", "third"}
	for i, id := range want {
		if issues[i].ID != id {
			t.Fatalf("SortIssues order = %v, want %v (stability broken)", issueIDs(issues), want)
		}
	}
}

func issueIDs(issues []Issue) []string {
	ids := make([]string, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}
	return ids
}
