package beads

import (
	"sort"
)

// PreviewFetcher is the subset of Adapter behavior powering the issue
// preview's subtasks + blocked-by trees, letting tests inject a stub instead
// of shelling out to bd.
type PreviewFetcher interface {
	DepTree(id string, direction DepDirection) ([]DepTreeNode, error)
}

// previewData is the async-loaded portion of an issue preview: the header
// and description come from the Issue the caller already has in hand and
// render instantly, while the two trees here are fetched lazily.
type previewData struct {
	subtasks  *SubtaskNode
	blockedBy *BlockedByNode
	err       error
}

// blocksDependencyType and parentChildDependencyType are the two edge types
// `bd dep tree` labels its nodes with (via edge_from_parent). A single dep
// tree call mixes both kinds of edges together, so the subtasks and
// blocked-by trees each need to keep only their own edge type and stop
// descending the moment an edge of the other type appears (e.g. root's own
// parent-child link to its epic, when walking the blocked-by/down tree).
const (
	blocksDependencyType      = "blocks"
	parentChildDependencyType = "parent-child"
)

// edgeChildrenOf reduces a flat `bd dep tree` result to a childrenOf-style
// map (issue id -> its direct children/blockers), keeping only nodes
// reachable from the root through an unbroken chain of edgeType edges. Nodes
// are processed in depth order (rather than list order) so a node's parent
// is always resolved before the node itself, regardless of how bd orders
// the response.
func edgeChildrenOf(nodes []DepTreeNode, edgeType string) map[string][]Issue {
	byDepth := make([]DepTreeNode, len(nodes))
	copy(byDepth, nodes)
	sort.SliceStable(byDepth, func(i, j int) bool { return byDepth[i].Depth < byDepth[j].Depth })

	kept := make(map[string]bool, len(nodes))
	childrenOf := make(map[string][]Issue)
	for _, n := range byDepth {
		if n.Depth == 0 {
			kept[n.ID] = true
			continue
		}
		if n.EdgeFromParent != edgeType || !kept[n.ParentID] {
			continue
		}
		kept[n.ID] = true
		childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n.Issue)
	}
	return childrenOf
}

// FetchSubtasksTree fetches root's entire subtask hierarchy via a single
// `bd dep tree --direction up` call and builds the nested subtasks tree.
func FetchSubtasksTree(fetcher PreviewFetcher, root Issue) (SubtaskNode, error) {
	nodes, err := fetcher.DepTree(root.ID, DepUp)
	if err != nil {
		return SubtaskNode{}, err
	}
	childrenOf := edgeChildrenOf(nodes, parentChildDependencyType)
	return BuildSubtasksTree(root, childrenOf), nil
}

// FetchBlockedByTree fetches root's entire transitive blocker chain via a
// single `bd dep tree` call and builds the blocked-by tree.
func FetchBlockedByTree(fetcher PreviewFetcher, root Issue) (BlockedByNode, error) {
	nodes, err := fetcher.DepTree(root.ID, DepDown)
	if err != nil {
		return BlockedByNode{}, err
	}
	blockersOf := edgeChildrenOf(nodes, blocksDependencyType)
	return BuildBlockedByTree(root, blockersOf), nil
}

// fetchPreviewData fetches both trees for root, keeping either tree nil when
// root has no subtasks/blockers so the preview can skip an empty section.
func fetchPreviewData(fetcher PreviewFetcher, root Issue) previewData {
	var data previewData

	// Fetched sequentially rather than concurrently: bd's underlying db
	// serializes concurrent invocations, so two bd processes racing each
	// other pay lock-contention overhead that ends up slower (and much
	// noisier) than just running them back to back.
	subtasks, err := FetchSubtasksTree(fetcher, root)
	if err != nil {
		data.err = err
	} else if len(subtasks.Children) > 0 {
		data.subtasks = &subtasks
	}

	blockedBy, err := FetchBlockedByTree(fetcher, root)
	if err != nil {
		data.err = err
	} else if len(blockedBy.Children) > 0 {
		data.blockedBy = &blockedBy
	}

	return data
}
