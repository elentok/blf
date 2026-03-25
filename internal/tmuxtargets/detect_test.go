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
