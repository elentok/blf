package launcher

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher/ai"
	"github.com/elentok/blf/internal/launcher/apps"
	"github.com/elentok/blf/internal/launcher/commands"
	"github.com/elentok/blf/internal/launcher/history"
	"github.com/elentok/blf/internal/launcher/learnedrank"
)

// fakeProvider returns two fixed results for any non-empty query, regardless
// of query content — used to exercise learned-rank wiring without depending
// on fuzzy matching or filesystem state.
type fakeProvider struct{}

func (fakeProvider) Query(input string) []Result {
	if input == "" {
		return nil
	}
	return []Result{
		{Title: "First", Action: Action{Type: ActionCopy, Target: "first"}},
		{Title: "Second", Action: Action{Type: ActionCopy, Target: "second"}},
	}
}

// typeText feeds each rune to the model as a key press, returning the updated model.
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		msg := tea.KeyPressMsg{Code: r, Text: string(r)}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

// runCmd executes a tea.Cmd (if non-nil) and returns its message.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestEscResetsAndHides(t *testing.T) {
	hidden := false
	m := NewModel(ModelConfig{
		Providers:    []Provider{CalcProvider{}},
		HideTerminal: func() error { hidden = true; return nil },
		HideDelay:    0,
	})

	m = typeText(t, m, "1+1")
	if m.input.Value() != "1+1" {
		t.Fatalf("expected input %q, got %q", "1+1", m.input.Value())
	}
	if len(m.results) == 0 {
		t.Fatal("expected results before esc")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)

	if m.input.Value() != "" {
		t.Errorf("expected input reset, got %q", m.input.Value())
	}
	if len(m.results) != 0 {
		t.Errorf("expected results cleared, got %d", len(m.results))
	}
	if m.selected != 0 || m.offset != 0 {
		t.Errorf("expected selection reset, got selected=%d offset=%d", m.selected, m.offset)
	}

	runCmd(cmd)
	if !hidden {
		t.Error("expected HideTerminal to be called")
	}
}

func TestSyncEnterResetsAndHides(t *testing.T) {
	hidden := false
	copied := ""
	m := NewModel(ModelConfig{
		Providers:    []Provider{CalcProvider{}},
		CopyText:     func(s string) error { copied = s; return nil },
		HideTerminal: func() error { hidden = true; return nil },
		HideDelay:    0,
	})

	m = typeText(t, m, "2*3")
	if len(m.results) == 0 {
		t.Fatal("expected calc result")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if copied == "" {
		t.Error("expected calc value copied to clipboard")
	}
	if m.input.Value() != "" {
		t.Errorf("expected input reset, got %q", m.input.Value())
	}
	if len(m.results) != 0 {
		t.Errorf("expected results cleared, got %d", len(m.results))
	}

	runCmd(cmd)
	if !hidden {
		t.Error("expected HideTerminal to be called")
	}
}

func copyHistoryEntry(label string) history.Entry {
	return history.Entry{Label: label, ActionType: history.ActionTypeCopy}
}

func TestCtrlXDeletesSelectedHistoryEntry(t *testing.T) {
	h := history.New()
	h.Append(copyHistoryEntry("alpha"))
	h.Append(copyHistoryEntry("beta"))
	h.Append(copyHistoryEntry("gamma")) // most-recent first: gamma, beta, alpha
	m := NewModel(ModelConfig{History: h})

	// Empty input shows history rows; select the middle one (beta).
	m.recomputeResults()
	if len(m.results) != 3 {
		t.Fatalf("expected 3 history rows, got %d", len(m.results))
	}
	m.selected = 1
	if got := m.results[1].Action.Target; got != "beta" {
		t.Fatalf("expected selected row beta, got %q", got)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)

	for _, e := range h.Entries() {
		if e.Label == "beta" {
			t.Fatal("expected beta removed from history")
		}
	}
	if len(m.results) != 2 {
		t.Errorf("expected 2 rows after delete, got %d", len(m.results))
	}
	if m.selected < 0 || m.selected >= len(m.results) {
		t.Errorf("selection out of bounds: %d", m.selected)
	}
}

func TestCtrlXIgnoredForNonHistoryRow(t *testing.T) {
	h := history.New()
	h.Append(copyHistoryEntry("noop"))
	m := NewModel(ModelConfig{
		Providers: []Provider{CalcProvider{}},
		History:   h,
	})

	// Typing a calc query shows a calc result, not a history row.
	m = typeText(t, m, "1+1")
	if len(m.results) == 0 || m.results[0].Action.Type != ActionCopy {
		t.Fatalf("expected a calc (copy) result")
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)

	if h.Len() != 1 {
		t.Errorf("expected history untouched, len=%d", h.Len())
	}
}

// launchHistoryActionType mirrors ActionLaunch's iota value for building
// history.Entry values without importing history into a cycle (history
// itself deliberately doesn't import launcher).
const launchHistoryActionType = int(ActionLaunch)

func TestCtrlXRemovesCorrectEntry_whenLabelsCollide(t *testing.T) {
	h := history.New()
	h.Append(history.Entry{Label: "Same Name", ActionType: launchHistoryActionType, Target: "/Applications/a.app"})
	h.Append(history.Entry{Label: "Same Name", ActionType: launchHistoryActionType, Target: "/Applications/b.app"})
	m := NewModel(ModelConfig{History: h})

	m.recomputeResults()
	if len(m.results) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(m.results))
	}
	// Most-recent first: the b.app entry is selected by default (index 0).
	m.selected = 0
	if got := m.results[0].HistoryEntry.Target; got != "/Applications/b.app" {
		t.Fatalf("expected selected row to target b.app, got %q", got)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)

	entries := h.Entries()
	if len(entries) != 1 || entries[0].Target != "/Applications/a.app" {
		t.Fatalf("expected only the a.app entry to remain, got %+v", entries)
	}
}

