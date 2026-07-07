package beads

import "testing"

// fakePreviewFetcher is a fake PreviewFetcher driven by canned per-id
// responses, so FetchSubtasksTree/FetchBlockedByTree tests don't shell out
// to bd.
type fakePreviewFetcher struct {
	childrenOf map[string][]Issue
	depsOf     map[string][]Dependency
}

func (f fakePreviewFetcher) Children(id string) ([]Issue, error) {
	return f.childrenOf[id], nil
}

func (f fakePreviewFetcher) DepList(id string) ([]Dependency, error) {
	return f.depsOf[id], nil
}

func TestFetchBlockedByTree_FiltersOutParentChildEdges(t *testing.T) {
	// bd dep list returns every edge type touching an issue, including the
	// "parent-child" hierarchy edge to the issue's own epic. That must not
	// leak into the blocked-by tree, which is dependency edges only.
	root := Issue{ID: "blf-bnt.5"}
	fetcher := fakePreviewFetcher{
		depsOf: map[string][]Dependency{
			"blf-bnt.5": {
				{Issue: Issue{ID: "blf-bnt.4"}, DependencyType: "blocks"},
				{Issue: Issue{ID: "blf-bnt"}, DependencyType: "parent-child"},
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
		childrenOf: map[string][]Issue{
			"epic":   {{ID: "epic.1"}, {ID: "epic.2"}},
			"epic.2": {{ID: "epic.2.1"}},
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
	root := Issue{ID: "a"}
	fetcher := fakePreviewFetcher{
		depsOf: map[string][]Dependency{
			"a": {{Issue: Issue{ID: "b"}, DependencyType: "blocks"}},
			"b": {{Issue: Issue{ID: "a"}, DependencyType: "blocks"}},
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
