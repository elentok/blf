package fuzzyfinder

import "github.com/sahilm/fuzzy"

// MatchRanges returns the rune indices in source that fuzzy-match query,
// suitable for highlight rendering. Returns nil, false if query doesn't match.
// An empty query matches everything and returns nil, true (no ranges to highlight).
func MatchRanges(query, source string) ([]int, bool) {
	if query == "" {
		return nil, true
	}
	matches := fuzzy.Find(query, []string{source})
	if len(matches) == 0 {
		return nil, false
	}
	return matches[0].MatchedIndexes, true
}