func TestEnterOnLaunchHistoryRow_directFires(t *testing.T) {
	h := history.New()
	h.Append(history.Entry{Label: "Kitty", ActionType: launchHistoryActionType, Target: "/Applications/kitty.app"})
	launched := ""
	m := NewModel(ModelConfig{
		History:      h,
		LaunchApp:    func(path string) error { launched = path; return nil },
		HideTerminal: func() error { return nil },
		HideDelay:    0,
	})

	m.recomputeResults()
	if len(m.results) != 1 {
		t.Fatalf("expected 1 history row, got %d", len(m.results))
	}
	// Direct-fire happens without an intermediate populate-and-recompute step:
	// the input must stay empty right up to the point act() executes.
	if m.input.Value() != "" {
		t.Fatalf("expected empty input before firing, got %q", m.input.Value())
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	runCmd(cmd)

	if launched != "/Applications/kitty.app" {
		t.Fatalf("expected direct-fire launch of kitty.app, got %q", launched)
	}
}

func TestEnterOnCopyHistoryRow_populatesAndRecomputes(t *testing.T) {
	h := history.New()
	h.Append(copyHistoryEntry("1+1"))
	m := NewModel(ModelConfig{
		Providers: []Provider{CalcProvider{}},
		History:   h,
	})

	m.recomputeResults()
	if len(m.results) != 1 {
		t.Fatalf("expected 1 history row, got %d", len(m.results))
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	runCmd(cmd)

	// Regression: copy-type history rows must still populate the input and
	// recompute (calc result for "1+1"), not direct-fire.
	if m.input.Value() != "1+1" {
		t.Fatalf("expected input populated with %q, got %q", "1+1", m.input.Value())
	}
	if len(m.results) == 0 || m.results[0].Action.Type != ActionCopy {
		t.Fatalf("expected recomputed calc result, got %+v", m.results)
	}
}

func TestHistoryHint_suppressedForLaunchEntries(t *testing.T) {
	h := history.New()
	h.Append(history.Entry{Label: "Kitty", ActionType: launchHistoryActionType, Target: "/Applications/kitty.app"})
	h.Append(copyHistoryEntry("1+1"))
	m := NewModel(ModelConfig{
		Providers: []Provider{CalcProvider{}},
		History:   h,
	})

	m.recomputeResults()
	if len(m.results) != 2 {
		t.Fatalf("expected 2 history rows, got %d", len(m.results))
	}
	for _, r := range m.results {
		if r.Title == "Kitty" && r.Subtitle != "" {
			t.Errorf("expected no hint on launch-type row, got %q", r.Subtitle)
		}
		if r.Title == "1+1" && r.Subtitle == "" {
			t.Error("expected a computed hint on the copy-type row")
		}
	}
}

func TestHistoryRow_launchEntry_reDerivesIconAndSubtitleFromProvider(t *testing.T) {
	h := history.New()
	h.Append(history.Entry{Label: "Kitty", ActionType: launchHistoryActionType, Target: "/Applications/kitty.app"})

	appsProvider := NewAppsProvider(1)
	appsProvider.SetIndex(&apps.Index{Apps: []apps.App{
		{Name: "Kitty", Path: "/Applications/kitty.app", Subtitle: "Utilities"},
	}})

	m := NewModel(ModelConfig{
		Providers: []Provider{appsProvider},
		History:   h,
	})
	m.recomputeResults()

	if len(m.results) != 1 {
		t.Fatalf("expected 1 history row, got %d", len(m.results))
	}
	r := m.results[0]
	if r.Subtitle != "Utilities" {
		t.Errorf("expected subtitle re-derived from AppsProvider, got %q", r.Subtitle)
	}
	if r.Icon != IconRoleApp {
		t.Errorf("expected Icon re-derived as IconRoleApp, got %v", r.Icon)
	}
}

func TestHistoryRow_launchEntry_fallsBackToGenericIcon_whenAppNoLongerFound(t *testing.T) {
	h := history.New()
	h.Append(history.Entry{Label: "Removed App", ActionType: launchHistoryActionType, Target: "/Applications/removed.app"})

	appsProvider := NewAppsProvider(1)
	appsProvider.SetIndex(&apps.Index{}) // app no longer indexed (removed/renamed)

	m := NewModel(ModelConfig{
		Providers: []Provider{appsProvider},
		History:   h,
	})
	m.recomputeResults()

	if len(m.results) != 1 {
		t.Fatalf("expected 1 history row, got %d", len(m.results))
	}
	r := m.results[0]
	if r.Icon != IconRoleHistory {
		t.Errorf("expected fallback to IconRoleHistory, got %v", r.Icon)
	}
	if r.Subtitle != "" {
		t.Errorf("expected no subtitle when app can't be found, got %q", r.Subtitle)
	}
}

func TestCtrlRCtrlF_populateInputOnly_noDirectFire(t *testing.T) {
	h := history.New()
	h.Append(history.Entry{Label: "Kitty", ActionType: launchHistoryActionType, Target: "/Applications/kitty.app"})
	launched := ""
	m := NewModel(ModelConfig{
		History:   h,
		LaunchApp: func(path string) error { launched = path; return nil },
	})

	next, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m = next.(Model)

	if m.input.Value() != "Kitty" {
		t.Fatalf("expected input populated with %q, got %q", "Kitty", m.input.Value())
	}
	if launched != "" {
		t.Fatalf("expected ctrl+r to not direct-fire, but launched %q", launched)
	}
}

func TestResetShowsRecentHistory(t *testing.T) {
	h := history.New()
	h.Append(copyHistoryEntry("alpha"))
	h.Append(copyHistoryEntry("beta"))
	m := NewModel(ModelConfig{
		Providers:    []Provider{CalcProvider{}},
		History:      h,
		HideTerminal: func() error { return nil },
		HideDelay:    0,
	})

	// Type a query, then dismiss. The recent list must be ready for the next show,
	// since reopening the quick terminal emits no WindowSizeMsg to trigger a recompute.
	m = typeText(t, m, "1+1")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = next.(Model)

	// esc records the calc query into history, so 2 seeded + 1 = 3 recent rows.
	if len(m.results) != 3 {
		t.Fatalf("expected 3 recent rows after reset, got %d", len(m.results))
	}
	if m.results[0].Action.Type != ActionRecall {
		t.Errorf("expected recall rows, got %v", m.results[0].Action.Type)
	}
}

func TestResetAndHideNilHideTerminal(t *testing.T) {
	m := NewModel(ModelConfig{HideDelay: 0})
	m.input.SetValue("stale")
	m.status = "oops"

	cmd := m.resetAndHide()

	if m.input.Value() != "" {
		t.Errorf("expected input reset, got %q", m.input.Value())
	}
	if m.status != "" {
		t.Errorf("expected status cleared, got %q", m.status)
	}
	if cmd != nil {
		t.Error("expected nil cmd when HideTerminal is unset")
	}
}

func TestPickingNonFirstResultRecordsLearnedRank(t *testing.T) {
	lr := learnedrank.New()
	m := NewModel(ModelConfig{
		Providers:   []Provider{fakeProvider{}},
		CopyText:    func(string) error { return nil },
		LearnedRank: lr,
	})

	m = typeText(t, m, "ab")
	if len(m.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(m.results))
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(Model)
	if m.selected != 1 {
		t.Fatalf("expected selected=1 after down, got %d", m.selected)
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	runCmd(cmd)

	counts := lr.Counts("ab")
	key := Action{Type: ActionCopy, Target: "second"}.Key()
	if counts[key] != 1 {
		t.Fatalf("expected learned-rank count 1 for %q, got %d", key, counts[key])
	}
}

func TestPickingFirstResultDoesNotRecordLearnedRank(t *testing.T) {
	lr := learnedrank.New()
	m := NewModel(ModelConfig{
		Providers:   []Provider{fakeProvider{}},
		CopyText:    func(string) error { return nil },
		LearnedRank: lr,
	})

	m = typeText(t, m, "ab")
	if len(m.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(m.results))
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	runCmd(cmd)

	counts := lr.Counts("ab")
	if len(counts) != 0 {
		t.Fatalf("expected no learned-rank entries, got %v", counts)
	}
}

func TestLearnedRankRePicksSameQueryToTop(t *testing.T) {
	lr := learnedrank.New()
	m := NewModel(ModelConfig{
		Providers:   []Provider{fakeProvider{}},
		CopyText:    func(string) error { return nil },
		LearnedRank: lr,
	})

	m = typeText(t, m, "ab")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(Model)
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	runCmd(cmd)

	// esc/enter reset the input; re-type the exact same query.
	m = typeText(t, m, "ab")
	if len(m.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(m.results))
	}
	if got := m.results[0].Action.Target; got != "second" {
		t.Fatalf("expected previously non-first pick ranked first, got %q", got)
	}
}

func TestLearnedRankDoesNotLeakAcrossQueries(t *testing.T) {
	lr := learnedrank.New()
	m := NewModel(ModelConfig{
		Providers:   []Provider{fakeProvider{}},
		CopyText:    func(string) error { return nil },
		LearnedRank: lr,
	})

	m = typeText(t, m, "ab")
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(Model)
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	runCmd(cmd)

	// A different query text should not inherit the learned rank from "ab".
	m = typeText(t, m, "cd")
	if len(m.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(m.results))
	}
	if got := m.results[0].Action.Target; got != "first" {
		t.Fatalf("expected unaffected default ordering for a different query, got %q", got)
	}
}

func TestEnterOnReloadCommand_matchesCtrlShiftR(t *testing.T) {
	idx := &apps.Index{Apps: []apps.App{{Name: "TestApp", Path: "/Applications/TestApp.app"}}}
	appsProvider := NewAppsProvider(1.0)

	reloadCmd := commands.Command{
		Name: "reload",
		Run: func() tea.Cmd {
			return func() tea.Msg { return AppsReindexedMsg{Index: idx} }
		},
	}
	commandsProvider := NewCommandsProvider([]commands.Command{reloadCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		AppsProvider:     appsProvider,
		CommandsProvider: commandsProvider,
		HideDelay:        0,
	})

	m = typeText(t, m, "reload")
	if len(m.results) == 0 {
		t.Fatal("expected 'reload' command result")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if m.status != "running…" {
		t.Errorf("status = %q, want %q", m.status, "running…")
	}

	msg := runCmd(cmd)
	next, _ = m.Update(msg)
	m = next.(Model)

	if appsProvider.index != idx {
		t.Error("expected reload command to update AppsProvider's index, same as ctrl+shift+r")
	}
	if m.status != "reindexed 1 apps" {
		t.Errorf("expected transient reindexed status, got %q", m.status)
	}

	next, _ = m.Update(clearStatusMsg{})
	m = next.(Model)
	if m.status != "" {
		t.Errorf("expected reindexed status cleared after clearStatusMsg, got %q", m.status)
	}
}

func TestEnterOnCleanURLCommand_success_notifiesAndHides(t *testing.T) {
	hidden := false
	var notifyTitle, notifyMessage string

	cleanURLCmd := commands.Command{
		Name: "cleanurl",
		Run: func() tea.Cmd {
			return func() tea.Msg { return CleanURLDoneMsg{} }
		},
	}
	commandsProvider := NewCommandsProvider([]commands.Command{cleanURLCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		CommandsProvider: commandsProvider,
		HideTerminal:     func() error { hidden = true; return nil },
		ShowNotification: func(title, message string) error {
			notifyTitle = title
			notifyMessage = message
			return nil
		},
		HideDelay: 0,
	})

	m = typeText(t, m, "cleanurl")
	if len(m.results) == 0 {
		t.Fatal("expected 'cleanurl' command result")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	msg := runCmd(cmd)
	next, hideCmd := m.Update(msg)
	m = next.(Model)

	if notifyTitle == "" || notifyMessage == "" {
		t.Errorf("expected ShowNotification to be called with a title/message, got %q/%q", notifyTitle, notifyMessage)
	}

	runCmd(hideCmd)
	if !hidden {
		t.Error("expected HideTerminal to be called after a successful cleanurl")
	}
}

func TestEnterOnCleanURLCommand_failure_showsStatusAndStaysOpen(t *testing.T) {
	hidden := false
	cleanURLCmd := commands.Command{
		Name: "cleanurl",
		Run: func() tea.Cmd {
			return func() tea.Msg { return CleanURLDoneMsg{Err: errors.New("clipboard empty")} }
		},
	}
	commandsProvider := NewCommandsProvider([]commands.Command{cleanURLCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		CommandsProvider: commandsProvider,
		HideTerminal:     func() error { hidden = true; return nil },
		HideDelay:        0,
	})

	m = typeText(t, m, "cleanurl")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	msg := runCmd(cmd)
	next, hideCmd := m.Update(msg)
	m = next.(Model)

	if m.status != "cleanurl error: clipboard empty" {
		t.Errorf("status = %q, want %q", m.status, "cleanurl error: clipboard empty")
	}

	runCmd(hideCmd)
	if hidden {
		t.Error("expected launcher to stay open after a cleanurl error")
	}
}

func TestModelConfigUsesInjectedReadClipboard(t *testing.T) {
	m := NewModel(ModelConfig{
		Providers:     []Provider{CalcProvider{}},
		ReadClipboard: func() (string, error) { return "fake clipboard text", nil },
		HideDelay:     0,
	})

	text, err := m.cfg.ReadClipboard()
	if err != nil {
		t.Fatalf("ReadClipboard() error = %v", err)
	}
	if text != "fake clipboard text" {
		t.Errorf("ReadClipboard() = %q, want %q", text, "fake clipboard text")
	}
}

// commandEnteringAIPromptMode builds a commands.Command that, when run,
// emits EnterAIPromptModeMsg for kind — the same shape the real ai/improve
// commands' Run funcs use (they can't mutate the model directly).
func commandEnteringAIPromptMode(name string, kind AIPromptKind) commands.Command {
	return commands.Command{
		Name: name,
		Run: func() tea.Cmd {
			return func() tea.Msg { return EnterAIPromptModeMsg{Kind: kind} }
		},
	}
}

func TestPickingAICommand_entersAIPromptModeSetsChromeAndRecordsHistory(t *testing.T) {
	aiCmd := commandEnteringAIPromptMode("ai", AIPromptKindAI)
	commandsProvider := NewCommandsProvider([]commands.Command{aiCmd}, 1.0)
	hist := history.New()

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		CommandsProvider: commandsProvider,
		History:          hist,
		HideDelay:        0,
	})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	m = typeText(t, m, "ai")
	if len(m.results) == 0 {
		t.Fatal("expected 'ai' command result")
	}

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	msg := runCmd(cmd)
	next, _ = m.Update(msg)
	m = next.(Model)

	if m.aiPromptKind != AIPromptKindAI {
		t.Fatalf("aiPromptKind = %q, want %q", m.aiPromptKind, AIPromptKindAI)
	}

	view := m.widget.View()
	if !strings.Contains(view, "ai") {
		t.Errorf("expected widget prompt to name the kind, view:\n%s", view)
	}
	if !strings.Contains(view, aiPromptModeLegend) {
		t.Errorf("expected footer to show the key legend %q, view:\n%s", aiPromptModeLegend, view)
	}

	if hist.Len() != 1 || hist.Entries()[0].Label != "ai" {
		t.Errorf("expected a history entry for the 'ai' command, got entries=%v", hist.Entries())
	}
}

func TestPickingImproveCommand_entersAIPromptModeWithImproveKind(t *testing.T) {
	improveCmd := commandEnteringAIPromptMode("improve", AIPromptKindImprove)
	commandsProvider := NewCommandsProvider([]commands.Command{improveCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		CommandsProvider: commandsProvider,
		HideDelay:        0,
	})
	sized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = sized.(Model)

	m = typeText(t, m, "improve")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(runCmd(cmd))
	m = next.(Model)

	if m.aiPromptKind != AIPromptKindImprove {
		t.Fatalf("aiPromptKind = %q, want %q", m.aiPromptKind, AIPromptKindImprove)
	}
	if !strings.Contains(m.widget.View(), "improve") {
		t.Errorf("expected widget prompt to name the kind, view:\n%s", m.widget.View())
	}
}

func TestAIPromptMode_navigationSwallowedAndResultsNotRecomputed(t *testing.T) {
	aiCmd := commandEnteringAIPromptMode("ai", AIPromptKindAI)
	commandsProvider := NewCommandsProvider([]commands.Command{aiCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider, fakeProvider{}},
		CommandsProvider: commandsProvider,
		HideDelay:        0,
	})

	m = typeText(t, m, "ai")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(runCmd(cmd))
	m = next.(Model)

	resultsBefore := m.results
	selectedBefore := m.selected

	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg{Code: tea.KeyUp},
		tea.KeyPressMsg{Code: tea.KeyDown},
	} {
		next, _ = m.Update(key)
		m = next.(Model)
	}
	m = typeText(t, m, "prompt text")

	if len(m.results) != len(resultsBefore) {
		t.Errorf("expected results not to be recomputed while in mode, got %d results, want %d", len(m.results), len(resultsBefore))
	}
	if m.selected != selectedBefore {
		t.Errorf("expected navigation to be swallowed while in mode, selected = %d, want %d", m.selected, selectedBefore)
	}
}

