package launcher

import (
	"os"
	"strings"

	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/launcher/directories"
)

// DirectoryProvider fuzzy-matches configured filesystem directories and
// opens the selected one in the OS file manager.
type DirectoryProvider struct {
	weight float64
	dirs   []directories.Directory
	names  []string
}

var _ Provider = (*DirectoryProvider)(nil)

// NewDirectoryProvider filters dirs to those whose Path exists and is a
// directory (dirs must already have "~" expanded).
func NewDirectoryProvider(dirs []directories.Directory, weight float64) *DirectoryProvider {
	available := make([]directories.Directory, 0, len(dirs))
	for _, d := range dirs {
		if info, err := os.Stat(d.Path); err == nil && info.IsDir() {
			available = append(available, d)
		}
	}
	names := make([]string, len(available))
	for i, d := range available {
		names[i] = d.Name
	}
	return &DirectoryProvider{weight: weight, dirs: available, names: names}
}

func (p *DirectoryProvider) Query(input string) []Result {
	if Classify(input) == Computational {
		return nil
	}
	q := strings.TrimSpace(input)
	if q == "" {
		return nil
	}

	matches := fuzzyfinder.Find(q, p.names)
	results := make([]Result, 0, len(matches))
	for _, m := range matches {
		d := p.dirs[m.Index]
		lowerName := strings.ToLower(d.Name)
		lowerQ := strings.ToLower(q)
		results = append(results, Result{
			Title:         d.Name,
			Subtitle:      d.Path,
			Icon:          IconRoleDirectory,
			Source:        "directory",
			Weight:        p.weight,
			FuzzyScore:    m.Score,
			MatchRanges:   m.MatchedIndexes,
			IsExactMatch:  lowerName == lowerQ,
			IsPrefixMatch: strings.HasPrefix(lowerName, lowerQ),
			Action:        Action{Type: ActionOpen, Target: d.Path},
		})
	}
	return results
}

// LookupResult implements TargetLookupProvider by finding the directory whose
// Path matches action.Target.
func (p *DirectoryProvider) LookupResult(action Action) (Result, bool) {
	if action.Type != ActionOpen {
		return Result{}, false
	}
	for _, d := range p.dirs {
		if d.Path == action.Target {
			return Result{
				Title:    d.Name,
				Subtitle: d.Path,
				Icon:     IconRoleDirectory,
				Action:   Action{Type: ActionOpen, Target: d.Path},
			}, true
		}
	}
	return Result{}, false
}
