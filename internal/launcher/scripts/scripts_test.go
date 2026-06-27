package scripts_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/elentok/blf/internal/launcher/scripts"
)

func TestFilterForPlatform_mac(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("mac-only test")
	}
	input := []scripts.Script{
		{Name: "mac-only", Platform: "mac"},
		{Name: "linux-only", Platform: "linux"},
		{Name: "both", Platform: ""},
	}
	got := scripts.FilterForPlatform(input)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(got), got)
	}
	if got[0].Name != "mac-only" || got[1].Name != "both" {
		t.Errorf("unexpected names: %v", got)
	}
}

func TestFilterForPlatform_linux(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("linux-only test")
	}
	input := []scripts.Script{
		{Name: "mac-only", Platform: "mac"},
		{Name: "linux-only", Platform: "linux"},
		{Name: "both", Platform: ""},
	}
	got := scripts.FilterForPlatform(input)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(got), got)
	}
	if got[0].Name != "linux-only" || got[1].Name != "both" {
		t.Errorf("unexpected names: %v", got)
	}
}

func TestRunBash_success(t *testing.T) {
	s := scripts.Script{Type: scripts.TypeBash, Body: "echo hello"}
	r := scripts.Run(s)
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if r.Stdout != "hello" {
		t.Errorf("expected 'hello', got %q", r.Stdout)
	}
}

func TestRunBash_error(t *testing.T) {
	s := scripts.Script{Type: scripts.TypeBash, Body: "exit 1"}
	r := scripts.Run(s)
	if r.Err == nil {
		t.Fatal("expected error for exit 1")
	}
}

func TestRunBash_stderr(t *testing.T) {
	s := scripts.Script{Type: scripts.TypeBash, Body: "echo oops >&2; exit 1"}
	r := scripts.Run(s)
	if r.Err == nil {
		t.Fatal("expected error")
	}
	if r.Stderr != "oops" {
		t.Errorf("expected 'oops' in stderr, got %q", r.Stderr)
	}
}

func TestRunBash_outputTrimmed(t *testing.T) {
	s := scripts.Script{Type: scripts.TypeBash, Body: "printf 'line1\nline2\n'"}
	r := scripts.Run(s)
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if strings.HasSuffix(r.Stdout, "\n") {
		t.Errorf("trailing newline not trimmed: %q", r.Stdout)
	}
	if !strings.Contains(r.Stdout, "line1") {
		t.Errorf("expected 'line1' in output, got %q", r.Stdout)
	}
}

func TestMerge_override(t *testing.T) {
	builtins := []scripts.Script{
		{Name: "playpause", Body: "original"},
		{Name: "cleanurl", Body: "original"},
	}
	user := []scripts.Script{
		{Name: "playpause", Body: "overridden"},
		{Name: "mynew", Body: "new"},
	}
	got := scripts.Merge(builtins, user)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	found := map[string]string{}
	for _, s := range got {
		found[s.Name] = s.Body
	}
	if found["playpause"] != "overridden" {
		t.Errorf("override not applied: %q", found["playpause"])
	}
	if found["cleanurl"] != "original" {
		t.Errorf("untouched builtin changed: %q", found["cleanurl"])
	}
	if found["mynew"] != "new" {
		t.Errorf("new script not added: %q", found["mynew"])
	}
}