func TestAIPromptMode_escExitsModeAndLeavesLauncherVisible(t *testing.T) {
	hidden := false
	aiCmd := commandEnteringAIPromptMode("ai", AIPromptKindAI)
	commandsProvider := NewCommandsProvider([]commands.Command{aiCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		CommandsProvider: commandsProvider,
		HideTerminal:     func() error { hidden = true; return nil },
		HideDelay:        0,
	})

	m = typeText(t, m, "ai")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(runCmd(cmd))
	m = next.(Model)

	next, escCmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = next.(Model)

	if m.aiPromptKind != "" {
		t.Errorf("aiPromptKind = %q, want empty after esc", m.aiPromptKind)
	}
	runCmd(escCmd)
	if hidden {
		t.Error("expected esc to leave the launcher visible, but HideTerminal was called")
	}
}

func TestAIPromptMode_backgroundMessageLeavesFooterLegendIntact(t *testing.T) {
	aiCmd := commandEnteringAIPromptMode("ai", AIPromptKindAI)
	commandsProvider := NewCommandsProvider([]commands.Command{aiCmd}, 1.0)

	m := NewModel(ModelConfig{
		Providers:        []Provider{commandsProvider},
		CommandsProvider: commandsProvider,
		HideDelay:        0,
	})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	m = typeText(t, m, "ai")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(runCmd(cmd))
	m = next.(Model)

	// A background handler (apps reindex) that knows nothing about modes
	// must not clobber the mode's key legend with its own status text.
	next, _ = m.Update(AppsReindexedMsg{Err: errors.New("boom")})
	m = next.(Model)

	view := m.widget.View()
	if !strings.Contains(view, aiPromptModeLegend) {
		t.Errorf("expected footer to still show the key legend after a background message, view:\n%s", view)
	}
	if strings.Contains(view, "reindex error") {
		t.Errorf("expected the background message's status not to reach the footer while in mode, view:\n%s", view)
	}
}

