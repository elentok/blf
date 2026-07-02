package learnedrank

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Store holds counts of how many times a resultKey was picked for a given query.
type Store struct {
	counts map[string]map[string]int
}

// New returns an empty Store.
func New() *Store {
	return &Store{counts: make(map[string]map[string]int)}
}

// Load reads counts from path (one "query\tresultKey\tcount" triple per line).
// Returns an empty Store if the file does not exist or cannot be read.
func Load(path string) *Store {
	data, err := os.ReadFile(path)
	if err != nil {
		return New()
	}
	s := New()
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		count, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		s.set(parts[0], parts[1], count)
	}
	return s
}

// Save writes counts to path, creating parent directories as needed.
func (s *Store) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var sb strings.Builder
	for query, byKey := range s.counts {
		for resultKey, count := range byKey {
			sb.WriteString(query)
			sb.WriteByte('\t')
			sb.WriteString(resultKey)
			sb.WriteByte('\t')
			sb.WriteString(strconv.Itoa(count))
			sb.WriteByte('\n')
		}
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

// Increment increments the count for (query, resultKey) by 1.
// No-op if either query or resultKey is empty or whitespace-only after trimming.
func (s *Store) Increment(query, resultKey string) {
	q := strings.TrimSpace(query)
	k := strings.TrimSpace(resultKey)
	if q == "" || k == "" {
		return
	}
	s.set(q, k, s.get(q, k)+1)
}

// Counts returns a snapshot map of resultKey -> count for the given exact query.
// Returns an empty, non-nil map if none recorded.
func (s *Store) Counts(query string) map[string]int {
	out := make(map[string]int)
	for k, v := range s.counts[query] {
		out[k] = v
	}
	return out
}

func (s *Store) get(query, resultKey string) int {
	byKey, ok := s.counts[query]
	if !ok {
		return 0
	}
	return byKey[resultKey]
}

func (s *Store) set(query, resultKey string, count int) {
	byKey, ok := s.counts[query]
	if !ok {
		byKey = make(map[string]int)
		s.counts[query] = byKey
	}
	byKey[resultKey] = count
}
