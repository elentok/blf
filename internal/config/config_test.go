package config_test

import (
	"encoding/json"
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

func TestLoadConfig_launcherAI(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantModel   string
		wantTimeout string
		wantErr     bool
	}{
		{
			name:        "absent section yields defaults",
			data:        "",
			wantModel:   "haiku",
			wantTimeout: "120s",
		},
		{
			name: "provided section overrides both",
			data: `
[launcher.ai]
model = "sonnet"
timeout = "5m"
`,
			wantModel:   "sonnet",
			wantTimeout: "5m",
		},
		{
			name: "unparseable timeout falls back to default and raises a config error",
			data: `
[launcher.ai]
model = "sonnet"
timeout = "not-a-duration"
`,
			wantModel:   "sonnet",
			wantTimeout: "120s",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile := func(string) ([]byte, error) { return []byte(tt.data), nil }

			cfg, err := config.LoadConfig(readFile, "/home/nobody")
			if tt.wantErr && err == nil {
				t.Fatal("expected config error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Launcher.AI.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", cfg.Launcher.AI.Model, tt.wantModel)
			}
			if cfg.Launcher.AI.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %q, want %q", cfg.Launcher.AI.Timeout, tt.wantTimeout)
			}
		})
	}
}

func TestConfigSchemaJSON_launcherAI(t *testing.T) {
	var schema struct {
		Properties struct {
			Launcher struct {
				Properties struct {
					AI struct {
						Type       string `json:"type"`
						Properties struct {
							Model   struct{ Type string } `json:"model"`
							Timeout struct{ Type string } `json:"timeout"`
						} `json:"properties"`
					} `json:"ai"`
				} `json:"properties"`
			} `json:"launcher"`
		} `json:"properties"`
	}

	if err := json.Unmarshal(config.ConfigSchemaJSON(), &schema); err != nil {
		t.Fatalf("unmarshal embedded schema: %v", err)
	}

	ai := schema.Properties.Launcher.Properties.AI
	if ai.Type != "object" {
		t.Fatalf("expected launcher.ai to be an object, got %q", ai.Type)
	}
	if ai.Properties.Model.Type != "string" {
		t.Errorf("expected launcher.ai.model to be a string, got %q", ai.Properties.Model.Type)
	}
	if ai.Properties.Timeout.Type != "string" {
		t.Errorf("expected launcher.ai.timeout to be a string, got %q", ai.Properties.Timeout.Type)
	}
}