// enterAIMode drives m into ai prompt mode via the "ai" command, as the
// picking tests above do, and returns the updated model.
func enterAIMode(t *testing.T, cfg ModelConfig) Model {
	t.Helper()
	aiCmd := commandEnteringAIPromptMode("ai", AIPromptKindAI)
	commandsProvider := NewCommandsProvider([]commands.Command{aiCmd}, 1.0)
	cfg.Providers = append([]Provider{commandsProvider}, cfg.Providers...)
	cfg.CommandsProvider = commandsProvider

	m := NewModel(cfg)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	m = typeText(t, m, "ai")
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(runCmd(cmd))
	return next.(Model)
}

func TestAIPromptMode_entryReadsClipboardOnceAndShowsPreview(t *testing.T) {
	reads := 0
	m := enterAIMode(t, ModelConfig{
		ReadClipboard: func() (string, error) {
			reads++
			return "line one\nline two", nil
		},
		HideDelay: 0,
	})

	if reads != 1 {
		t.Fatalf("expected clipboard read exactly once at mode entry, got %d reads", reads)
	}

	view := m.widget.View()
	if !strings.Contains(view, clipboardPreviewHeader) {
		t.Errorf("expected preview header in view, got:\n%s", view)
	}
	if !strings.Contains(view, "line one") || !strings.Contains(view, "line two") {
		t.Errorf("expected clipboard contents in view, got:\n%s", view)
	}

	// Pressing keys that don't change the query must not re-read the clipboard.
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = next.(Model)
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(Model)
	if reads != 1 {
		t.Errorf("expected clipboard still read exactly once, got %d reads", reads)
	}
}

