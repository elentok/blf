package beads

import "sort"

// TreeRow is one row of the rendered issue list: an issue plus its nesting
// depth (0 = top-level) and whether it's shown only as context for a
// matching descendant rather than being a match itself. See CONTEXT.md's
// "issue tree" entry.
type TreeRow struct {
	Issue  Issue
	Depth  int
	Dimmed bool
}

// BuildIssueTree lays out issues the way `bd list` prints them in the
// terminal: grouped under their parent (Issue.Parent), with top-level items
// and each sibling group sorted by id ascending. `bd list --json` is not
// itself tree-ordered (a child can appear before its own parent in the
// array), so this reconstructs the grouping client-side.
//
// matchIDs, when non-nil, restricts the result to matching issues plus
// their ancestors: a non-matching ancestor of a match is still included
// (Dimmed: true) so the match keeps its tree context, but a subtree with no
// match anywhere in it is dropped entirely. A nil matchIDs (empty query)
// includes every issue, undimmed.
func BuildIssueTree(issues []Issue, matchIDs map[string]bool) []TreeRow {
	byID := make(map[string]Issue, len(issues))
	for _, issue := range issues {
		byID[issue.ID] = issue
	}

	childrenOf := make(map[string][]Issue)
	var topLevel []Issue
	for _, issue := range issues {
		if issue.Parent != "" {
			if _, ok := byID[issue.Parent]; ok {
				childrenOf[issue.Parent] = append(childrenOf[issue.Parent], issue)
				continue
			}
		}
		topLevel = append(topLevel, issue)
	}
	sortByID(topLevel)
	for id := range childrenOf {
		sortByID(childrenOf[id])
	}

	var rows []TreeRow
	for _, issue := range topLevel {
		rows = append(rows, buildTreeRows(issue, 0, childrenOf, matchIDs)...)
	}
	return rows
}

func sortByID(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
}

// buildTreeRows emits issue and its descendants as rows. When matchIDs is
// non-nil, a subtree is pruned unless issue itself matches or at least one
// descendant does; an included issue that doesn't itself match is marked
// Dimmed.
func buildTreeRows(issue Issue, depth int, childrenOf map[string][]Issue, matchIDs map[string]bool) []TreeRow {
	var childRows []TreeRow
	for _, child := range childrenOf[issue.ID] {
		childRows = append(childRows, buildTreeRows(child, depth+1, childrenOf, matchIDs)...)
	}

	if matchIDs == nil || matchIDs[issue.ID] {
		return append([]TreeRow{{Issue: issue, Depth: depth}}, childRows...)
	}
	if len(childRows) > 0 {
		return append([]TreeRow{{Issue: issue, Depth: depth, Dimmed: true}}, childRows...)
	}
	return nil
}
