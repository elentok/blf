package kitty

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// managedCommandMarker identifies the hooks this command owns. Matching is
// deliberately NARROW: only hooks whose command contains this exact substring
// are reconciled, so unrelated `blf kitty ...` hooks are never touched.
const managedCommandMarker = "blf kitty set-agent-state"

// managedHook is one canonical Claude Code hook setup-claude installs. Non-tool
// events (UserPromptSubmit/Notification/Stop) take no matcher; the tool events
// (PreToolUse/PostToolUse) match every tool with "*".
type managedHook struct {
	event   string
	matcher string
	command string
}

// managedHooks is the canonical hook set. Keeping the agent's reported state in
// sync: working while a prompt is being handled and around each tool call,
// waiting when Claude asks for input, idle when it stops. PostToolUse is what
// clears "waiting" after the user answers a question/permission prompt — the
// question tool completing is the reliable re-engagement signal, since answering
// through the selector is not a UserPromptSubmit. After the final tool,
// PostToolUse sets working but Stop fires last and wins (idle). See ADR 0004.
var managedHooks = []managedHook{
	{event: "UserPromptSubmit", command: "blf kitty set-agent-state working"},
	{event: "PreToolUse", matcher: "*", command: "blf kitty set-agent-state working"},
	{event: "PostToolUse", matcher: "*", command: "blf kitty set-agent-state working"},
	{event: "Notification", command: "blf kitty set-agent-state waiting --only-if-working"},
	{event: "Stop", command: "blf kitty set-agent-state idle"},
}

// reconcileClaudeHooks is the pure core of setup-claude: it takes parsed Claude
// settings and returns them with this command's managed hooks brought up to
// date. It (a) strips every managed hook (marker match) from every event,
// (b) drops any event left with no hook groups, then (c) re-adds the canonical
// managed groups. Unknown keys are preserved by value, and because step (a)
// removes exactly what step (c) adds, running it twice is a no-op.
func reconcileClaudeHooks(settings map[string]any) map[string]any {
	if settings == nil {
		settings = map[string]any{}
	}

	hooks := asStringMap(settings["hooks"])

	for event, raw := range hooks {
		groups := pruneManagedGroups(raw)
		if len(groups) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = groups
	}

	for _, mh := range managedHooks {
		hooks[mh.event] = append(asAnySlice(hooks[mh.event]), managedGroup(mh.matcher, mh.command))
	}

	settings["hooks"] = hooks
	return settings
}

// pruneManagedGroups removes managed hooks from one event's group list: any
// inner hook matching the marker is dropped, and a group whose hooks become
// empty is dropped entirely. Entries that aren't recognizable groups are kept
// untouched.
func pruneManagedGroups(raw any) []any {
	groups := asAnySlice(raw)
	kept := make([]any, 0, len(groups))
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			kept = append(kept, g)
			continue
		}
		inner, ok := group["hooks"].([]any)
		if !ok {
			kept = append(kept, g)
			continue
		}

		keptInner := make([]any, 0, len(inner))
		for _, h := range inner {
			if isManagedHook(h) {
				continue
			}
			keptInner = append(keptInner, h)
		}
		if len(keptInner) == 0 {
			continue
		}
		group["hooks"] = keptInner
		kept = append(kept, group)
	}
	return kept
}

func isManagedHook(h any) bool {
	hook, ok := h.(map[string]any)
	if !ok {
		return false
	}
	command, _ := hook["command"].(string)
	return strings.Contains(command, managedCommandMarker)
}

func managedGroup(matcher, command string) map[string]any {
	group := map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	}
	if matcher != "" {
		group["matcher"] = matcher
	}
	return group
}

func asStringMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asAnySlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

// SetupClaude installs/refreshes the agent-state hooks in the user's global
// Claude Code settings (~/.claude/settings.json), idempotently. With dryRun it
// prints the resulting change to stdout and writes nothing.
func SetupClaude(dryRun bool, d Deps) error {
	home, err := d.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	path := filepath.Join(home, ".claude", "settings.json")

	settings, err := readClaudeSettings(path, d)
	if err != nil {
		return err
	}

	before, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	reconcileClaudeHooks(settings)

	after, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	if dryRun {
		_, err := io.WriteString(d.Stdout, renderSettingsDiff(path, string(before), string(after)))
		return err
	}

	dir := filepath.Dir(path)
	if err := d.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := d.WriteFile(path, append(after, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// readClaudeSettings parses the settings file, treating a missing or empty file
// as an empty object.
func readClaudeSettings(path string, d Deps) (map[string]any, error) {
	data, err := d.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		settings = map[string]any{}
	}
	return settings, nil
}

// renderSettingsDiff produces the dry-run output: a line diff of the
// canonicalized settings (so JSON reformatting never shows as noise).
func renderSettingsDiff(path, before, after string) string {
	if before == after {
		return fmt.Sprintf("%s is already up to date\n", path)
	}
	return fmt.Sprintf("# dry-run, %s would change:\n%s", path, lineDiff(before, after))
}

// lineDiff renders a minimal LCS-based unified-style diff: unchanged lines are
// prefixed "  ", removals "- ", additions "+ ".
func lineDiff(before, after string) string {
	a := strings.Split(before, "\n")
	b := strings.Split(after, "\n")
	m, n := len(a), len(b)

	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out strings.Builder
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case a[i] == b[j]:
			fmt.Fprintf(&out, "  %s\n", a[i])
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			fmt.Fprintf(&out, "- %s\n", a[i])
			i++
		default:
			fmt.Fprintf(&out, "+ %s\n", b[j])
			j++
		}
	}
	for ; i < m; i++ {
		fmt.Fprintf(&out, "- %s\n", a[i])
	}
	for ; j < n; j++ {
		fmt.Fprintf(&out, "+ %s\n", b[j])
	}
	return out.String()
}
