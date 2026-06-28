package kitty

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// eventGroups returns the hook groups registered for an event.
func eventGroups(t *testing.T, settings map[string]any, event string) []any {
	t.Helper()
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("settings has no hooks object: %#v", settings)
	}
	groups, ok := hooks[event].([]any)
	if !ok {
		t.Fatalf("event %q is not a group array: %#v", event, hooks[event])
	}
	return groups
}

// groupCommands returns every command string inside a hook group.
func groupCommands(t *testing.T, group any) []string {
	t.Helper()
	m, ok := group.(map[string]any)
	if !ok {
		t.Fatalf("group is not an object: %#v", group)
	}
	inner, ok := m["hooks"].([]any)
	if !ok {
		t.Fatalf("group has no hooks array: %#v", group)
	}
	var cmds []string
	for _, h := range inner {
		hook, ok := h.(map[string]any)
		if !ok {
			t.Fatalf("hook is not an object: %#v", h)
		}
		cmd, _ := hook["command"].(string)
		cmds = append(cmds, cmd)
	}
	return cmds
}

// eventCommands flattens every command across every group for an event.
func eventCommands(t *testing.T, settings map[string]any, event string) []string {
	t.Helper()
	var all []string
	for _, g := range eventGroups(t, settings, event) {
		all = append(all, groupCommands(t, g)...)
	}
	return all
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestReconcileClaudeHooksInstallsCanonicalSet(t *testing.T) {
	got := reconcileClaudeHooks(map[string]any{})

	want := map[string]string{
		"UserPromptSubmit": "blf kitty set-agent-state working",
		"PreToolUse":       "blf kitty set-agent-state working",
		"Notification":     "blf kitty set-agent-state waiting",
		"Stop":             "blf kitty set-agent-state idle",
	}
	for event, command := range want {
		groups := eventGroups(t, got, event)
		if len(groups) != 1 {
			t.Fatalf("event %q has %d groups, want 1", event, len(groups))
		}
		if cmds := groupCommands(t, groups[0]); len(cmds) != 1 || cmds[0] != command {
			t.Fatalf("event %q commands = %v, want [%q]", event, cmds, command)
		}
	}

	// PreToolUse is the only tool event and must carry matcher "*".
	pre := eventGroups(t, got, "PreToolUse")[0].(map[string]any)
	if pre["matcher"] != "*" {
		t.Fatalf("PreToolUse matcher = %v, want *", pre["matcher"])
	}
	// Non-tool events take no matcher.
	for _, event := range []string{"UserPromptSubmit", "Notification", "Stop"} {
		g := eventGroups(t, got, event)[0].(map[string]any)
		if _, ok := g["matcher"]; ok {
			t.Fatalf("event %q should have no matcher: %#v", event, g)
		}
	}
}

func TestReconcileClaudeHooksIsIdempotent(t *testing.T) {
	mustJSON := func(v any) string {
		t.Helper()
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(data)
	}

	once := mustJSON(reconcileClaudeHooks(map[string]any{}))

	var reparsed map[string]any
	if err := json.Unmarshal([]byte(once), &reparsed); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	twice := mustJSON(reconcileClaudeHooks(reparsed))

	if once != twice {
		t.Fatalf("not idempotent:\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestReconcileClaudeHooksPrunesStaleEventAndRemovesEmptyKey(t *testing.T) {
	// A managed hook lingering under a since-renamed event must be removed, and
	// the now-empty event key dropped.
	input := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "blf kitty set-agent-state working"},
					},
				},
			},
		},
	}

	got := reconcileClaudeHooks(input)
	hooks := got["hooks"].(map[string]any)
	if _, ok := hooks["PostToolUse"]; ok {
		t.Fatalf("stale PostToolUse event should be removed: %#v", hooks["PostToolUse"])
	}
	// Canonical set still installed.
	if cmds := eventCommands(t, got, "Stop"); !contains(cmds, "blf kitty set-agent-state idle") {
		t.Fatalf("Stop missing canonical hook: %v", cmds)
	}
}

func TestReconcileClaudeHooksPreservesUnrelatedSettings(t *testing.T) {
	input := map[string]any{
		"permissions": map[string]any{"allow": []any{"Bash"}},
		"env":         map[string]any{"FOO": "bar"},
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo hi"},
					},
				},
			},
		},
	}

	got := reconcileClaudeHooks(input)

	if perms, ok := got["permissions"].(map[string]any); !ok || perms["allow"] == nil {
		t.Fatalf("permissions not preserved: %#v", got["permissions"])
	}
	if env, ok := got["env"].(map[string]any); !ok || env["FOO"] != "bar" {
		t.Fatalf("env not preserved: %#v", got["env"])
	}
	if cmds := eventCommands(t, got, "PostToolUse"); !contains(cmds, "echo hi") {
		t.Fatalf("unrelated PostToolUse hook not preserved: %v", cmds)
	}
	// Canonical set still installed alongside.
	if cmds := eventCommands(t, got, "UserPromptSubmit"); !contains(cmds, "blf kitty set-agent-state working") {
		t.Fatalf("UserPromptSubmit missing canonical hook: %v", cmds)
	}
}

func TestReconcileClaudeHooksLeavesOtherBlfHooksAlone(t *testing.T) {
	// An unrelated `blf kitty ...` hook (not set-agent-state) must survive,
	// even when it shares an event/group with a managed hook.
	input := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "blf kitty other-cmd"},
						map[string]any{"type": "command", "command": "blf kitty set-agent-state idle"},
					},
				},
			},
		},
	}

	got := reconcileClaudeHooks(input)
	cmds := eventCommands(t, got, "Stop")
	if !contains(cmds, "blf kitty other-cmd") {
		t.Fatalf("unrelated blf hook was pruned: %v", cmds)
	}
	// The canonical idle hook is present exactly once (old managed one pruned,
	// fresh one appended).
	count := 0
	for _, c := range cmds {
		if c == "blf kitty set-agent-state idle" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one managed idle hook, got %d: %v", count, cmds)
	}
}

func TestSetupClaudeMissingFileWritesCanonical(t *testing.T) {
	var wrotePath string
	var wroteData []byte
	d := Deps{
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

	if err := SetupClaude(false, d); err != nil {
		t.Fatalf("SetupClaude: %v", err)
	}
	if wrotePath != "/home/me/.claude/settings.json" {
		t.Fatalf("wrote to %q, want /home/me/.claude/settings.json", wrotePath)
	}

	var got map[string]any
	if err := json.Unmarshal(wroteData, &got); err != nil {
		t.Fatalf("written file is not valid JSON: %v\n%s", err, wroteData)
	}
	if cmds := eventCommands(t, got, "Notification"); !contains(cmds, "blf kitty set-agent-state waiting") {
		t.Fatalf("written settings missing canonical Notification hook: %v", cmds)
	}
}

func TestSetupClaudeDryRunWritesNothing(t *testing.T) {
	writeCalled := false
	var out bytes.Buffer
	d := Deps{
		Stdout:      &out,
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		ReadFile:    func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		MkdirAll:    func(string, os.FileMode) error { writeCalled = true; return nil },
		WriteFile:   func(string, []byte, os.FileMode) error { writeCalled = true; return nil },
	}

	if err := SetupClaude(true, d); err != nil {
		t.Fatalf("SetupClaude: %v", err)
	}
	if writeCalled {
		t.Fatalf("--dry-run must not write or mkdir")
	}
	if !strings.Contains(out.String(), "set-agent-state") {
		t.Fatalf("--dry-run should show the change, got: %q", out.String())
	}
}
