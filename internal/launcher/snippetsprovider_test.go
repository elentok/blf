package launcher_test

import (
	"testing"

	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/launcher"
)

func TestSnippetsProvider_matchesByNameAndCopiesValue(t *testing.T) {
	snippets := []config.SnippetConfig{
		{Name: "shipping", Value: "123 Main St"},
		{Name: "billing", Value: "456 Other Ave"},
	}
	p := launcher.NewSnippetsProvider(snippets, 1.0)

	results := p.Query("shipping")
	if len(results) != 1 || results[0].Title != "shipping" {
		t.Fatalf("expected only 'shipping' to match, got %+v", results)
	}
	got := results[0]
	if got.Action.Type != launcher.ActionCopy || got.Action.Target != "123 Main St" {
		t.Fatalf("expected ActionCopy with target '123 Main St', got %+v", got.Action)
	}
}

func TestSnippetsProvider_doesNotMatchByValue(t *testing.T) {
	snippets := []config.SnippetConfig{
		{Name: "shipping", Value: "123 Main St"},
	}
	p := launcher.NewSnippetsProvider(snippets, 1.0)

	if got := p.Query("Main St"); len(got) != 0 {
		t.Errorf("expected value content not to be searchable, got %+v", got)
	}
}

func TestSnippetsProvider_emptyInput(t *testing.T) {
	p := launcher.NewSnippetsProvider([]config.SnippetConfig{{Name: "shipping", Value: "x"}}, 1.0)

	if got := p.Query(""); len(got) != 0 {
		t.Errorf("expected no results for empty input, got %+v", got)
	}
}
