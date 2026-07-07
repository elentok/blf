package beads

// SubtaskNode is a node in the subtasks tree: the parent->child hierarchy
// rooted at an issue (typically an epic), nested arbitrarily deep.
type SubtaskNode struct {
	Issue    Issue
	Children []SubtaskNode
}

// BuildSubtasksTree builds the nested subtasks tree rooted at root, using
// childrenOf (issue id -> its direct children, already fetched by the caller
// via bd children for every id reachable from root) to recursively nest.
// The parent-child hierarchy is a tree in bd's data model, so no cycle
// handling is needed here (unlike BuildBlockedByTree).
func BuildSubtasksTree(root Issue, childrenOf map[string][]Issue) SubtaskNode {
	node := SubtaskNode{Issue: root}
	for _, child := range childrenOf[root.ID] {
		node.Children = append(node.Children, BuildSubtasksTree(child, childrenOf))
	}
	return node
}

// CompletionCount returns the (closed, total) counts of every descendant of
// n, excluding n itself, for rendering an epic's "x/y done" summary.
func (n SubtaskNode) CompletionCount() (closed, total int) {
	for _, c := range n.Children {
		total++
		if c.Issue.Status == "closed" {
			closed++
		}
		cClosed, cTotal := c.CompletionCount()
		closed += cClosed
		total += cTotal
	}
	return closed, total
}

// BlockedByNode is a node in the transitive "blocked by" dependency tree,
// rooted at an issue and expanded in the "depends on" direction. IsBackRef
// marks a node that collapses a repeat visit (a diamond-shared blocker) or a
// cycle back to an already-rendered node, instead of re-expanding it.
type BlockedByNode struct {
	Issue     Issue
	IsBackRef bool
	Children  []BlockedByNode
}

// BuildBlockedByTree builds the transitive blocked-by tree rooted at root,
// expanding via blockersOf (issue id -> the issues it directly depends on).
// Every node is expanded at most once across the whole tree: a blocker
// reached a second time (whether via a different branch, a diamond, or a
// cycle back to an ancestor) is rendered as a back-reference instead of
// being re-expanded, which keeps the tree finite.
func BuildBlockedByTree(root Issue, blockersOf map[string][]Issue) BlockedByNode {
	seen := map[string]bool{root.ID: true}
	return buildBlockedByNode(root, blockersOf, seen)
}

func buildBlockedByNode(issue Issue, blockersOf map[string][]Issue, seen map[string]bool) BlockedByNode {
	node := BlockedByNode{Issue: issue}
	for _, blocker := range blockersOf[issue.ID] {
		if seen[blocker.ID] {
			node.Children = append(node.Children, BlockedByNode{Issue: blocker, IsBackRef: true})
			continue
		}
		seen[blocker.ID] = true
		node.Children = append(node.Children, buildBlockedByNode(blocker, blockersOf, seen))
	}
	return node
}
