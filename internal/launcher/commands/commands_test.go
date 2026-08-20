package commands_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/launcher/commands"
)

func TestNewBuiltins(t *testing.T) {
	var reloadCalled, cleanURLCalled, aiCalled, improveCalled bool
	reload := func() tea.Cmd {
		reloadCalled = true
		return nil
	}
	cleanURL := func() tea.Cmd {
		cleanURLCalled = true
		return nil
	}
	ai := func() tea.Cmd {
		aiCalled = true
		return nil
	}
	improve := func() tea.Cmd {
		improveCalled = true
		return nil
	}

	cmds := commands.NewBuiltins(reload, cleanURL, ai, improve)

	if len(cmds) != 4 {
		t.Fatalf("expected 4 builtins, got %d", len(cmds))
	}
	if cmds[0].Name != "reload" {
		t.Errorf("cmds[0].Name = %q, want reload", cmds[0].Name)
	}
	if cmds[1].Name != "cleanurl" {
		t.Errorf("cmds[1].Name = %q, want cleanurl", cmds[1].Name)
	}
	if cmds[2].Name != "ai" {
		t.Errorf("cmds[2].Name = %q, want ai", cmds[2].Name)
	}
	if cmds[3].Name != "improve" {
		t.Errorf("cmds[3].Name = %q, want improve", cmds[3].Name)
	}

	cmds[0].Run()
	if !reloadCalled {
		t.Error("reload Run field was not wired to the reload func")
	}

	cmds[1].Run()
	if !cleanURLCalled {
		t.Error("cleanurl Run field was not wired to the cleanURL func")
	}

	cmds[2].Run()
	if !aiCalled {
		t.Error("ai Run field was not wired to the ai func")
	}

	cmds[3].Run()
	if !improveCalled {
		t.Error("improve Run field was not wired to the improve func")
	}
}
