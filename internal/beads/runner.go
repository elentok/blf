package beads

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// ErrBdNotFound is returned when the bd binary is not in PATH.
var ErrBdNotFound = errors.New("bd (Beads CLI) not found in PATH")

// Runner runs the bd binary with the given argv and returns its stdout.
// Injected so tests can fake bd's output without a real binary.
type Runner interface {
	Run(args []string) ([]byte, error)
}

// execRunner is the real Runner, invoking the bd binary via os/exec.
type execRunner struct{}

func (execRunner) Run(args []string) ([]byte, error) {
	cmd := exec.Command("bd", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, ErrBdNotFound
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New("bd " + strings.Join(args, " ") + ": " + msg)
	}

	return stdout.Bytes(), nil
}
