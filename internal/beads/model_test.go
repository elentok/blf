package beads

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stubLister is a fake IssueLister for model tests, so they don't shell out
// to bd.
type stubLister struct {
	issues []Issue
	err    error
	ready  map[string]bool
}

func (s stubLister) List(scope Scope) ([]Issue, error) {
	return s.issues, s.err
}

func (s stubLister) Ready() (map[string]bool, error) {
	return s.ready, nil
}

type stubMutator struct {
	createdTitle  string
	createdOpts   CreateOptions
	updatedID     string
	updatedStatus string
	closedID      string
	reopenedID    string
	createResult  Issue
	updateResult  Issue
	closeResult   Issue
	reopenResult  Issue
}

func (s *stubMutator) Create(title string, opts CreateOptions) (Issue, error) {
	s.createdTitle = title
	s.createdOpts = opts
	if s.createResult.ID != "" {
		return s.createResult, nil
	}
	return Issue{ID: "new-1", Title: title, Status: "open", IssueType: "task"}, nil
}

func (s *stubMutator) UpdateStatus(id, status string) (Issue, error) {
	s.updatedID = id
	s.updatedStatus = status
	if s.updateResult.ID != "" {
		return s.updateResult, nil
	}
	return Issue{ID: id, Status: status}, nil
}

func (s *stubMutator) Close(id string) (Issue, error) {
	s.closedID = id
	if s.closeResult.ID != "" {
		return s.closeResult, nil
	}
	return Issue{ID: id, Status: "closed"}, nil
}

func (s *stubMutator) Reopen(id string) (Issue, error) {
	s.reopenedID = id
	if s.reopenResult.ID != "" {
		return s.reopenResult, nil
	}
	return Issue{ID: id, Status: "open"}, nil
}

// stubPreviewFetcher is a fake PreviewFetcher for model tests, so preview
// fetches don't shell out to bd.
type stubPreviewFetcher struct {
	downOf map[string][]DepTreeNode
	upOf   map[string][]DepTreeNode
}

func (s stubPreviewFetcher) DepTree(id string, direction DepDirection) ([]DepTreeNode, error) {
	if direction == DepUp {
		return s.upOf[id], nil
	}
	return s.downOf[id], nil
}

func testIssues() []Issue {
	return []Issue{
		{ID: "abc-1", Title: "fix the bug", Status: "open"},
		{ID: "abc-2", Title: "add a feature", Status: "in_progress"},
		{ID: "abc-3", Title: "refactor code", Status: "closed"},
	}
}

// modelPress sends a key to the model and returns the updated model.
func modelPress(m Model, key string) Model {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		msg = tea.KeyPressMsg{Code: tea.KeyEsc}
	case "ctrl+c":
		msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+a":
		msg = tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl}
	case "ctrl+e":
		msg = tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}
	case "ctrl+f":
		msg = tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
	case "ctrl+g":
		msg = tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl}
	case "ctrl+r":
		msg = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	case "ctrl+s":
		msg = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "ctrl+t":
		msg = tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}
	case "ctrl+x":
		msg = tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}
	case "?":
		msg = tea.KeyPressMsg{Code: '?', Text: "?"}
	case "down":
		msg = tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		msg = tea.KeyPressMsg{Code: tea.KeyUp}
	default:
		if len(key) == 1 {
			r := rune(key[0])
			msg = tea.KeyPressMsg{Code: r, Text: key}
		}
	}
	next, cmd := m.Update(msg)
	_ = cmd
	return next.(Model)
}

func modelType(m Model, query string) Model {
	for _, r := range query {
		m = modelPress(m, string(r))
	}
	return m
}

// loadIssues sizes the model and injects issues directly via issuesLoadedMsg,
// mirroring how internal/claudehistory's tests skip running Init's real cmd.
func loadIssues(m Model, issues []Issue) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	next, _ = m.Update(issuesLoadedMsg{issues: issues})
	return next.(Model)
}

func issueSet(ids ...string) []Issue {
	var issues []Issue
	for _, id := range ids {
		issues = append(issues, Issue{ID: id, Title: id, Status: "open", IssueType: "task"})
	}
	return issues
}