func TestAIPromptMode_firstTypedCharRemovesPreview(t *testing.T) {
	m := enterAIMode(t, ModelConfig{
		ReadClipboard: func() (string, error) { return "clip text", nil },
		HideDelay:     0,
	})

	if !strings.Contains(m.widget.View(), clipboardPreviewHeader) {
		t.Fatalf("expected preview before typing, view:\n%s", m.widget.View())
	}

	m = typeText(t, m, "x")

	view := m.widget.View()
	if strings.Contains(view, clipboardPreviewHeader) {
		t.Errorf("expected preview removed after first typed char, view:\n%s", view)
	}
	if strings.Contains(view, "clip text") {
		t.Errorf("expected clipboard contents removed after first typed char, view:\n%s", view)
	}
}

func TestAIPromptMode_readsClipboardExactlyOnceAcrossManyKeystrokes(t *testing.T) {
	reads := 0
	m := enterAIMode(t, ModelConfig{
		ReadClipboard: func() (string, error) {
			reads++
			return "clip text", nil
		},
		HideDelay: 0,
	})

	m = typeText(t, m, "hello world")
	for range 5 {
		next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		m = next.(Model)
	}

	if reads != 1 {
		t.Errorf("expected clipboard read exactly once regardless of keystrokes, got %d reads", reads)
	}
}

