package tmuxtargets

import (
	"reflect"
	"testing"
)

func TestDetectTargetsFindsRequestedPatterns(t *testing.T) {
	lines := []string{
		"visit hello.com/world and https://example.com/path and ticket #123",
		"path src/main.go:12:5 email me@example.com hash deadbeef uuid 550e8400-e29b-41d4-a716-446655440000",
		"host api.example.com:443 branch feature/tmux-targets",
		"agent codex resume abc123_def-456",
		"tool opencode -s ses_2b871e869fferrNuTKf7FV4oXf",
		"cli claude --resume 0bf7fab1-358e-49a0-95fd-fd7cede8baac",
		"worker agent --resume thread_123",
		"cursor cursor-agent --resume thread-456",
	}

	targets := detectTargets(lines)

	gotTexts := make([]string, 0, len(targets))
	for _, tr := range targets {
		gotTexts = append(gotTexts, tr.text)
	}
	wantTexts := []string{
		"hello.com/world",
		"https://example.com/path",
		"#123",
		"src/main.go:12:5",
		"me@example.com",
		"deadbeef",
		"550e8400-e29b-41d4-a716-446655440000",
		"api.example.com:443",
		"feature/tmux-targets",
		"codex resume abc123_def-456",
		"opencode -s ses_2b871e869fferrNuTKf7FV4oXf",
		"claude --resume 0bf7fab1-358e-49a0-95fd-fd7cede8baac",
		"agent --resume thread_123",
		"cursor-agent --resume thread-456",
	}
	if !reflect.DeepEqual(gotTexts, wantTexts) {
		t.Fatalf("target texts = %#v, want %#v", gotTexts, wantTexts)
	}

	var hasOpenable bool
	for _, tr := range targets {
		if tr.openable {
			hasOpenable = true
			break
		}
	}
	if !hasOpenable {
		t.Fatal("expected at least one openable target")
	}
}

func TestDetectTargetsRecognizesResumeCommandAsSingleTarget(t *testing.T) {
	lines := []string{
		"resume with codex resume abc123_def-456 now",
		"tool opencode -s ses_2b871e869fferrNuTKf7FV4oXf",
		"cli claude --resume 0bf7fab1-358e-49a0-95fd-fd7cede8baac",
		"worker agent --resume thread_123",
		"cursor cursor-agent --resume thread-456",
	}
	targets := detectTargets(lines)

	want := []string{
		"codex resume abc123_def-456",
		"opencode -s ses_2b871e869fferrNuTKf7FV4oXf",
		"claude --resume 0bf7fab1-358e-49a0-95fd-fd7cede8baac",
		"agent --resume thread_123",
		"cursor-agent --resume thread-456",
	}
	if len(targets) != len(want) {
		t.Fatalf("expected %d targets, got %d (%#v)", len(want), len(targets), targets)
	}
	for i, text := range want {
		if targets[i].text != text {
			t.Fatalf("target %d = %q, want %q", i, targets[i].text, text)
		}
		if targets[i].openable {
			t.Fatalf("target %q should not be openable", targets[i].text)
		}
	}
}

func TestDetectTargetsPrefersURLOverBareDomainOverlap(t *testing.T) {
	line := []string{"check https://example.com/path now"}
	targets := detectTargets(line)
	if len(targets) == 0 {
		t.Fatal("expected target")
	}
	if targets[0].text != "https://example.com/path" {
		t.Fatalf("expected full url target, got %q", targets[0].text)
	}
}

func TestDetectTargetsDeduplicatesRepeatedTargetText(t *testing.T) {
	lines := []string{
		"first https://example.com/path and #123",
		"repeat https://example.com/path and #123 again",
	}

	targets := detectTargets(lines)
	if len(targets) != 2 {
		t.Fatalf("expected 2 unique targets, got %d (%#v)", len(targets), targets)
	}
	if targets[0].text != "https://example.com/path" {
		t.Fatalf("first target = %q", targets[0].text)
	}
	if targets[1].text != "#123" {
		t.Fatalf("second target = %q", targets[1].text)
	}
}

func TestDetectTargetsRecognizesTildePaths(t *testing.T) {
	lines := []string{"open ~/my/path and ~/my/other/file.go:42"}
	targets := detectTargets(lines)

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d (%#v)", len(targets), targets)
	}
	if targets[0].text != "~/my/path" {
		t.Fatalf("first target = %q", targets[0].text)
	}
	if targets[1].text != "~/my/other/file.go:42" {
		t.Fatalf("second target = %q", targets[1].text)
	}
}

func TestDetectTargetsBareDomainRequiresPath(t *testing.T) {
	lines := []string{"README.md github.com github.com/elentok"}
	targets := detectTargets(lines)

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 target, got %d (%#v)", len(targets), targets)
	}
	if targets[0].text != "github.com/elentok" {
		t.Fatalf("target = %q, want github.com/elentok", targets[0].text)
	}
}

func TestDetectTargetsBareFilenameIgnoredButPathAccepted(t *testing.T) {
	lines := []string{"README.md src/README.md"}
	targets := detectTargets(lines)

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 target, got %d (%#v)", len(targets), targets)
	}
	if targets[0].text != "src/README.md" {
		t.Fatalf("target = %q, want src/README.md", targets[0].text)
	}
}
