package launcher

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
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