func TestAIPromptMode_noPreviewLineCanBeSelected(t *testing.T) {
	m := enterAIMode(t, ModelConfig{
		ReadClipboard: func() (string, error) { return "a\nb\nc", nil },
		HideDelay:     0,
	})

	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg{Code: tea.KeyUp},
		tea.KeyPressMsg{Code: tea.KeyDown},
		tea.KeyPressMsg{Code: tea.KeyDown},
	} {
		next, _ := m.Update(key)
		m = next.(Model)
	}

	if m.widget.Selected() != 0 {
		t.Errorf("expected selection to stay pinned at 0 while previewing, got %d", m.widget.Selected())
	}
}

// runBatch executes cmd, recursively flattening a tea.BatchMsg, and returns
// the resulting messages in the order the runtime would deliver them.
func runBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runBatch(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// findAIRunDoneMsg returns the first AIRunDoneMsg among msgs.
func findAIRunDoneMsg(t *testing.T, msgs []tea.Msg) AIRunDoneMsg {
	t.Helper()
	for _, msg := range msgs {
		if done, ok := msg.(AIRunDoneMsg); ok {
			return done
		}
	}
	t.Fatalf("expected an AIRunDoneMsg among %d messages", len(msgs))
	return AIRunDoneMsg{}
}

func TestAIPromptMode_enterDispatchesTypedInput(t *testing.T) {
	hidden := false
	var gotModel string
	var gotInput string
	fakeExec := func(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
		gotModel = args[argAfter(args, "--model")]
		b, _ := io.ReadAll(stdin)
		gotInput = string(b)
		return []byte("response"), nil, nil
	}

	m := enterAIMode(t, ModelConfig{
		HideTerminal: func() error { hidden = true; return nil },
		HideDelay:    0,
		AIExec:       fakeExec,
		AIModel:      "haiku",
		AITimeout:    time.Second,
	})

	m = typeText(t, m, "hello there")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.aiPromptKind != "" {
		t.Errorf("expected ai prompt mode to be exited after dispatch, got %q", m.aiPromptKind)
	}
	if m.input.Value() != "" {
		t.Errorf("expected input reset after dispatch, got %q", m.input.Value())
	}

	msgs := runBatch(cmd)
	if !hidden {
		t.Error("expected dispatch to take the reset-and-hide path")
	}

	done := findAIRunDoneMsg(t, msgs)
	if done.Kind != AIPromptKindAI {
		t.Errorf("Kind = %q, want %q", done.Kind, AIPromptKindAI)
	}
	if done.Input != "hello there" {
		t.Errorf("Input = %q, want %q", done.Input, "hello there")
	}
	if done.Result.Status != ai.StatusSuccess || done.Result.Response != "response" {
		t.Errorf("Result = %+v, want success/\"response\"", done.Result)
	}
	if gotInput != "hello there" {
		t.Errorf("exec stdin = %q, want %q", gotInput, "hello there")
	}
	if gotModel != "haiku" {
		t.Errorf("exec --model = %q, want %q", gotModel, "haiku")
	}
}

// argAfter returns the index of the arg following name in args, for reading
// a "--flag value" pair out of an argv slice.
func argAfter(args []string, name string) int {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return i + 1
		}
	}
	return -1
}

