package launcher

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/launcher/apps"
)

// AppsReindexedMsg is emitted when app reindexing completes.
type AppsReindexedMsg struct {
	Index *apps.Index
	Err   error
}

// AppsRefreshTickMsg triggers a periodic reindex.
type AppsRefreshTickMsg struct{}

// AppLaunchResultMsg is emitted when an async app launch completes.
type AppLaunchResultMsg struct {
	AppPath string
	Err     error
}

// ReindexCmd scans the filesystem for apps and saves the result to cachePath.
func ReindexCmd(homeDir, cachePath string) tea.Cmd {
	return func() tea.Msg {
		idx, err := apps.Reindex(homeDir)
		if err != nil {
			return AppsReindexedMsg{Err: err}
		}
		if cachePath != "" {
			_ = apps.Save(cachePath, idx)
		}
		return AppsReindexedMsg{Index: idx}
	}
}

// LoadAppsFromDiskCmd reads the cached apps index without a full filesystem scan.
func LoadAppsFromDiskCmd(cachePath string) tea.Cmd {
	return func() tea.Msg {
		idx, err := apps.Load(cachePath)
		return AppsReindexedMsg{Index: idx, Err: err}
	}
}

// ScheduleAppsRefresh returns a tea.Cmd that fires AppsRefreshTickMsg after d.
func ScheduleAppsRefresh(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		<-time.After(d)
		return AppsRefreshTickMsg{}
	}
}

// AppsProvider is a Provider that fuzzy-matches installed applications.
type AppsProvider struct {
	index  *apps.Index
	weight float64
}

var _ Provider = (*AppsProvider)(nil)

// NewAppsProvider creates an AppsProvider with the given source weight.
func NewAppsProvider(weight float64) *AppsProvider {
	return &AppsProvider{weight: weight}
}

// SetIndex updates the provider's app index.
func (p *AppsProvider) SetIndex(idx *apps.Index) {
	p.index = idx
}

// IndexedAt returns the timestamp of the current index, or zero if none.
func (p *AppsProvider) IndexedAt() time.Time {
	if p.index == nil {
		return time.Time{}
	}
	return p.index.IndexedAt
}

func (p *AppsProvider) Query(input string) []Result {
	if Classify(input) == Computational {
		return nil
	}
	q := strings.TrimSpace(input)
	if q == "" || p.index == nil || len(p.index.Apps) == 0 {
		return nil
	}

	names := make([]string, len(p.index.Apps))
	for i, a := range p.index.Apps {
		names[i] = a.Name
	}

	matches := fuzzyfinder.Find(q, names)
	results := make([]Result, 0, len(matches))
	for _, m := range matches {
		app := p.index.Apps[m.Index]
		lowerName := strings.ToLower(app.Name)
		lowerQ := strings.ToLower(q)
		results = append(results, Result{
			Title:         app.Name,
			Subtitle:      app.Subtitle,
			Icon:          IconRoleApp,
			IconGlyph:     AppIconGlyph(app.Name),
			Source:        "apps",
			Weight:        p.weight,
			FuzzyScore:    m.Score,
			MatchRanges:   m.MatchedIndexes,
			IsExactMatch:  lowerName == lowerQ,
			IsPrefixMatch: strings.HasPrefix(lowerName, lowerQ),
			Action:        Action{Type: ActionLaunch, Target: app.Path},
		})
	}
	return results
}

// LookupResult implements TargetLookupProvider by finding the app whose Path
// matches action.Target.
func (p *AppsProvider) LookupResult(action Action) (Result, bool) {
	if action.Type != ActionLaunch || p.index == nil {
		return Result{}, false
	}
	for _, app := range p.index.Apps {
		if app.Path == action.Target {
			return Result{
				Title:     app.Name,
				Subtitle:  app.Subtitle,
				Icon:      IconRoleApp,
				IconGlyph: AppIconGlyph(app.Name),
				Action:    Action{Type: ActionLaunch, Target: app.Path},
			}, true
		}
	}
	return Result{}, false
}
