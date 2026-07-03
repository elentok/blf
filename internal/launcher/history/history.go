package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const MaxEntries = 30

// ActionTypeCopy identifies a copy-type entry (calc/unit/currency results),
// which stores raw query text and has no direct-fire target.
//
// This must stay numerically in sync with launcher.ActionCopy's iota value
// (0). The history package intentionally does not import the launcher
// package (which already imports history) to avoid a cycle, so the
// correspondence is enforced by convention, not the type system.
const ActionTypeCopy = 0

// Entry is one persisted launcher-history row.
//
// For copy-type entries (ActionType == ActionTypeCopy), Label is the raw
// query text and Target is unused: recalling means populating the input
// with Label and recomputing.
//
// For every other ActionType (launch/run/open), Label is the picked
// result's display title and ActionType/Target are its Action.Type/Target:
// recalling means direct-firing that action.
type Entry struct {
	Label      string
	ActionType int
	Target     string
}

// key returns the identity used for dedup-on-append and Remove matching:
// (ActionType, Target) for anything with a real target, falling back to
// Label alone for copy-type entries (which have no target).
func (e Entry) key() string {
	if e.ActionType == ActionTypeCopy {
		return "label:" + e.Label
	}
	return "action:" + strconv.Itoa(e.ActionType) + ":" + e.Target
}

// History holds a capped, deduplicated list of launcher history entries,
// most-recent first.
type History struct {
	entries []Entry
}

// New returns an empty History.
func New() *History {
	return &History{}
}

// Load reads entries from path (one JSON object per line, most-recent
// first). Returns an empty History if the file does not exist, cannot be
// read, or fails to parse as JSON-lines (e.g. a pre-upgrade plain-text
// history file) — same failure-tolerant contract as a missing file.
func Load(path string) *History {
	data, err := os.ReadFile(path)
	if err != nil {
		return New()
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return New()
	}
	lines := strings.Split(trimmed, "\n")
	entries := make([]Entry, 0, len(lines))
	for _, l := range lines {
		if l == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			return New()
		}
		entries = append(entries, e)
	}
	if len(entries) > MaxEntries {
		entries = entries[:MaxEntries]
	}
	return &History{entries: entries}
}

// Save writes entries to path (one JSON object per line, most-recent
// first), creating parent dirs as needed.
func (h *History) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var sb strings.Builder
	for _, e := range h.entries {
		data, err := json.Marshal(e)
		if err != nil {
			return err
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

// Append adds entry to the front, moving it if an entry with the same
// identity key already exists, then caps at MaxEntries. Ignores entries
// with an empty Label.
func (h *History) Append(entry Entry) {
	entry.Label = strings.TrimSpace(entry.Label)
	if entry.Label == "" {
		return
	}
	k := entry.key()
	filtered := h.entries[:0]
	for _, e := range h.entries {
		if e.key() != k {
			filtered = append(filtered, e)
		}
	}
	h.entries = append([]Entry{entry}, filtered...)
	if len(h.entries) > MaxEntries {
		h.entries = h.entries[:MaxEntries]
	}
}

// Remove deletes the first entry with the same identity key as entry.
// Returns true if an entry was removed.
func (h *History) Remove(entry Entry) bool {
	k := entry.key()
	for i, e := range h.entries {
		if e.key() == k {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			return true
		}
	}
	return false
}

// Entries returns a snapshot of entries (most-recent first).
func (h *History) Entries() []Entry {
	out := make([]Entry, len(h.entries))
	copy(out, h.entries)
	return out
}

// Len returns the number of stored entries.
func (h *History) Len() int {
	return len(h.entries)
}
