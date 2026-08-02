package power

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PidInfo is the decoded contents of the daemon's pidfile.
type PidInfo struct {
	PID       int
	StartTime time.Time
}

// FormatPidFile renders a PidInfo as the daemon's pidfile content:
// "<pid>\n<start-RFC3339>\n".
func FormatPidFile(info PidInfo) []byte {
	return fmt.Appendf(nil, "%d\n%s\n", info.PID, info.StartTime.Format(time.RFC3339))
}

// ParsePidFile parses a pidfile's content back into a PidInfo.
func ParsePidFile(data []byte) (PidInfo, error) {
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 2 {
		return PidInfo{}, fmt.Errorf("malformed pidfile: want 2 lines, got %d", len(lines))
	}

	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return PidInfo{}, fmt.Errorf("malformed pidfile: invalid pid: %w", err)
	}

	startTime, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[1]))
	if err != nil {
		return PidInfo{}, fmt.Errorf("malformed pidfile: invalid start time: %w", err)
	}

	return PidInfo{PID: pid, StartTime: startTime}, nil
}

// IsProcessAlive reports whether pid is alive, using an injected liveness
// check (typically syscall.Kill(pid, 0)): ESRCH means dead, nil or any other
// error (e.g. EPERM, which still proves the pid exists) means alive.
func IsProcessAlive(pid int, kill func(pid int, sig syscall.Signal) error) bool {
	err := kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}
