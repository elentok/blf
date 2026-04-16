package targets

import (
	"reflect"
	"testing"
)

func TestNormalizeViewportTextReplacesPromptGlyphs(t *testing.T) {
	got := NormalizeViewportText("abcde\r\nnext\n")
	want := []string{"a b c d e", "next"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}

func TestNormalizeViewportTextHandlesEmptyInput(t *testing.T) {
	got := NormalizeViewportText("")
	if len(got) != 0 {
		t.Fatalf("expected no lines, got %#v", got)
	}
}
