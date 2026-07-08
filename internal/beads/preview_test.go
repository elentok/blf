package beads

import "testing"

// fakePreviewFetcher is a fake PreviewFetcher driven by canned per-direction
// dep tree responses, so FetchSubtasksTree/FetchBlockedByTree tests don't
// shell out to bd.
type fakePreviewFetcher struct {
	downOf map[string][]DepTreeNode
	upOf   map[string][]DepTreeNode
}

func (f fakePreviewFetcher) DepTree(id string, direction DepDirection) ([]DepTreeNode, error) {
	if direction == DepUp {
		return f.upOf[id], nil
	}
	return f.downOf[id], nil
}

func TestFetchBlockedByTree_FiltersOutParentChildEdges(t *testing.T) {
	// bd dep tree mixes every edge type touching the tree, including the
	// "parent-child" hierarchy edge to the issue's own epic. That must not
	// leak into the blocked-by tree, which is dependency edges only.
	root := Issue{ID: "blf-bnt.5"}
	fetcher := fakePreviewFetcher{
		downOf: map[string][]DepTreeNode{
			"blf-bnt.5": {
				{Issue: Issue{ID: "blf-bnt.5"}, Depth: 0},
				{Issue: Issue{ID: "blf-bnt.4"}, Depth: 1, ParentID: "blf-bnt.5", EdgeFromParent: "blocks"},
				{Issue: Issue{ID: "blf-bnt"}, Depth: 1, ParentID: "blf-bnt.5", EdgeFromParent: "parent-child"},
			},
		},
	}

	tree, err := FetchBlockedByTree(fetcher, root)
	if err != nil {
		t.Fatalf("FetchBlockedByTree: unexpected error: %v", err)
	}

	if len(tree.Children) != 1 || tree.Children[0].Issue.ID != "blf-bnt.4" {
		t.Fatalf("expected only the 'blocks' edge in the tree, got %+v", tree.Children)
	}
}

func TestFetchSubtasksTree_RecursesAllLevels(t *testing.T) {
	root := Issue{ID: "epic", IssueType: "epic"}
	fetcher := fakePreviewFetcher{
		upOf: map[string][]DepTreeNode{
			"epic": {
				{Issue: Issue{ID: "epic"}, Depth: 0},
				{Issue: Issue{ID: "epic.1"}, Depth: 1, ParentID: "epic", EdgeFromParent: "parent-child"},
				{Issue: Issue{ID: "epic.2"}, Depth: 1, ParentID: "epic", EdgeFromParent: "parent-child"},
				{Issue: Issue{ID: "epic.2.1"}, Depth: 2, ParentID: "epic.2", EdgeFromParent: "parent-child"},
			},
		},
	}

	tree, err := FetchSubtasksTree(fetcher, root)
	if err != nil {
		t.Fatalf("FetchSubtasksTree: unexpected error: %v", err)
	}

	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 direct children, got %d", len(tree.Children))
	}
	if len(tree.Children[1].Children) != 1 || tree.Children[1].Children[0].Issue.ID != "epic.2.1" {
		t.Fatalf("expected epic.2 to nest epic.2.1, got %+v", tree.Children[1].Children)
	}
}

func TestFetchBlockedByTree_StopsAtCyclesWithoutInfiniteFetching(t *testing.T) {
	// With --show-all-paths, bd itself would keep emitting nodes around a
	// cycle up to its own max-depth safety limit; BuildBlockedByTree's own
	// seen-tracking must still collapse the repeat back to a back-reference
	// rather than trusting bd to have stopped exactly at one repeat.
	root := Issue{ID: "a"}
	fetcher := fakePreviewFetcher{
		downOf: map[string][]DepTreeNode{
			"a": {
				{Issue: Issue{ID: "a"}, Depth: 0},
				{Issue: Issue{ID: "b"}, Depth: 1, ParentID: "a", EdgeFromParent: "blocks"},
				{Issue: Issue{ID: "a"}, Depth: 2, ParentID: "b", EdgeFromParent: "blocks"},
			},
		},
	}

	tree, err := FetchBlockedByTree(fetcher, root)
	if err != nil {
		t.Fatalf("FetchBlockedByTree: unexpected error: %v", err)
	}
	if len(tree.Children) != 1 || tree.Children[0].Issue.ID != "b" {
		t.Fatalf("expected root to expand 'b', got %+v", tree.Children)
	}
	back := tree.Children[0].Children
	if len(back) != 1 || back[0].Issue.ID != "a" || !back[0].IsBackRef {
		t.Fatalf("expected 'b' to collapse back to 'a' as a back-reference, got %+v", back)
	}
}
