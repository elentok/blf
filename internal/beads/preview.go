package beads

// PreviewFetcher is the subset of Adapter behavior powering the issue
// preview's subtasks + blocked-by trees, letting tests inject a stub instead
// of shelling out to bd.
type PreviewFetcher interface {
	Children(id string) ([]Issue, error)
	DepList(id string) ([]Dependency, error)
}

// previewData is the async-loaded portion of an issue preview: the header
// and description come from the Issue the caller already has in hand and
// render instantly, while the two trees here are fetched lazily.
type previewData struct {
	subtasks  *SubtaskNode
	blockedBy *BlockedByNode
	err       error
}

// FetchSubtasksTree recursively fetches every level of root's children via
// fetcher.Children and builds the nested subtasks tree.
func FetchSubtasksTree(fetcher PreviewFetcher, root Issue) (SubtaskNode, error) {
	childrenOf := map[string][]Issue{}
	var walk func(id string) error
	walk = func(id string) error {
		children, err := fetcher.Children(id)
		if err != nil {
			return err
		}
		childrenOf[id] = children
		for _, child := range children {
			if err := walk(child.ID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root.ID); err != nil {
		return SubtaskNode{}, err
	}
	return BuildSubtasksTree(root, childrenOf), nil
}

// blocksDependencyType is the bd dep list edge type for an actual blocking
// dependency. bd dep list returns every edge type touching an issue
// (including "parent-child", the hierarchy edge owned by the subtasks tree),
// so this must be filtered out here to keep the two tree sections separate.
const blocksDependencyType = "blocks"

// FetchBlockedByTree recursively fetches every level of root's blockers via
// fetcher.DepList and builds the transitive blocked-by tree. Nodes already
// discovered are not re-fetched, so a cycle in the dependency data can't
// cause infinite fetching either.
func FetchBlockedByTree(fetcher PreviewFetcher, root Issue) (BlockedByNode, error) {
	blockersOf := map[string][]Issue{}
	visited := map[string]bool{root.ID: true}
	var walk func(id string) error
	walk = func(id string) error {
		deps, err := fetcher.DepList(id)
		if err != nil {
			return err
		}
		var blockers []Issue
		for _, d := range deps {
			if d.DependencyType == blocksDependencyType {
				blockers = append(blockers, d.Issue)
			}
		}
		blockersOf[id] = blockers
		for _, blocker := range blockers {
			if visited[blocker.ID] {
				continue
			}
			visited[blocker.ID] = true
			if err := walk(blocker.ID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root.ID); err != nil {
		return BlockedByNode{}, err
	}
	return BuildBlockedByTree(root, blockersOf), nil
}

// fetchPreviewData fetches both trees for root, keeping either tree nil when
// root has no subtasks/blockers so the preview can skip an empty section.
func fetchPreviewData(fetcher PreviewFetcher, root Issue) previewData {
	var data previewData

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