func TestEnterYieldsSelectedID(t *testing.T) {
	copied := ""
	m := NewModel(ModelConfig{
		Lister:   stubLister{issues: testIssues()},
		Scope:    ScopeActionable,
		CopyText: func(s string) error { copied = s; return nil },
	})
	m = loadIssues(m, testIssues())

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.SelectedID() != "abc-1" {
		t.Errorf("SelectedID() = %q, want %q", m.SelectedID(), "abc-1")
	}
	if copied != "abc-1" {
		t.Errorf("CopyText got %q, want %q", copied, "abc-1")
	}
	if cmd == nil {
		t.Error("expected a quit cmd after enter")
	}
}

func TestEnterAfterFilterYieldsFilteredSelection(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())

	m = modelType(m, "feature")
	if len(*m.displayRef) != 1 || (*m.displayRef)[0].ID != "abc-2" {
		t.Fatalf("expected filtered list to contain only abc-2, got %+v", *m.displayRef)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.SelectedID() != "abc-2" {
		t.Errorf("SelectedID() = %q, want %q", m.SelectedID(), "abc-2")
	}
}

func TestEnterOnEmptyListIsNoop(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: nil}})
	m = loadIssues(m, nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)

	if m.SelectedID() != "" {
		t.Errorf("expected no selection, got %q", m.SelectedID())
	}
	if cmd != nil {
		t.Error("expected no cmd for enter on an empty list")
	}
}

func TestEscQuitsWithoutSelection(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a quit cmd for esc")
	}
}

func TestLoadErrorIsSurfaced(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{err: errors.New("bd exploded")}})
	next, _ := m.Update(issuesLoadedMsg{err: errors.New("bd exploded")})
	m = next.(Model)

	if m.loadErr == nil {
		t.Error("expected loadErr to be set")
	}
}

func TestEmptyScopeShowsEmptyState(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: nil}})
	m = loadIssues(m, nil)

	if len(m.allItems) != 0 {
		t.Fatalf("expected no items, got %d", len(m.allItems))
	}
}

func TestCtrlFCyclesScopeAndRefetches(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}, Scope: ScopeActionable})
	m = loadIssues(m, testIssues())

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m = next.(Model)

	if m.scope != ScopeReady {
		t.Errorf("scope after one ctrl+f = %v, want %v", m.scope, ScopeReady)
	}
	if cmd == nil {
		t.Fatal("expected a re-fetch cmd after ctrl+f")
	}
}

func TestScopeCycleOrder(t *testing.T) {
	s := ScopeActionable
	want := []Scope{ScopeReady, ScopeBlocked, ScopeAll, ScopeActionable}
	for i, w := range want {
		s = nextScope(s)
		if s != w {
			t.Fatalf("step %d: nextScope = %v, want %v", i, s, w)
		}
	}
}

func TestQuestionMarkOpensHelpAndNextKeyClosesIt(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())

	m = modelPress(m, "?")
	if !m.helpMode {
		t.Fatal("expected help mode to open")
	}

	view := m.View().Content
	if !strings.Contains(view, "blf beads - help") {
		t.Fatalf("expected help view title, got %q", view)
	}

	m = modelPress(m, "down")
	if m.helpMode {
		t.Fatal("expected any key to close help mode")
	}
}

func TestCreateModeEscRestoresQuery(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())
	m = modelType(m, "feature")

	m = modelPress(m, "ctrl+a")
	if m.mode != modeCreate {
		t.Fatalf("expected create mode, got %v", m.mode)
	}
	if got := m.widget.Query(); got != "" {
		t.Fatalf("expected create mode to clear the input, got %q", got)
	}

	m = modelType(m, "new task")
	m = modelPress(m, "esc")

	if m.mode != modeBrowse {
		t.Fatalf("expected browse mode after esc, got %v", m.mode)
	}
	if got := m.widget.Query(); got != "feature" {
		t.Fatalf("expected query restored to %q, got %q", "feature", got)
	}
}

func TestCreateModeOnEpicDefaultsParentAndCtrlTTogglesStandalone(t *testing.T) {
	issues := []Issue{
		{ID: "epic-1", Title: "Epic", Status: "open", IssueType: "epic"},
		{ID: "task-1", Title: "Task", Status: "open", IssueType: "task"},
	}
	m := NewModel(ModelConfig{Lister: stubLister{issues: issues}})
	m = loadIssues(m, issues)

	m = modelPress(m, "ctrl+a")
	if m.createParentID != "epic-1" {
		t.Fatalf("expected epic parent default, got %q", m.createParentID)
	}
	if m.createStandalone {
		t.Fatal("expected create mode to default to child mode")
	}

	m = modelPress(m, "ctrl+t")
	if !m.createStandalone {
		t.Fatal("expected ctrl+t to toggle standalone on")
	}
}

