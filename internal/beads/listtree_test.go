package beads

import "testing"

func rowIDs(rows []TreeRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.Issue.ID
	}
	return ids
}

func mustEqualIDs(t *testing.T, got []TreeRow, want []string) {
	t.Helper()
	gotIDs := rowIDs(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("row ids = %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("row ids = %v, want %v", gotIDs, want)
		}
	}
}

func TestBuildIssueTree_GroupsByParentAndSortsByID(t *testing.T) {
	issues := []Issue{
		{ID: "epic-b", IssueType: "epic"},
		{ID: "epic-a", IssueType: "epic"},
		{ID: "epic-a.2", Parent: "epic-a"},
		{ID: "epic-a.1", Parent: "epic-a"},
		{ID: "standalone-1"},
	}

	rows := BuildIssueTree(issues, nil)

	mustEqualIDs(t, rows, []string{"epic-a", "epic-a.1", "epic-a.2", "epic-b", "standalone-1"})
	if rows[0].Depth != 0 || rows[1].Depth != 1 || rows[2].Depth != 1 {
		t.Fatalf("unexpected depths: %+v", rows)
	}
}

func TestBuildIssueTree_OrphanParentTreatedAsTopLevel(t *testing.T) {
	issues := []Issue{
		{ID: "child-1", Parent: "missing-parent"},
	}

	rows := BuildIssueTree(issues, nil)

	mustEqualIDs(t, rows, []string{"child-1"})
	if rows[0].Depth != 0 {
		t.Fatalf("expected orphan to render at depth 0, got %+v", rows[0])
	}
}

func TestBuildIssueTree_NilMatchIDsIncludesEverythingUndimmed(t *testing.T) {
	issues := []Issue{
		{ID: "epic-a", IssueType: "epic"},
		{ID: "epic-a.1", Parent: "epic-a"},
	}

	rows := BuildIssueTree(issues, nil)

	for _, r := range rows {
		if r.Dimmed {
			t.Fatalf("expected no dimmed rows with nil matchIDs, got %+v", r)
		}
	}
}

func TestBuildIssueTree_PrunesNonMatchingSubtrees(t *testing.T) {
	issues := []Issue{
		{ID: "epic-a", IssueType: "epic"},
		{ID: "epic-a.1", Parent: "epic-a"},
		{ID: "epic-b", IssueType: "epic"},
		{ID: "epic-b.1", Parent: "epic-b"},
	}

	rows := BuildIssueTree(issues, map[string]bool{"epic-a.1": true})

	mustEqualIDs(t, rows, []string{"epic-a", "epic-a.1"})
}

func TestBuildIssueTree_NonMatchingAncestorIsDimmed(t *testing.T) {
	issues := []Issue{
		{ID: "epic-a", IssueType: "epic"},
		{ID: "epic-a.1", Parent: "epic-a"},
	}

	rows := BuildIssueTree(issues, map[string]bool{"epic-a.1": true})

	mustEqualIDs(t, rows, []string{"epic-a", "epic-a.1"})
	if !rows[0].Dimmed {
		t.Errorf("expected non-matching parent epic-a to be dimmed, got %+v", rows[0])
	}
	if rows[1].Dimmed {
		t.Errorf("expected matching child epic-a.1 to not be dimmed, got %+v", rows[1])
	}
}

func TestBuildIssueTree_MatchingAncestorKeepsNonMatchingChildrenOut(t *testing.T) {
	issues := []Issue{
		{ID: "epic-a", IssueType: "epic"},
		{ID: "epic-a.1", Parent: "epic-a"},
		{ID: "epic-a.2", Parent: "epic-a"},
	}

	// Only the epic itself matches; neither child matches and neither has
	// a matching descendant, so both subtrees are pruned even though the
	// parent is kept.
	rows := BuildIssueTree(issues, map[string]bool{"epic-a": true})

	mustEqualIDs(t, rows, []string{"epic-a"})
}
