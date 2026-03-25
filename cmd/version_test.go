package cmd

import (
	"strings"
	"testing"
)

func TestRunVersionUsesInjectedVersion(t *testing.T) {
	orig := version
	version = "v1.2.3"
	t.Cleanup(func() { version = orig })

	b := &strings.Builder{}
	if err := runVersion(b); err != nil {
		t.Fatalf("runVersion returned error: %v", err)
	}
	if got := b.String(); got != "blf v1.2.3\n" {
		t.Fatalf("version output = %q", got)
	}
}
