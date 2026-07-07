package beads

import "testing"

func TestBuildSubtasksTree_Nested(t *testing.T) {
	root := Issue{ID: "epic", Title: "the epic", IssueType: "epic"}
	childrenOf := map[string][]Issue{
		"epic": {
			{ID: "epic.1", Title: "first", Status: "closed"},
			{ID: "epic.2", Title: "second", Status: "open"},
		},
		"epic.2": {
			{ID: "epic.2.1", Title: "grandchild", Status: "closed"},
		},
	}

	tree := BuildSubtasksTree(root, childrenOf)

	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 direct children, got %d", len(tree.Children))
	}
	if tree.Children[1].Issue.ID != "epic.2" {
		t.Fatalf("expected second child to be epic.2, got %q", tree.Children[1].Issue.ID)
	}
	if len(tree.Children[1].Children) != 1 || tree.Children[1].Children[0].Issue.ID != "epic.2.1" {
		t.Fatalf("expected epic.2 to nest grandchild epic.2.1, got %+v", tree.Children[1].Children)
	}
}

func TestSubtaskNode_CompletionCount(t *testing.T) {
	root := Issue{ID: "epic", IssueType: "epic"}
	childrenOf := map[string][]Issue{
		"epic": {
			{ID: "epic.1", Status: "closed"},
			{ID: "epic.2", Status: "open"},
			{ID: "epic.3", Status: "closed"},
		},
		"epic.2": {
			{ID: "epic.2.1", Status: "closed"},
			{ID: "epic.2.2", Status: "open"},
		},
	}

	tree := BuildSubtasksTree(root, childrenOf)
	closed, total := tree.CompletionCount()

	if closed != 3 || total != 5 {
		t.Fatalf("CompletionCount() = (%d, %d), want (3, 5)", closed, total)
	}
}

func TestBuildBlockedByTree_Diamond(t *testing.T) {
	// root depends on a and b; both a and b depend on shared. shared should
	// be expanded once and collapsed to a back-reference the second time.
	root := Issue{ID: "root"}
	blockersOf := map[string][]Issue{
		"root":   {{ID: "a"}, {ID: "b"}},
		"a":      {{ID: "shared"}},
		"b":      {{ID: "shared"}},
		"shared": {{ID: "leaf"}},
	}

	tree := BuildBlockedByTree(root, blockersOf)

	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 direct blockers, got %d", len(tree.Children))
	}

	a := tree.Children[0]
	if a.Issue.ID != "a" || a.IsBackRef {
		t.Fatalf("expected first child 'a' expanded, got %+v", a)
	}
	if len(a.Children) != 1 || a.Children[0].Issue.ID != "shared" || a.Children[0].IsBackRef {
		t.Fatalf("expected 'a' to expand 'shared', got %+v", a.Children)
	}
	if len(a.Children[0].Children) != 1 || a.Children[0].Children[0].Issue.ID != "leaf" {
		t.Fatalf("expected 'shared' to expand 'leaf' under 'a', got %+v", a.Children[0].Children)
	}

	b := tree.Children[1]
	if b.Issue.ID != "b" || b.IsBackRef {
		t.Fatalf("expected second child 'b' expanded, got %+v", b)
	}
	if len(b.Children) != 1 || b.Children[0].Issue.ID != "shared" || !b.Children[0].IsBackRef {
		t.Fatalf("expected 'shared' under 'b' to be a back-reference, got %+v", b.Children)
	}
	if len(b.Children[0].Children) != 0 {
		t.Fatalf("expected back-reference node to not re-expand, got %+v", b.Children[0].Children)
	}
}

func TestBuildBlockedByTree_Cycle(t *testing.T) {
	// root -> a -> b -> root (cycle back to the tree's own root).
	root := Issue{ID: "root"}
	blockersOf := map[string][]Issue{
		"root": {{ID: "a"}},
		"a":    {{ID: "b"}},
		"b":    {{ID: "root"}},
	}

	tree := BuildBlockedByTree(root, blockersOf)

	if len(tree.Children) != 1 || tree.Children[0].Issue.ID != "a" {
		t.Fatalf("expected root to expand 'a', got %+v", tree.Children)
	}
	a := tree.Children[0]
	if len(a.Children) != 1 || a.Children[0].Issue.ID != "b" {
		t.Fatalf("expected 'a' to expand 'b', got %+v", a.Children)
	}
	b := a.Children[0]
	if len(b.Children) != 1 || b.Children[0].Issue.ID != "root" || !b.Children[0].IsBackRef {
		t.Fatalf("expected 'b' to collapse back to root as a back-reference, got %+v", b.Children)
	}
	if len(b.Children[0].Children) != 0 {
		t.Fatalf("expected the cycle's back-reference to not recurse further, got %+v", b.Children[0].Children)
	}
}

func TestBuildBlockedByTree_NoBlockers(t *testing.T) {
	root := Issue{ID: "root"}
	tree := BuildBlockedByTree(root, map[string][]Issue{})

	if len(tree.Children) != 0 {
		t.Fatalf("expected no children for a root with no blockers, got %+v", tree.Children)
	}
}