func TestAIPromptMode_enterWithEmptyInputDispatchesClipboardSnapshot(t *testing.T) {
	var gotInput string
	fakeExec := func(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
		b, _ := io.ReadAll(stdin)
		gotInput = string(b)
		return []byte("ok"), nil, nil
	}

	m := enterAIMode(t, ModelConfig{
		ReadClipboard: func() (string, error) { return "clipboard text", nil },
		HideDelay:     0,
		AIExec:        fakeExec,
	})

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	msgs := runBatch(cmd)

	done := findAIRunDoneMsg(t, msgs)
	if done.Input != "clipboard text" {
		t.Errorf("Input = %q, want %q", done.Input, "clipboard text")
	}
	if gotInput != "clipboard text" {
		t.Errorf("exec stdin = %q, want %q", gotInput, "clipboard text")
	}
}

func TestAIPromptMode_clipboardChangeAfterEntryDoesNotAffectDispatch(t *testing.T) {
	clip := "original clipboard"
	var gotInput string
	fakeExec := func(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
		b, _ := io.ReadAll(stdin)
		gotInput = string(b)
		return []byte("ok"), nil, nil
	}

	m := enterAIMode(t, ModelConfig{
		ReadClipboard: func() (string, error) { return clip, nil },
		HideDelay:     0,
		AIExec:        fakeExec,
	})

	clip = "changed after entry"

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	msgs := runBatch(cmd)

	done := findAIRunDoneMsg(t, msgs)
	if done.Input != "original clipboard" {
		t.Errorf("Input = %q, want the snapshot taken at mode entry %q", done.Input, "original clipboard")
	}
	if gotInput != "original clipboard" {
		t.Errorf("exec stdin = %q, want %q", gotInput, "original clipboard")
	}
}

func TestAIPromptMode_enterWithNothingToSendShowsInlineErrorAndStays(t *testing.T) {
	execCalled := false
	fakeExec := func(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
		execCalled = true
		return []byte("ok"), nil, nil
	}

	m := enterAIMode(t, ModelConfig{
		HideDelay: 0,
		AIExec:    fakeExec,
	})

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if cmd != nil {
		msgs := runBatch(cmd)
		for _, msg := range msgs {
			if _, ok := msg.(AIRunDoneMsg); ok {
				t.Fatalf("expected no ai run dispatched with nothing to send")
			}
		}
	}
	if execCalled {
		t.Error("expected exec not to be called with nothing to send")
	}
	if m.aiPromptKind == "" {
		t.Error("expected to stay in ai prompt mode")
	}
	if m.input.Value() != "" {
		t.Errorf("expected the cursor/input left intact, got %q", m.input.Value())
	}
	if m.aiPromptError == "" {
		t.Error("expected an inline error to be set")
	}
	if !strings.Contains(m.widget.View(), m.aiPromptError) {
		t.Errorf("expected the inline error in the view, got:\n%s", m.widget.View())
	}
}

func TestAIPromptMode_inlineErrorClearsOnNextKeystroke(t *testing.T) {
	m := enterAIMode(t, ModelConfig{HideDelay: 0})

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if m.aiPromptError == "" {
		t.Fatal("expected an inline error to be set before typing")
	}

	m = typeText(t, m, "x")

	if m.aiPromptError != "" {
		t.Errorf("expected the inline error cleared after the next keystroke, got %q", m.aiPromptError)
	}
	if strings.Contains(m.widget.View(), "nothing to send") {
		t.Errorf("expected the error no longer shown in the view, got:\n%s", m.widget.View())
	}
	if !strings.Contains(m.widget.View(), aiPromptModeLegend) {
		t.Errorf("expected the footer to show the legend again, got:\n%s", m.widget.View())
	}
}

