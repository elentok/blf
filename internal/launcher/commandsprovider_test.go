package launcher_test

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/commands"
)

func testCommands() []commands.Command {
	return []commands.Command{
		{Name: "reload", Run: func() tea.Cmd { return nil }},
		{Name: "cleanurl", Run: func() tea.Cmd { return nil }},
	}
}

func TestCommandsProvider_Query(t *testing.T) {
	p := launcher.NewCommandsProvider(testCommands(), 1.0)

	results := p.Query("reload")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'reload', got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Title != "reload" {
		t.Errorf("Title = %q, want reload", r.Title)
	}
	if r.Icon != launcher.IconRoleCommand {
		t.Errorf("Icon = %v, want IconRoleCommand", r.Icon)
	}
	if r.Action != (launcher.Action{Type: launcher.ActionCommand, Target: "reload"}) {
		t.Errorf("Action = %+v, want ActionCommand/reload", r.Action)
	}
	if !r.IsExactMatch {
		t.Error("expected IsExactMatch for 'reload'")
	}
}

func TestCommandsProvider_Query_fuzzy(t *testing.T) {
	p := launcher.NewCommandsProvider(testCommands(), 1.0)

	results := p.Query("clnurl")
	if len(results) == 0 {
		t.Fatal("expected fuzzy match for 'clnurl'")
	}
	if results[0].Title != "cleanurl" {
		t.Errorf("Title = %q, want cleanurl", results[0].Title)
	}
}

func TestCommandsProvider_Query_empty(t *testing.T) {
	p := launcher.NewCommandsProvider(testCommands(), 1.0)
	if results := p.Query(""); results != nil {
		t.Errorf("expected nil results for empty query, got %v", results)
	}
}

func TestCommandsProvider_Find(t *testing.T) {
	p := launcher.NewCommandsProvider(testCommands(), 1.0)

	if _, ok := p.Find("RELOAD"); !ok {
		t.Error("expected case-insensitive Find to match 'reload'")
	}
	if _, ok := p.Find("missing"); ok {
		t.Error("expected Find to fail for unknown name")
	}
}

func TestCommandsProvider_LookupResult(t *testing.T) {
	p := launcher.NewCommandsProvider(testCommands(), 1.0)

	r, ok := p.LookupResult(launcher.Action{Type: launcher.ActionCommand, Target: "cleanurl"})
	if !ok {
		t.Fatal("expected LookupResult to find 'cleanurl'")
	}
	if r.Title != "cleanurl" {
		t.Errorf("Title = %q, want cleanurl", r.Title)
	}

	if _, ok := p.LookupResult(launcher.Action{Type: launcher.ActionRun, Target: "cleanurl"}); ok {
		t.Error("expected LookupResult to reject non-ActionCommand actions")
	}
	if _, ok := p.LookupResult(launcher.Action{Type: launcher.ActionCommand, Target: "missing"}); ok {
		t.Error("expected LookupResult to fail for unknown target")
	}
}

func TestNewBuiltinCommands(t *testing.T) {
	cmds := launcher.NewBuiltinCommands("/home/user", "/home/user/.cache/apps.json", func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	})
	if len(cmds) != 4 {
		t.Fatalf("expected 4 builtin commands, got %d", len(cmds))
	}
	names := []string{cmds[0].Name, cmds[1].Name, cmds[2].Name, cmds[3].Name}
	want := []string{"reload", "cleanurl", "ai", "improve"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("names = %v, want %v", names, want)
			break
		}
	}
	for _, c := range cmds {
		if c.Run == nil {
			t.Errorf("expected builtin command %q to have a non-nil Run func", c.Name)
		}
	}
}
