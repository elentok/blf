package tmuxlinks

import (
	"reflect"
	"testing"
)

func TestExtractURLs(t *testing.T) {
	input := "see https://a.example/path, then http://b.example and https://a.example/path again"
	got := ExtractURLs(input)
	want := []string{"https://a.example/path", "http://b.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractURLs() = %#v, want %#v", got, want)
	}
}

func TestExtractURLsStripsTrailingPunctuation(t *testing.T) {
	input := "(https://example.com/x). [https://example.com/y]; https://example.com/z:"
	got := ExtractURLs(input)
	want := []string{"https://example.com/x", "https://example.com/y", "https://example.com/z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractURLs() = %#v, want %#v", got, want)
	}
}

func TestExtractURLsIgnoresNonHTTP(t *testing.T) {
	input := "mailto:me@example.com file:///tmp/a https://good.example"
	got := ExtractURLs(input)
	want := []string{"https://good.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractURLs() = %#v, want %#v", got, want)
	}
}
