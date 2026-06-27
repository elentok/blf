package history

import (
	"os"
	"path/filepath"
	"strings"
)

const MaxEntries = 500

// History holds a capped, deduplicated list of launcher queries, most-recent first.
type History struct {
	entries []string
}

// New returns an empty History.
func New() *History {
	return &History{}
}

// Load reads entries from path (one per line, most-recent first).
// Returns an empty History if the file does not exist or cannot be read.
func Load(path string) *History {
	data, err := os.ReadFile(path)
	if err != nil {
		return New()
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	entries := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			entries = append(entries, l)
		}
	}
	if len(entries) > MaxEntries {
		entries = entries[:MaxEntries]
	}
	return &History{entries: entries}
}

// Save writes entries to path (one per line, most-recent first), creating parent dirs as needed.
func (h *History) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var sb strings.Builder
	for _, e := range h.entries {
		sb.WriteString(e)
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o600)
}

// Append adds query to the front, moving it if it already exists, then caps at MaxEntries.
// Ignores empty or whitespace-only strings.
func (h *History) Append(query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	// Remove existing occurrence (case-sensitive).
	filtered := h.entries[:0]
	for _, e := range h.entries {
		if e != q {
			filtered = append(filtered, e)
		}
	}
	// Prepend.
	h.entries = append([]string{q}, filtered...)
	if len(h.entries) > MaxEntries {
		h.entries = h.entries[:MaxEntries]
	}
}

// Remove deletes the first entry equal to query (case-sensitive).
// Returns true if an entry was removed.
func (h *History) Remove(query string) bool {
	for i, e := range h.entries {
		if e == query {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			return true
		}
	}
	return false
}

// Entries returns a snapshot of entries (most-recent first).
func (h *History) Entries() []string {
	out := make([]string, len(h.entries))
	copy(out, h.entries)
	return out
}

// Len returns the number of stored entries.
func (h *History) Len() int {
	return len(h.entries)
}
