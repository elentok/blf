package commands_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/launcher/commands"
)

func TestNewBuiltins(t *testing.T) {
	var reloadCalled, cleanURLCalled bool
	reload := func() tea.Cmd {
		reloadCalled = true
		return nil
	}
	cleanURL := func() tea.Cmd {
		cleanURLCalled = true
		return nil
	}

	cmds := commands.NewBuiltins(reload, cleanURL)

	if len(cmds) != 2 {
		t.Fatalf("expected 2 builtins, got %d", len(cmds))
	}
	if cmds[0].Name != "reload" {
		t.Errorf("cmds[0].Name = %q, want reload", cmds[0].Name)
	}
	if cmds[1].Name != "cleanurl" {
		t.Errorf("cmds[1].Name = %q, want cleanurl", cmds[1].Name)
	}

	cmds[0].Run()
	if !reloadCalled {
		t.Error("reload Run field was not wired to the reload func")
	}

	cmds[1].Run()
	if !cleanURLCalled {
		t.Error("cleanurl Run field was not wired to the cleanURL func")
	}
}
