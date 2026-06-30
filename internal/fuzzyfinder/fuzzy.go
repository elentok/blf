package fuzzyfinder

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// Find matches query against candidates, splitting query on whitespace into
// words and requiring every word to fuzzy-match (AND semantics): a candidate
// is only included if it matches all words, not just any one of them.
// A candidate's score is the sum of its per-word scores, and MatchedIndexes
// is the merged, deduplicated, sorted union of each word's matched indexes.
// Results are sorted by descending score, mirroring fuzzy.Find. An empty
// query returns nil, matching the underlying library's behavior.
func Find(query string, candidates []string) fuzzy.Matches {
	words := strings.Fields(query)
	if len(words) == 0 {
		return nil
	}

	combined := make(map[int]*fuzzy.Match)
	for wi, word := range words {
		wordMatches := fuzzy.Find(word, candidates)
		if len(wordMatches) == 0 {
			return nil
		}
		if wi == 0 {
			for i := range wordMatches {
				m := wordMatches[i]
				combined[m.Index] = &m
			}
			continue
		}
		for i := range wordMatches {
			wm := wordMatches[i]
			existing, ok := combined[wm.Index]
			if !ok {
				continue
			}
			existing.Score += wm.Score
			existing.MatchedIndexes = mergeIndexes(existing.MatchedIndexes, wm.MatchedIndexes)
		}
		for idx := range combined {
			if !matchedWord(wordMatches, idx) {
				delete(combined, idx)
			}
		}
	}

	matches := make(fuzzy.Matches, 0, len(combined))
	for _, m := range combined {
		matches = append(matches, *m)
	}
	sort.Stable(matches)
	return matches
}

func matchedWord(wordMatches fuzzy.Matches, index int) bool {
	for _, m := range wordMatches {
		if m.Index == index {
			return true
		}
	}
	return false
}

// mergeIndexes returns the deduplicated, sorted union of a and b.
func mergeIndexes(a, b []int) []int {
	seen := make(map[int]struct{}, len(a)+len(b))
	merged := make([]int, 0, len(a)+len(b))
	for _, idx := range a {
		if _, ok := seen[idx]; !ok {
			seen[idx] = struct{}{}
			merged = append(merged, idx)
		}
	}
	for _, idx := range b {
		if _, ok := seen[idx]; !ok {
			seen[idx] = struct{}{}
			merged = append(merged, idx)
		}
	}
	sort.Ints(merged)
	return merged
}

// MatchRanges returns the rune indices in source that fuzzy-match query,
// suitable for highlight rendering. Returns nil, false if query doesn't match.
// An empty query matches everything and returns nil, true (no ranges to highlight).
func MatchRanges(query, source string) ([]int, bool) {
	if query == "" {
		return nil, true
	}
	matches := Find(query, []string{source})
	if len(matches) == 0 {
		return nil, false
	}
	return matches[0].MatchedIndexes, true
}