func TestCreateModeEnterCreatesAndRefreshesSelection(t *testing.T) {
	mutator := &stubMutator{
		createResult: Issue{ID: "new-1", Title: "new task", Status: "open", IssueType: "task"},
	}
	initial := []Issue{
		{ID: "epic-1", Title: "Epic", Status: "open", IssueType: "epic"},
	}
	reloaded := []Issue{
		{ID: "epic-1", Title: "Epic", Status: "open", IssueType: "epic"},
		{ID: "new-1", Title: "new task", Status: "open", IssueType: "task", Parent: "epic-1"},
	}
	m := NewModel(ModelConfig{
		Lister:  stubLister{issues: initial},
		Mutator: mutator,
	})
	m = loadIssues(m, initial)

	m = modelPress(m, "ctrl+a")
	m = modelType(m, "new task")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	msg := cmd()
	next, cmd = m.Update(msg)
	m = next.(Model)

	if mutator.createdTitle != "new task" {
		t.Fatalf("created title = %q, want %q", mutator.createdTitle, "new task")
	}
	if mutator.createdOpts.Parent != "epic-1" {
		t.Fatalf("expected created issue to inherit epic parent, got %+v", mutator.createdOpts)
	}
	if cmd == nil {
		t.Fatal("expected create to trigger a reload")
	}

	next, _ = m.Update(issuesLoadedMsg{issues: reloaded})
	m = next.(Model)
	next, _ = m.Update(readyLoadedMsg{ready: map[string]bool{"epic-1": true, "new-1": true}})
	m = next.(Model)

	if m.mode != modeBrowse {
		t.Fatalf("expected browse mode after successful create, got %v", m.mode)
	}
	if m.selectedRowID() != "new-1" {
		t.Fatalf("expected new issue selected after reload, got %q", m.selectedRowID())
	}
}

func TestStatusModeEscRestoresQuery(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}})
	m = loadIssues(m, testIssues())
	m = modelType(m, "feature")

	m = modelPress(m, "ctrl+s")
	if m.mode != modeStatus {
		t.Fatalf("expected status mode, got %v", m.mode)
	}

	m = modelPress(m, "esc")
	if got := m.widget.Query(); got != "feature" {
		t.Fatalf("expected query restored to %q, got %q", "feature", got)
	}
}

func TestStatusModeEnterUpdatesSelectedIssue(t *testing.T) {
	mutator := &stubMutator{}
	issues := testIssues()
	m := NewModel(ModelConfig{Lister: stubLister{issues: issues}, Mutator: mutator})
	m = loadIssues(m, issues)
	m = modelPress(m, "down")

	m = modelPress(m, "ctrl+s")
	m = modelPress(m, "down") // in_progress -> blocked

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(Model)

	if mutator.updatedID != "abc-2" {
		t.Fatalf("updated id = %q, want %q", mutator.updatedID, "abc-2")
	}
	if mutator.updatedStatus != "blocked" {
		t.Fatalf("updated status = %q, want %q", mutator.updatedStatus, "blocked")
	}
}

func TestCtrlXClosesOpenIssue(t *testing.T) {
	mutator := &stubMutator{}
	issues := []Issue{{ID: "abc-1", Title: "task", Status: "open"}}
	m := NewModel(ModelConfig{Lister: stubLister{issues: issues}, Mutator: mutator})
	m = loadIssues(m, issues)

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)
	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(Model)

	if mutator.closedID != "abc-1" {
		t.Fatalf("closed id = %q, want %q", mutator.closedID, "abc-1")
	}
}

func TestCtrlXReopensClosedIssue(t *testing.T) {
	mutator := &stubMutator{}
	issues := []Issue{{ID: "abc-1", Title: "task", Status: "closed"}}
	m := NewModel(ModelConfig{Lister: stubLister{issues: issues}, Mutator: mutator})
	m = loadIssues(m, issues)

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	m = next.(Model)
	msg := cmd()
	next, _ = m.Update(msg)
	m = next.(Model)

	if mutator.reopenedID != "abc-1" {
		t.Fatalf("reopened id = %q, want %q", mutator.reopenedID, "abc-1")
	}
}

