package cmd

import (
	"strings"
	"testing"
)

func TestRunKittyTargetsRoutesToDependency(t *testing.T) {
	var got []string
	d := deps{
		runKittyTargets: func(args []string) error {
			got = append([]string{}, args...)
			return nil
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"targets", "--overlay", "--target", "17"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(got, " ") != "--overlay --target 17" {
		t.Fatalf("kitty targets called with %v", got)
	}
}
