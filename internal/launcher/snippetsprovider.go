package launcher

import (
	"strings"

	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/fuzzyfinder"
)

// SnippetsProvider is a Provider that fuzzy-matches named snippets and
// copies the selected snippet's value to the clipboard.
type SnippetsProvider struct {
	snippets []config.SnippetConfig
	weight   float64
}

var _ Provider = (*SnippetsProvider)(nil)

// NewSnippetsProvider creates a SnippetsProvider from a list of snippets.
func NewSnippetsProvider(snippets []config.SnippetConfig, weight float64) *SnippetsProvider {
	return &SnippetsProvider{snippets: snippets, weight: weight}
}

func (p *SnippetsProvider) Query(input string) []Result {
	if Classify(input) == Computational {
		return nil
	}
	q := strings.TrimSpace(input)
	if q == "" || len(p.snippets) == 0 {
		return nil
	}

	names := make([]string, len(p.snippets))
	for i, s := range p.snippets {
		names[i] = s.Name
	}

	matches := fuzzyfinder.Find(q, names)
	results := make([]Result, 0, len(matches))
	for _, m := range matches {
		s := p.snippets[m.Index]
		lowerName := strings.ToLower(s.Name)
		lowerQ := strings.ToLower(q)
		results = append(results, Result{
			Title:         s.Name,
			Icon:          IconRoleSnippet,
			IconGlyph:     s.Icon,
			Source:        "snippets",
			Weight:        p.weight,
			FuzzyScore:    m.Score,
			MatchRanges:   m.MatchedIndexes,
			IsExactMatch:  lowerName == lowerQ,
			IsPrefixMatch: strings.HasPrefix(lowerName, lowerQ),
			Action:        Action{Type: ActionCopy, Target: s.Value},
		})
	}
	return results
}