func TestCtrlRStartsReloadAndReselectsCurrentIssue(t *testing.T) {
	issues := []Issue{
		{ID: "abc-1", Title: "first", Status: "open"},
		{ID: "abc-2", Title: "second", Status: "open"},
	}
	m := NewModel(ModelConfig{Lister: stubLister{issues: issues}})
	m = loadIssues(m, issues)
	m = modelPress(m, "down")

	next, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m = next.(Model)

	if !m.loading {
		t.Fatal("expected ctrl+r to set loading")
	}
	if m.pendingSelectID != "abc-2" {
		t.Fatalf("pendingSelectID = %q, want %q", m.pendingSelectID, "abc-2")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+r to trigger reload")
	}

	next, _ = m.Update(issuesLoadedMsg{issues: issues})
	m = next.(Model)
	if m.selectedRowID() != "abc-2" {
		t.Fatalf("selected row after reload = %q, want %q", m.selectedRowID(), "abc-2")
	}
}

func TestReadyLoadedMsgResortsIssues(t *testing.T) {
	issues := []Issue{
		{ID: "blocked-1", Status: "open", DependencyCount: 1},
		{ID: "unblocked-1", Status: "open", DependencyCount: 1},
	}
	m := NewModel(ModelConfig{Lister: stubLister{issues: issues}})
	m = loadIssues(m, issues)

	next, _ := m.Update(readyLoadedMsg{ready: map[string]bool{"unblocked-1": true}})
	m = next.(Model)

	if len(*m.displayRef) != 2 || (*m.displayRef)[0].ID != "unblocked-1" {
		t.Fatalf("expected unblocked-1 sorted first once readiness loads, got %+v", *m.displayRef)
	}
}

func TestRenderIssueRowShowsReadinessGlyphAndBadge(t *testing.T) {
	issue := Issue{ID: "abc-1", Title: "do work", Status: "open", DependencyCount: 2, DependentCount: 1}

	row := renderIssueRow(issue, map[string]bool{}, "", false)

	if !strings.Contains(row, readinessGlyph(Blocked)) {
		t.Errorf("expected blocked glyph in row, got %q", row)
	}
	if !strings.Contains(row, "↓2 ↑1") {
		t.Errorf("expected badge %q in row, got %q", "↓2 ↑1", row)
	}
}

func TestRenderIssueRowHidesZeroBadge(t *testing.T) {
	issue := Issue{ID: "abc-1", Title: "do work", Status: "open"}

	row := renderIssueRow(issue, map[string]bool{"abc-1": true}, "", false)

	if strings.Contains(row, "↓") || strings.Contains(row, "↑") {
		t.Errorf("expected no badge for zero blocker/dependent counts, got %q", row)
	}
}

func TestRenderIssueRowTagsEpicAndSubtask(t *testing.T) {
	epic := Issue{ID: "epic-1", Title: "big epic", IssueType: "epic"}
	row := renderIssueRow(epic, map[string]bool{}, "", false)
	if !strings.Contains(row, "epic") {
		t.Errorf("expected epic tag in row, got %q", row)
	}

	subtask := Issue{ID: "sub-1", Title: "subtask", Parent: "epic-1"}
	row = renderIssueRow(subtask, map[string]bool{}, "", false)
	if !strings.Contains(row, "↳ epic-1") {
		t.Errorf("expected parent breadcrumb in row, got %q", row)
	}
}

func TestPreviewVisible_WideTerminalShowsByDefault(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	m = next.(Model)

	if !m.previewVisible() {
		t.Error("expected preview visible on a wide terminal")
	}
}

func TestPreviewVisible_MediumWidthStacksInsteadOfHiding(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = next.(Model)

	if !m.previewVisible() {
		t.Error("expected preview visible (stacked) on a medium-width terminal")
	}
	if !m.previewStacked {
		t.Error("expected preview to be stacked below the list on a medium-width terminal")
	}
}

func TestPreviewVisible_TooNarrowHidesByDefault(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 24})
	m = next.(Model)

	if m.previewVisible() {
		t.Error("expected preview hidden on a too-narrow terminal")
	}
}

