package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestConfigEditSnippets_seedsExampleWhenMissing(t *testing.T) {
	written := map[string][]byte{}
	d := deps{
		userHomeDir: func() (string, error) { return "/home/nobody", nil },
		fileExists:  func(string) (bool, error) { return false, nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		writeFile: func(path string, data []byte, _ os.FileMode) error {
			written[path] = data
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false }, // no EDITOR set
		stdin:     strings.NewReader(""),
		stdout:    &strings.Builder{},
		stderr:    &strings.Builder{},
	}

	err := runConfigEditSnippets(d)
	if err == nil || !strings.Contains(err.Error(), "EDITOR") {
		t.Fatalf("expected EDITOR-not-set error, got %v", err)
	}

	snippetsPath := findPathSuffix(written, "snippets.toml")
	schemaPath := findPathSuffix(written, "snippets.schema.json")
	taploPath := findPathSuffix(written, "taplo.toml")

	if snippetsPath == "" {
		t.Fatalf("expected snippets.toml to be seeded, wrote %v", written)
	}
	if !strings.Contains(string(written[snippetsPath]), "[[snippet]]") {
		t.Fatalf("expected commented-out example snippet, got %q", written[snippetsPath])
	}
	if !strings.HasPrefix(string(written[snippetsPath]), "#:schema ./snippets.schema.json") {
		t.Fatalf("expected #:schema pragma as first line, got %q", written[snippetsPath])
	}
	if schemaPath == "" {
		t.Fatalf("expected snippets.schema.json to be written, wrote %v", written)
	}
	if !strings.Contains(string(written[schemaPath]), `"title": "blf snippets"`) {
		t.Fatalf("expected embedded JSON schema content, got %q", written[schemaPath])
	}
	if taploPath == "" {
		t.Fatalf("expected taplo.toml to be written, wrote %v", written)
	}
	if !strings.Contains(string(written[taploPath]), `include = ["snippets.toml"]`) {
		t.Fatalf("expected taplo.toml to associate snippets.toml, got %q", written[taploPath])
	}
}

func TestConfigEditSnippets_doesNotRewriteExistingWithPragma(t *testing.T) {
	written := map[string][]byte{}
	existing := "#:schema ./snippets.schema.json\n[[snippet]]\nname = \"x\"\nvalue = \"y\"\n"
	d := deps{
		userHomeDir: func() (string, error) { return "/home/nobody", nil },
		fileExists:  func(string) (bool, error) { return true, nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		readFile:    func(string) ([]byte, error) { return []byte(existing), nil },
		writeFile: func(path string, data []byte, _ os.FileMode) error {
			written[path] = data
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false },
		stdin:     strings.NewReader(""),
		stdout:    &strings.Builder{},
		stderr:    &strings.Builder{},
	}

	_ = runConfigEditSnippets(d)
	if findPathSuffix(written, "snippets.toml") != "" {
		t.Fatalf("expected existing snippets.toml not to be rewritten, but wrote %v", written)
	}
	if len(written) != 2 {
		t.Fatalf("expected only the schema and taplo.toml files to be (re)written, got %v", written)
	}
}

func TestConfigEditSnippets_backfillsPragmaOnExistingFile(t *testing.T) {
	written := map[string][]byte{}
	existing := "[[snippet]]\nname = \"x\"\nvalue = \"y\"\n"
	d := deps{
		userHomeDir: func() (string, error) { return "/home/nobody", nil },
		fileExists:  func(string) (bool, error) { return true, nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		readFile:    func(string) ([]byte, error) { return []byte(existing), nil },
		writeFile: func(path string, data []byte, _ os.FileMode) error {
			written[path] = data
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false },
		stdin:     strings.NewReader(""),
		stdout:    &strings.Builder{},
		stderr:    &strings.Builder{},
	}

	_ = runConfigEditSnippets(d)

	snippetsPath := findPathSuffix(written, "snippets.toml")
	if snippetsPath == "" {
		t.Fatalf("expected snippets.toml to be rewritten with the pragma, wrote %v", written)
	}
	got := string(written[snippetsPath])
	if !strings.HasPrefix(got, "#:schema ./snippets.schema.json\n") {
		t.Fatalf("expected pragma prepended, got %q", got)
	}
	if !strings.Contains(got, existing) {
		t.Fatalf("expected original content preserved, got %q", got)
	}
}

func TestConfigEdit_seedsDefaultsWhenMissing(t *testing.T) {
	written := map[string][]byte{}
	d := deps{
		userHomeDir: func() (string, error) { return "/home/nobody", nil },
		fileExists:  func(string) (bool, error) { return false, nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		writeFile: func(path string, data []byte, _ os.FileMode) error {
			written[path] = data
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false }, // no EDITOR set
		stdin:     strings.NewReader(""),
		stdout:    &strings.Builder{},
		stderr:    &strings.Builder{},
	}

	err := runConfigEdit(d)
	if err == nil || !strings.Contains(err.Error(), "EDITOR") {
		t.Fatalf("expected EDITOR-not-set error, got %v", err)
	}

	configPath := findPathSuffix(written, "config.toml")
	schemaPath := findPathSuffix(written, "config.schema.json")
	taploPath := findPathSuffix(written, "taplo.toml")

	if configPath == "" {
		t.Fatalf("expected config.toml to be seeded, wrote %v", written)
	}
	if !strings.HasPrefix(string(written[configPath]), "#:schema ./config.schema.json") {
		t.Fatalf("expected #:schema pragma as first line, got %q", written[configPath])
	}
	if !strings.Contains(string(written[configPath]), "[launcher]") {
		t.Fatalf("expected encoded defaults, got %q", written[configPath])
	}
	if schemaPath == "" {
		t.Fatalf("expected config.schema.json to be written, wrote %v", written)
	}
	if !strings.Contains(string(written[schemaPath]), `"title": "blf config"`) {
		t.Fatalf("expected embedded JSON schema content, got %q", written[schemaPath])
	}
	if taploPath == "" {
		t.Fatalf("expected taplo.toml to be written, wrote %v", written)
	}
	if !strings.Contains(string(written[taploPath]), `include = ["config.toml"]`) {
		t.Fatalf("expected taplo.toml to associate config.toml, got %q", written[taploPath])
	}
}

func TestConfigEdit_backfillsPragmaOnExistingFile(t *testing.T) {
	written := map[string][]byte{}
	existing := "[launcher]\napp_weight = 1.0\n"
	d := deps{
		userHomeDir: func() (string, error) { return "/home/nobody", nil },
		fileExists:  func(string) (bool, error) { return true, nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		readFile:    func(string) ([]byte, error) { return []byte(existing), nil },
		writeFile: func(path string, data []byte, _ os.FileMode) error {
			written[path] = data
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false },
		stdin:     strings.NewReader(""),
		stdout:    &strings.Builder{},
		stderr:    &strings.Builder{},
	}

	_ = runConfigEdit(d)

	configPath := findPathSuffix(written, "config.toml")
	if configPath == "" {
		t.Fatalf("expected config.toml to be rewritten with the pragma, wrote %v", written)
	}
	got := string(written[configPath])
	if !strings.HasPrefix(got, "#:schema ./config.schema.json\n") {
		t.Fatalf("expected pragma prepended, got %q", got)
	}
	if !strings.Contains(got, existing) {
		t.Fatalf("expected original content preserved, got %q", got)
	}
}

func TestConfigEdit_doesNotRewriteExistingWithPragma(t *testing.T) {
	written := map[string][]byte{}
	existing := "#:schema ./config.schema.json\n[launcher]\napp_weight = 1.0\n"
	d := deps{
		userHomeDir: func() (string, error) { return "/home/nobody", nil },
		fileExists:  func(string) (bool, error) { return true, nil },
		mkdirAll:    func(string, os.FileMode) error { return nil },
		readFile:    func(string) ([]byte, error) { return []byte(existing), nil },
		writeFile: func(path string, data []byte, _ os.FileMode) error {
			written[path] = data
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false },
		stdin:     strings.NewReader(""),
		stdout:    &strings.Builder{},
		stderr:    &strings.Builder{},
	}

	_ = runConfigEdit(d)
	if findPathSuffix(written, "config.toml") != "" {
		t.Fatalf("expected existing config.toml not to be rewritten, but wrote %v", written)
	}
	if len(written) != 2 {
		t.Fatalf("expected only the schema and taplo.toml files to be (re)written, got %v", written)
	}
}

func findPathSuffix(written map[string][]byte, suffix string) string {
	for p := range written {
		if strings.HasSuffix(p, suffix) {
			return p
		}
	}
	return ""
}