func TestAIPromptMode_concurrentRunsAreIndependent(t *testing.T) {
	release := make(chan struct{})
	var wg sync.WaitGroup

	slowExec := func(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
		<-release
		return []byte("slow"), nil, nil
	}
	fastExec := func(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
		return []byte("fast"), nil, nil
	}

	slowModel := enterAIMode(t, ModelConfig{HideDelay: 0, AIExec: slowExec})
	slowModel = typeText(t, slowModel, "slow one")
	_, slowCmd := slowModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	fastModel := enterAIMode(t, ModelConfig{HideDelay: 0, AIExec: fastExec})
	fastModel = typeText(t, fastModel, "fast one")
	_, fastCmd := fastModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	wg.Go(func() {
		runBatch(slowCmd)
	})

	fastMsgs := make(chan []tea.Msg, 1)
	wg.Go(func() {
		fastMsgs <- runBatch(fastCmd)
	})

	select {
	case msgs := <-fastMsgs:
		done := findAIRunDoneMsg(t, msgs)
		if done.Result.Response != "fast" {
			t.Errorf("Response = %q, want %q", done.Result.Response, "fast")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the fast run to complete without waiting for the slow run")
	}

	close(release)
	wg.Wait()
}

func TestAIRunDone_successAppendsCopiesAndNotifies(t *testing.T) {
	store := ai.NewStore()
	var copied string
	var notifyTitle, notifyMessage string

	m := NewModel(ModelConfig{
		AIRunsStore: store,
		CopyText:    func(s string) error { copied = s; return nil },
		ShowNotification: func(title, message string) error {
			notifyTitle = title
			notifyMessage = message
			return nil
		},
	})

	next, _ := m.Update(AIRunDoneMsg{
		Kind:  AIPromptKindAI,
		Input: "hello",
		Result: ai.InvokeResult{
			Status:   ai.StatusSuccess,
			Response: "line one\nline two",
		},
	})
	m = next.(Model)

	runs := store.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run in the store, got %d", len(runs))
	}
	run := runs[0]
	if run.ID == "" {
		t.Error("expected a non-empty run id")
	}
	if run.Kind != ai.KindAI || run.Input != "hello" || run.Response != "line one\nline two" || run.Status != ai.StatusSuccess {
		t.Errorf("unexpected run: %+v", run)
	}

	if copied != "line one\nline two" {
		t.Errorf("copied = %q, want the full response", copied)
	}
	if notifyTitle != "ai" {
		t.Errorf("notifyTitle = %q, want %q", notifyTitle, "ai")
	}
	if notifyMessage != "line one\nline two" {
		t.Errorf("notifyMessage = %q, want the full multi-line response", notifyMessage)
	}
}

func TestAIRunDone_failureAppendsNotifiesAndLeavesClipboard(t *testing.T) {
	store := ai.NewStore()
	copyCalled := false
	var notifyTitle, notifyMessage string

	m := NewModel(ModelConfig{
		AIRunsStore: store,
		CopyText:    func(s string) error { copyCalled = true; return nil },
		ShowNotification: func(title, message string) error {
			notifyTitle = title
			notifyMessage = message
			return nil
		},
	})

	next, _ := m.Update(AIRunDoneMsg{
		Kind:  AIPromptKindImprove,
		Input: "hello",
		Result: ai.InvokeResult{
			Status: ai.StatusFailure,
			Err:    errors.New("boom"),
		},
	})
	m = next.(Model)

	runs := store.Runs()
	if len(runs) != 1 || runs[0].Status != ai.StatusFailure {
		t.Fatalf("expected 1 failed run in the store, got %+v", runs)
	}

	if copyCalled {
		t.Error("expected the clipboard left untouched on failure")
	}
	if !strings.Contains(notifyTitle, "improve") {
		t.Errorf("notifyTitle = %q, want it to name the kind", notifyTitle)
	}
	if notifyTitle == "improve" {
		t.Errorf("notifyTitle = %q, want a failure marker distinct from the success title", notifyTitle)
	}
	if notifyMessage != "boom" {
		t.Errorf("notifyMessage = %q, want the error", notifyMessage)
	}
}

func TestAIRunDone_noLauncherHistoryEntry(t *testing.T) {
	h := history.New()
	m := NewModel(ModelConfig{
		AIRunsStore: ai.NewStore(),
		History:     h,
	})

	next, _ := m.Update(AIRunDoneMsg{
		Kind: AIPromptKindAI,
		Result: ai.InvokeResult{
			Status:   ai.StatusSuccess,
			Response: "r",
		},
	})
	m = next.(Model)

	next, _ = m.Update(AIRunDoneMsg{
		Kind: AIPromptKindAI,
		Result: ai.InvokeResult{
			Status: ai.StatusFailure,
			Err:    errors.New("boom"),
		},
	})
	m = next.(Model)

	if h.Len() != 0 {
		t.Errorf("expected no launcher-history entries, got %d", h.Len())
	}
}

func TestAIRunDone_recomputesResultsOnlyWhenInputEmpty(t *testing.T) {
	m := NewModel(ModelConfig{Providers: []Provider{fakeProvider{}}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	// Empty input: completion must recompute, replacing whatever was left over.
	m.results = []Result{{Title: "stale"}}
	next, _ = m.Update(AIRunDoneMsg{
		Kind:   AIPromptKindAI,
		Result: ai.InvokeResult{Status: ai.StatusSuccess, Response: "r"},
	})
	m = next.(Model)
	if len(m.results) == 1 && m.results[0].Title == "stale" {
		t.Errorf("expected results recomputed on completion with empty input, got %+v", m.results)
	}

	// Non-empty input: completion must leave the in-progress results alone.
	m = typeText(t, m, "query")
	m.results = []Result{{Title: "sentinel"}}
	next, _ = m.Update(AIRunDoneMsg{
		Kind:   AIPromptKindAI,
		Result: ai.InvokeResult{Status: ai.StatusSuccess, Response: "r"},
	})
	m = next.(Model)
	if len(m.results) != 1 || m.results[0].Title != "sentinel" {
		t.Errorf("expected results left alone on completion with typed input, got %+v", m.results)
	}
}
