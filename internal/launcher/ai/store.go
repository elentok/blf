// Package ai holds the pure logic behind the launcher's ai and improve
// helpers: prompt construction, invocation, and the runs store.
package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Kind identifies which ai helper produced a run.
type Kind string

const (
	KindAI      Kind = "ai"
	KindImprove Kind = "improve"
)

// Status is the outcome of a run.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
	StatusTimeout Status = "timeout"
)

// MaxRuns is the cap on stored records; Append trims the oldest beyond it.
const MaxRuns = 200

// StateFileName is the runs store's file name under config.XDGStateDir.
const StateFileName = "ai-runs.jsonl"

// Run is one persisted ai-run record.
type Run struct {
	ID        string
	Timestamp time.Time
	Kind      Kind
	Input     string
	Response  string
	Status    Status
}

// Store holds a capped list of Run records, most-recent first.
type Store struct {
	runs []Run
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{}
}

// LoadStore reads runs from path (one JSON object per line, most-recent
// first). Returns an empty Store if the file does not exist, cannot be
// read, or fails to parse as JSON-lines — same failure-tolerant contract as
// history.Load.
func LoadStore(path string) *Store {
	data, err := os.ReadFile(path)
	if err != nil {
		return NewStore()
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return NewStore()
	}
	lines := strings.Split(trimmed, "\n")
	runs := make([]Run, 0, len(lines))
	for _, l := range lines {
		if l == "" {
			continue
		}
		var r Run
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			return NewStore()
		}
		runs = append(runs, r)
	}
	if len(runs) > MaxRuns {
		runs = runs[:MaxRuns]
	}
	return &Store{runs: runs}
}

// Save writes runs to path (one JSON object per line, most-recent first),
// creating parent dirs as needed. It rewrites the whole file, matching
// history.Save.
func (s *Store) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var sb strings.Builder
	for _, r := range s.runs {
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

// Append adds run to the front, then caps at MaxRuns.
func (s *Store) Append(run Run) {
	s.runs = append([]Run{run}, s.runs...)
	if len(s.runs) > MaxRuns {
		s.runs = s.runs[:MaxRuns]
	}
}

// Delete removes the run with the given id. Returns true if a run was
// removed.
func (s *Store) Delete(id string) bool {
	for i, r := range s.runs {
		if r.ID == id {
			s.runs = append(s.runs[:i], s.runs[i+1:]...)
			return true
		}
	}
	return false
}

// Runs returns a snapshot of runs (most-recent first).
func (s *Store) Runs() []Run {
	out := make([]Run, len(s.runs))
	copy(out, s.runs)
	return out
}

// Len returns the number of stored runs.
func (s *Store) Len() int {
	return len(s.runs)
}
