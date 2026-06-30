package fuzzyfinder

import (
	"slices"
	"testing"
)

func indexes(matches []int) []int {
	if matches == nil {
		return []int{}
	}
	return matches
}

func TestFindSingleWordMatchesAnyCandidateContainingIt(t *testing.T) {
	candidates := []string{"one fish", "two fish", "red fish"}
	matches := Find("fish", candidates)
	if len(matches) != 3 {
		t.Fatalf("expected all 3 candidates to match, got %d", len(matches))
	}
}

func TestFindMultiWordRequiresAllWordsToMatch(t *testing.T) {
	candidates := []string{"one fish two fish", "red fish blue fish", "one two three"}
	matches := Find("one two", candidates)

	var got []string
	for _, m := range matches {
		got = append(got, m.Str)
	}
	want := []string{"one fish two fish", "one two three"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFindMultiWordOrderIndependent(t *testing.T) {
	candidates := []string{"one two"}
	m1 := Find("one two", candidates)
	m2 := Find("two one", candidates)
	if len(m1) != 1 || len(m2) != 1 {
		t.Fatalf("expected both word orders to match, got %d and %d", len(m1), len(m2))
	}
}

func TestFindNoMatchWhenOneWordMissing(t *testing.T) {
	candidates := []string{"one fish two fish"}
	matches := Find("one three", candidates)
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(matches))
	}
}

func TestFindEmptyQueryReturnsNil(t *testing.T) {
	matches := Find("", []string{"anything"})
	if matches != nil {
		t.Fatalf("expected nil for empty query, got %v", matches)
	}
}

func TestFindMergesMatchedIndexes(t *testing.T) {
	candidates := []string{"one two"}
	matches := Find("one two", candidates)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	idx := indexes(matches[0].MatchedIndexes)
	if !slices.IsSorted(idx) {
		t.Fatalf("expected sorted indexes, got %v", idx)
	}
	seen := map[int]bool{}
	for _, i := range idx {
		if seen[i] {
			t.Fatalf("expected deduplicated indexes, got duplicate %d in %v", i, idx)
		}
		seen[i] = true
	}
}

func TestMatchRangesEmptyQueryMatchesEverything(t *testing.T) {
	ranges, ok := MatchRanges("", "anything")
	if !ok || ranges != nil {
		t.Fatalf("expected (nil, true) for empty query, got (%v, %v)", ranges, ok)
	}
}

func TestMatchRangesMultiWordAnd(t *testing.T) {
	if _, ok := MatchRanges("one two", "one fish two fish"); !ok {
		t.Fatalf("expected match")
	}
	if _, ok := MatchRanges("one three", "one fish two fish"); ok {
		t.Fatalf("expected no match")
	}
}
