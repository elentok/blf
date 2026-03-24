package tmuxlinks

import (
	"fmt"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	got := shellQuote("https://example.com/a'b")
	want := `'https://example.com/a'"'"'b'`
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestBuildDisplayMenuArgsOpen(t *testing.T) {
	args, err := BuildDisplayMenuArgs(ModeOpen, []string{"https://example.com"})
	if err != nil {
		t.Fatalf("BuildDisplayMenuArgs returned error: %v", err)
	}

	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "Open URL") {
		t.Fatalf("expected title Open URL in args: %#v", args)
	}
	if !strings.Contains(joined, "run-shell") {
		t.Fatalf("expected run-shell callback in args: %#v", args)
	}
	if !strings.Contains(joined, "blf open") {
		t.Fatalf("expected callback to run blf open: %#v", args)
	}
	if !strings.Contains(joined, "-x\nC\n-y\nC") {
		t.Fatalf("expected centered menu args in %#v", args)
	}
}

func TestBuildDisplayMenuArgsCappedTo30(t *testing.T) {
	urls := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		urls = append(urls, fmt.Sprintf("https://example.com/%d", i))
	}

	args, err := BuildDisplayMenuArgs(ModeCopy, urls)
	if err != nil {
		t.Fatalf("BuildDisplayMenuArgs returned error: %v", err)
	}

	const fixed = 7 // display-menu -T <title> -x C -y C
	if got := (len(args) - fixed) / 3; got != 30 {
		t.Fatalf("menu entries = %d, want 30", got)
	}
}

func TestTruncateLabel(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 120)
	got := truncateLabel(long, 20)
	if len([]rune(got)) != 20 {
		t.Fatalf("truncateLabel length = %d, want 20", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncateLabel missing ellipsis: %q", got)
	}
}
