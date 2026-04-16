package targets

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

	tgts := DetectTargets(lines)

	gotTexts := make([]string, 0, len(tgts))
	for _, tr := range tgts {
		gotTexts = append(gotTexts, tr.Text)
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
	for _, tr := range tgts {
		if tr.Openable {
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
	tgts := DetectTargets(lines)

	want := []string{
		"codex resume abc123_def-456",
		"opencode -s ses_2b871e869fferrNuTKf7FV4oXf",
		"claude --resume 0bf7fab1-358e-49a0-95fd-fd7cede8baac",
		"agent --resume thread_123",
		"cursor-agent --resume thread-456",
	}
	if len(tgts) != len(want) {
		t.Fatalf("expected %d targets, got %d (%#v)", len(want), len(tgts), tgts)
	}
	for i, text := range want {
		if tgts[i].Text != text {
			t.Fatalf("target %d = %q, want %q", i, tgts[i].Text, text)
		}
		if tgts[i].Openable {
			t.Fatalf("target %q should not be openable", tgts[i].Text)
		}
	}
}

func TestDetectTargetsPrefersURLOverBareDomainOverlap(t *testing.T) {
	line := []string{"check https://example.com/path now"}
	tgts := DetectTargets(line)
	if len(tgts) == 0 {
		t.Fatal("expected target")
	}
	if tgts[0].Text != "https://example.com/path" {
		t.Fatalf("expected full url target, got %q", tgts[0].Text)
	}
}

func TestDetectTargetsDeduplicatesRepeatedTargetText(t *testing.T) {
	lines := []string{
		"first https://example.com/path and #123",
		"repeat https://example.com/path and #123 again",
	}

	tgts := DetectTargets(lines)
	if len(tgts) != 2 {
		t.Fatalf("expected 2 unique targets, got %d (%#v)", len(tgts), tgts)
	}
	if tgts[0].Text != "https://example.com/path" {
		t.Fatalf("first target = %q", tgts[0].Text)
	}
	if tgts[1].Text != "#123" {
		t.Fatalf("second target = %q", tgts[1].Text)
	}
}

func TestDetectTargetsRecognizesTildePaths(t *testing.T) {
	lines := []string{"open ~/my/path and ~/my/other/file.go:42"}
	tgts := DetectTargets(lines)

	if len(tgts) != 2 {
		t.Fatalf("expected 2 targets, got %d (%#v)", len(tgts), tgts)
	}
	if tgts[0].Text != "~/my/path" {
		t.Fatalf("first target = %q", tgts[0].Text)
	}
	if tgts[1].Text != "~/my/other/file.go:42" {
		t.Fatalf("second target = %q", tgts[1].Text)
	}
}

func TestDetectTargetsBareDomainRequiresPath(t *testing.T) {
	lines := []string{"README.md github.com github.com/elentok"}
	tgts := DetectTargets(lines)

	if len(tgts) != 1 {
		t.Fatalf("expected exactly 1 target, got %d (%#v)", len(tgts), tgts)
	}
	if tgts[0].Text != "github.com/elentok" {
		t.Fatalf("target = %q, want github.com/elentok", tgts[0].Text)
	}
}

func TestDetectTargetsBareFilenameIgnoredButPathAccepted(t *testing.T) {
	lines := []string{"README.md src/README.md"}
	tgts := DetectTargets(lines)

	if len(tgts) != 1 {
		t.Fatalf("expected exactly 1 target, got %d (%#v)", len(tgts), tgts)
	}
	if tgts[0].Text != "src/README.md" {
		t.Fatalf("target = %q, want src/README.md", tgts[0].Text)
	}
}
