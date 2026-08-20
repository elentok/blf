package launcher

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/elentok/blf/internal/launcher/ai"
)

// aiRecentRunLimit caps how many recent ai runs surface as recent-item rows.
const aiRecentRunLimit = 5

// aiTitleMaxLen is the approximate character budget for a row's title before
// it is truncated with a tail ellipsis.
const aiTitleMaxLen = 60

// AIProvider surfaces the newest successful ai runs as recent-item rows. It
// contributes nothing for a non-empty query: `ai` and `improve` are resolved
// through the commands provider, and searching past runs is out of scope.
type AIProvider struct {
	store *ai.Store
}

var _ Provider = (*AIProvider)(nil)

// NewAIProvider creates an AIProvider reading from store.
func NewAIProvider(store *ai.Store) *AIProvider {
	return &AIProvider{store: store}
}

// Query returns rows for the aiRecentRunLimit newest successful runs when
// input is empty, oldest-first order preserved from the store (most-recent
// first); it returns nil for any non-empty input.
func (p *AIProvider) Query(input string) []Result {
	if input != "" || p.store == nil {
		return nil
	}

	runs := p.store.Runs()
	results := make([]Result, 0, aiRecentRunLimit)
	for _, r := range runs {
		if r.Status != ai.StatusSuccess {
			continue
		}
		results = append(results, aiResultFromRun(r))
		if len(results) == aiRecentRunLimit {
			break
		}
	}
	return results
}

// aiResultFromRun builds the recent-items row for a stored ai run. Action.Target
// is the run's ID, so acting on the row (ticket 10) can look it up in the store.
func aiResultFromRun(r ai.Run) Result {
	icon := IconRoleAI
	if r.Kind == ai.KindImprove {
		icon = IconRoleImprove
	}
	return Result{
		Title:    ansi.Truncate(r.Input, aiTitleMaxLen, "…"),
		Subtitle: firstLine(r.Response),
		Icon:     icon,
		Action:   Action{Type: ActionAIRun, Target: r.ID},
	}
}

// firstLine returns the text before the first newline in s, or all of s if
// it has none.
func firstLine(s string) string {
	if before, _, found := strings.Cut(s, "\n"); found {
		return before
	}
	return s
}
