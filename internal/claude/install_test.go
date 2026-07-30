package claude

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestInstallStatusLineMissingFileWritesCanonical(t *testing.T) {
	var wrotePath string
	var wroteData []byte
	d := InstallDeps{
		Stdout:      &bytes.Buffer{},
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		ReadFile:    func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		WriteFile: func(path string, data []byte, _ os.FileMode) error {
			wrotePath = path
			wroteData = data
			return nil
		},
	}

	if err := InstallStatusLine(d); err != nil {
		t.Fatalf("InstallStatusLine: %v", err)
	}
	if wrotePath != "/home/me/.claude/settings.json" {
		t.Fatalf("wrote to %q, want /home/me/.claude/settings.json", wrotePath)
	}

	var got map[string]any
	if err := json.Unmarshal(wroteData, &got); err != nil {
		t.Fatalf("written file is not valid JSON: %v\n%s", err, wroteData)
	}
	statusLine, ok := got["statusLine"].(map[string]any)
	if !ok {
		t.Fatalf("written settings missing statusLine object: %#v", got)
	}
	if statusLine["command"] != StatusLineCommand {
		t.Fatalf("statusLine command = %v, want %q", statusLine["command"], StatusLineCommand)
	}
	if statusLine["type"] != "command" {
		t.Fatalf("statusLine type = %v, want %q", statusLine["type"], "command")
	}
}

func TestInstallStatusLinePreservesOtherSettings(t *testing.T) {
	existing := `{"model":"sonnet","statusLine":{"type":"command","command":"old-command"}}`
	var wroteData []byte
	d := InstallDeps{
		Stdout:      &bytes.Buffer{},
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		ReadFile:    func(string) ([]byte, error) { return []byte(existing), nil },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		WriteFile: func(_ string, data []byte, _ os.FileMode) error {
			wroteData = data
			return nil
		},
	}

	if err := InstallStatusLine(d); err != nil {
		t.Fatalf("InstallStatusLine: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(wroteData, &got); err != nil {
		t.Fatalf("written file is not valid JSON: %v\n%s", err, wroteData)
	}
	if got["model"] != "sonnet" {
		t.Fatalf("expected unrelated settings preserved, got %#v", got)
	}
	statusLine := got["statusLine"].(map[string]any)
	if statusLine["command"] != StatusLineCommand {
		t.Fatalf("statusLine command = %v, want %q", statusLine["command"], StatusLineCommand)
	}
}

func TestInstallStatusLinePrintsConfirmation(t *testing.T) {
	var out bytes.Buffer
	d := InstallDeps{
		Stdout:      &out,
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		ReadFile:    func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		MkdirAll:    func(string, os.FileMode) error { return nil },
		WriteFile:   func(string, []byte, os.FileMode) error { return nil },
	}

	if err := InstallStatusLine(d); err != nil {
		t.Fatalf("InstallStatusLine: %v", err)
	}
	if !strings.Contains(out.String(), "/home/me/.claude/settings.json") {
		t.Fatalf("expected confirmation to mention settings path, got %q", out.String())
	}
}
