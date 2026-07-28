package config_test

import (
	"errors"
	"os"
	"testing"

	"github.com/elentok/blf/internal/config"
)

func TestLoadSnippets_missingFile(t *testing.T) {
	readFile := func(string) ([]byte, error) { return nil, os.ErrNotExist }

	snippets, err := config.LoadSnippets(readFile, "/home/nobody")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(snippets) != 0 {
		t.Fatalf("expected no snippets, got %+v", snippets)
	}
}

func TestLoadSnippets_malformedFile(t *testing.T) {
	readFile := func(string) ([]byte, error) { return []byte("not = valid = toml = ["), nil }

	snippets, err := config.LoadSnippets(readFile, "/home/nobody")
	if err == nil {
		t.Fatal("expected error for malformed file")
	}
	if len(snippets) != 0 {
		t.Fatalf("expected no snippets on error, got %+v", snippets)
	}
}

func TestLoadSnippets_parsesEntries(t *testing.T) {
	data := []byte(`
[[snippet]]
name = "shipping"
icon = ""
value = """
123 Main St
Springfield
"""

[[snippet]]
name = "billing"
value = "456 Other Ave"
`)
	readFile := func(string) ([]byte, error) { return data, nil }

	snippets, err := config.LoadSnippets(readFile, "/home/nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snippets) != 2 {
		t.Fatalf("expected 2 snippets, got %+v", snippets)
	}
	if snippets[0].Name != "shipping" || snippets[0].Icon != "" {
		t.Errorf("unexpected first snippet: %+v", snippets[0])
	}
	if snippets[1].Name != "billing" || snippets[1].Value != "456 Other Ave" {
		t.Errorf("unexpected second snippet: %+v", snippets[1])
	}
}

func TestLoadSnippets_readErrorOtherThanNotExist(t *testing.T) {
	wantErr := errors.New("permission denied")
	readFile := func(string) ([]byte, error) { return nil, wantErr }

	_, err := config.LoadSnippets(readFile, "/home/nobody")
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}