func TestTabTogglesPreviewVisibility(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	m = next.(Model)

	next, _ = m.Update(tea.KeyPressMsg{Code: '\t'})
	m = next.(Model)
	if m.previewVisible() {
		t.Error("expected tab to hide the preview on a wide terminal")
	}

	next, _ = m.Update(tea.KeyPressMsg{Code: '\t'})
	m = next.(Model)
	if !m.previewVisible() {
		t.Error("expected a second tab to re-show the preview")
	}
}

func TestTabOnNarrowTerminalForcesPreviewOn(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 24})
	m = next.(Model)

	next, _ = m.Update(tea.KeyPressMsg{Code: '\t'})
	m = next.(Model)
	if !m.previewVisible() {
		t.Error("expected tab to force the preview on despite the narrow terminal")
	}
}

func TestApplyLayout_SideBySideOnWideTerminal(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	m = next.(Model)

	if m.previewStacked {
		t.Error("expected side-by-side layout on a wide terminal")
	}
	if m.previewWidth <= 0 || m.previewWidth >= m.width {
		t.Errorf("expected previewWidth between 0 and width, got %d (width %d)", m.previewWidth, m.width)
	}
}

func TestApplyLayout_StackedOnMediumTerminal(t *testing.T) {
	m := NewModel(ModelConfig{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	m = next.(Model)

	if !m.previewStacked {
		t.Error("expected stacked layout on a medium-width terminal")
	}
	if m.previewWidth != m.width {
		t.Errorf("expected preview to span the full width when stacked, got %d (width %d)", m.previewWidth, m.width)
	}
	if m.previewHeight <= 0 || m.previewHeight >= m.height {
		t.Errorf("expected previewHeight between 0 and height, got %d (height %d)", m.previewHeight, m.height)
	}
}

func TestSelectionChangeStartsPreviewDebounceAndFetch(t *testing.T) {
	fetcher := stubPreviewFetcher{
		upOf: map[string][]DepTreeNode{
			"abc-2": {
				{Issue: Issue{ID: "abc-2"}, Depth: 0},
				{Issue: Issue{ID: "abc-2.1", Title: "child", Status: "closed"}, Depth: 1, ParentID: "abc-2", EdgeFromParent: "parent-child"},
			},
		},
	}
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}, Preview: fetcher})
	m = loadIssues(m, testIssues())

	// Move the cursor from abc-1 to abc-2.
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = next.(Model)
	if m.selectedRowID() != "abc-2" {
		t.Fatalf("expected selection to move to abc-2, got %q", m.selectedRowID())
	}
	if cmd == nil {
		t.Fatal("expected a debounce cmd after the selection changed")
	}

	debounceMsg := cmd()
	dm, ok := debounceMsg.(previewDebounceMsg)
	if !ok || dm.id != "abc-2" {
		t.Fatalf("expected previewDebounceMsg for abc-2, got %#v", debounceMsg)
	}

	next, cmd = m.Update(dm)
	m = next.(Model)
	if cmd == nil {
		t.Fatal("expected a fetch cmd from the debounce message")
	}

	loadedMsg := cmd()
	lm, ok := loadedMsg.(previewLoadedMsg)
	if !ok || lm.id != "abc-2" {
		t.Fatalf("expected previewLoadedMsg for abc-2, got %#v", loadedMsg)
	}
	if lm.data.subtasks == nil || len(lm.data.subtasks.Children) != 1 {
		t.Fatalf("expected abc-2's fetched preview to include its child, got %+v", lm.data)
	}

	next, _ = m.Update(lm)
	m = next.(Model)
	if _, cached := m.previewCache["abc-2"]; !cached {
		t.Error("expected abc-2's preview to be cached after previewLoadedMsg")
	}
}

func TestStalePreviewMessagesAreIgnored(t *testing.T) {
	m := NewModel(ModelConfig{Lister: stubLister{issues: testIssues()}, Preview: stubPreviewFetcher{}})
	m = loadIssues(m, testIssues())

	next, _ := m.Update(previewDebounceMsg{seq: m.previewSeq + 99, id: "abc-1"})
	m = next.(Model)
	if _, cached := m.previewCache["abc-1"]; cached {
		t.Error("expected a stale-seq debounce message to be ignored")
	}

	next, _ = m.Update(previewLoadedMsg{seq: m.previewSeq + 99, id: "abc-1", data: previewData{}})
	m = next.(Model)
	if _, cached := m.previewCache["abc-1"]; cached {
		t.Error("expected a stale-seq loaded message to be ignored")
	}
}
