package power

import (
	"fmt"
	"strings"
	"time"
)

// FormatMarkFile renders a checkpoint timestamp as `power mark`'s file
// content: "<RFC3339>\n".
func FormatMarkFile(t time.Time) []byte {
	return fmt.Appendf(nil, "%s\n", t.Format(time.RFC3339))
}

// ParseMarkFile parses a mark file's content back into a timestamp.
func ParseMarkFile(data []byte) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}, fmt.Errorf("malformed mark file: %w", err)
	}
	return t, nil
}
