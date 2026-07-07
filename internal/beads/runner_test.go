package beads

import (
	"errors"
	"os/exec"
	"testing"
)

func TestExecRunner_bdNotFound(t *testing.T) {
	if _, err := exec.LookPath("bd"); err == nil {
		t.Skip("bd is installed; cannot test the not-found path without mocking PATH")
	}
	_, err := execRunner{}.Run([]string{"where"})
	if !errors.Is(err, ErrBdNotFound) {
		t.Errorf("Run() = %v, want ErrBdNotFound", err)
	}
}

func TestExecRunner_success(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}
	out, err := execRunner{}.Run([]string{"types", "--json"})
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Error("Run() returned no output")
	}
}

func TestExecRunner_commandError(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}
	_, err := execRunner{}.Run([]string{"not-a-real-subcommand"})
	if err == nil {
		t.Fatal("expected error for invalid subcommand")
	}
}
