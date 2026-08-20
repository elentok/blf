package launcher

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/launcher/ai"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/history"
	"github.com/elentok/blf/internal/launcher/learnedrank"
	"github.com/elentok/blf/internal/launcher/scripts"
)

const maxResults = 200

// ModelConfig holds injectable dependencies for the launcher model.
type ModelConfig struct {
	Providers        []Provider
	ConfigErr        error
	CopyText         func(string) error
	ReadClipboard    func() (string, error) // optional; nil disables clipboard reads
	HideTerminal     func() error
	LaunchApp        func(string) error // optional; launches an app by path
	OpenTarget       func(string) error // optional; opens a file/URL via `open` (no -a)
	UseNerdFont      bool
	CurrencyCache    *currency.Cache                   // optional; nil disables currency refresh
	AppsProvider     *AppsProvider                     // optional; nil disables app search
	AppsCachePath    string                            // path to apps.json; empty disables refresh
	HomeDir          string                            // used by ReindexCmd
	ScriptsProvider  *ScriptsProvider                  // optional; nil disables script execution
	CommandsProvider *CommandsProvider                 // optional; nil disables built-in command execution
	SnippetsProvider *SnippetsProvider                 // optional; nil disables snippet reload on "reload"
	History          *history.History                  // optional; nil disables history
	HistoryPath      string                            // path to persist history; empty skips persistence
	LearnedRank      *learnedrank.Store                // optional; nil disables the learned-rank feature
	LearnedRankPath  string                            // path to persist learned ranks; empty skips persistence
	HideDelay        time.Duration                     // delay before hiding the terminal (see resetAndHide); 0 = immediate
	ShowNotification func(title, message string) error // optional; nil disables completion notifications
	NoBorder         bool                              // disable the outer border frame
	AIExec           ai.ExecFunc                       // optional; nil disables ai run dispatch
	AIModel          string                            // claude model alias passed to ai.Invoke
	AITimeout        time.Duration                     // per-run deadline passed to ai.Invoke; 0 disables it
}

// inputProxy mirrors the widget query via a shared *string pointer so tests
// can call m.input.Value() / m.input.SetValue() without the launcher owning a
// separate textinput.
type inputProxy struct {
	queryRef  *string
	widgetRef *fuzzyfinder.Model
}

func (p inputProxy) Value() string     { return *p.queryRef }
func (p inputProxy) SetValue(s string) { *p.queryRef = s; p.widgetRef.SetQuery(s) }
func (p inputProxy) Reset()            { *p.queryRef = ""; p.widgetRef.SetQuery("") }

// clearStatusMsg is sent after a delay to erase a transient status message.
type clearStatusMsg struct{}

// Model is the bubbletea model for the launcher TUI.
type Model struct {
	cfg               ModelConfig
	input             inputProxy // delegates to widget; satisfies test field access
	widget            fuzzyfinder.Model
	resultsRef        *[]Result
	widthRef          *int
	results           []Result
	selected          int
	offset            int // kept for test compat; reset to 0 in resetAndHide
	width             int
	height            int
	helpMode          bool
	status            string       // transient status / error message
	lastAppsIndexedAt time.Time    // mtime of last loaded apps cache
	scriptOutput      []Result     // non-nil after a "show" script; overrides provider results
	historyIdx        int          // -1 = not navigating; >=0 = index into history entries
	aiPromptKind      AIPromptKind // "" = not in ai prompt mode; else the kind that entered it
	clipboardSnapshot string       // clipboard contents read once at ai prompt mode entry
	aiPromptError     string       // inline error shown in the footer in ai prompt mode; cleared on the next keystroke
	previewRef        *[]string    // pointer for the RenderRow closure; non-empty = preview lines replace m.results
}

// AIRunDoneMsg carries the outcome of one ai run dispatched from ai prompt
// mode's Enter handler. Handling completion — appending to the runs store,
// copying to the clipboard, notifying — is a later ticket; dispatch only
// needs the message shape to fire the background command.
type AIRunDoneMsg struct {
	Kind   AIPromptKind
	Input  string
	Result ai.InvokeResult
}

// aiPromptModeLegend is the footer key legend shown while in ai prompt mode.
const aiPromptModeLegend = "esc: cancel"

// clipboardPreviewHeader is the first line of the clipboard preview shown
// with an empty input in ai prompt mode.
const clipboardPreviewHeader = "Press enter to use the clipboard:"

// NewModel creates a launcher Model ready to run.
func NewModel(cfg ModelConfig) Model {
	queryRef := new(string)
	resultsRef := new([]Result)
	widthRef := new(int)
	previewRef := new([]string)

	m := Model{
		cfg:        cfg,
		historyIdx: -1,
		resultsRef: resultsRef,
		widthRef:   widthRef,
		previewRef: previewRef,
	}

	useNerdFont := cfg.UseNerdFont
	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			if preview := *previewRef; len(preview) > 0 {
				if i >= len(preview) {
					return ""
				}
				return preview[i]
			}
			results := *resultsRef
			if i >= len(results) {
				return ""
			}
			return renderResultRow(results[i], useNerdFont, selected)
		},
		Footer:    "?: help",
		ItemCount: 0,
		NoBorder:  cfg.NoBorder,
	})

	m.input = inputProxy{queryRef: queryRef, widgetRef: &m.widget}

	if cfg.ConfigErr != nil {
		m.widget.SetFooter("config: " + cfg.ConfigErr.Error())
	}

	return m
}

// setQuery sets the query on both the shared query ref and the live widget.
//
// m.input mirrors the query via a widget pointer captured at construction, but
// bubbletea copies the model on every Update so that pointer is stale relative
// to the live widget — writes through it never reach the rendered textinput.
// Programmatic query changes must therefore go through the live m.widget here.
func (m *Model) setQuery(s string) {
	*m.input.queryRef = s
	m.widget.SetQuery(s)
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.widget.Init()}
	if m.cfg.CurrencyCache != nil {
		cmds = append(cmds, FetchRatesCmd(m.cfg.CurrencyCache))
	}
	if m.cfg.AppsProvider != nil && m.cfg.HomeDir != "" {
		cmds = append(cmds, ReindexCmd(m.cfg.HomeDir, m.cfg.AppsCachePath))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		*m.widthRef = msg.Width
		m.widget.SetSize(msg.Width, msg.Height)
		// On show, repopulate results so an empty query shows the default recent
		// list rather than the stale/empty list left behind when we last hid.
		m.recomputeResults()
		// On show, reload apps from disk if the cache was updated externally.
		if m.cfg.AppsProvider != nil && m.cfg.AppsCachePath != "" {
			if info, err := os.Stat(m.cfg.AppsCachePath); err == nil {
				if info.ModTime().After(m.lastAppsIndexedAt) {
					return m, LoadAppsFromDiskCmd(m.cfg.AppsCachePath)
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		if m.helpMode {
			m.helpMode = false
			return m, nil
		}
		if m.aiPromptKind != "" {
			// Any keystroke clears a stale inline error before it is
			// re-evaluated below, so the error never outlives the
			// keypress after the one that raised it.
			if m.aiPromptError != "" {
				m.aiPromptError = ""
				m.updateFooter()
			}
			switch key {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.exitAIPromptMode()
				return m, nil
			case "enter":
				return m, m.dispatchAIRun()
			case "up", "down", "ctrl+k", "ctrl+j", "ctrl+p", "ctrl+n":
				// Navigation is swallowed: results are not recomputed
				// while in mode, so there is nothing to navigate.
				return m, nil
			default:
				var cmd tea.Cmd
				m.widget, cmd = m.widget.Update(msg)
				*m.input.queryRef = m.widget.Query()
				// Results are not recomputed while in mode (no provider
				// query), but the clipboard preview is a pure function of
				// (mode, input=="") and must react to every keystroke: it
				// disappears on the first typed character and reappears if
				// backspaced back to empty.
				m.syncWidget()
				return m, cmd
			}
		}
		switch key {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Reset and hide the quick terminal (ADR 0002)
			if len(m.results) > 0 {
				role := m.results[0].Icon
				if role == IconRoleCalc || role == IconRoleUnit || role == IconRoleCurrency {
					m.recordHistory(m.input.Value(), m.results[0])
				}
			}
			return m, m.resetAndHide()

		case "up", "ctrl+k", "ctrl+p":
			if m.selected > 0 {
				m.selected--
				if m.selected < m.offset {
					m.offset = m.selected
				}
				m.widget.SetSelected(m.selected)
			}
			return m, nil

		case "down", "ctrl+j", "ctrl+n":
			if m.selected < len(m.results)-1 {
				m.selected++
				visibleRows := m.visibleResultRows()
				if m.selected >= m.offset+visibleRows {
					m.offset = m.selected - visibleRows + 1
				}
				m.widget.SetSelected(m.selected)
			}
			return m, nil

		case "enter":
			if len(m.results) == 0 {
				return m, nil
			}
			result := m.results[m.selected]
			cmd, err := m.act(result)
			if err != nil {
				m.status = err.Error()
				m.updateFooter()
				return m, nil
			}
			if cmd != nil {
				// Async action (e.g. launch): wait for result before hiding.
				return m, cmd
			}
			// Sync action success: reset and hide.
			return m, m.resetAndHide()

		case "ctrl+shift+r":
			if m.cfg.AppsProvider != nil && m.cfg.HomeDir != "" {
				m.status = "reindexing apps…"
				m.updateFooter()
				return m, ReindexCmd(m.cfg.HomeDir, m.cfg.AppsCachePath)
			}
			return m, nil

		case "ctrl+r":
			if m.cfg.History == nil || m.cfg.History.Len() == 0 {
				return m, nil
			}
			entries := m.cfg.History.Entries()
			next := m.historyIdx + 1
			if next >= len(entries) {
				next = len(entries) - 1
			}
			m.historyIdx = next
			m.setQuery(entries[next].Label)
			m.scriptOutput = nil
			m.recomputeResults()
			return m, nil

		case "ctrl+f":
			if m.cfg.History == nil || m.historyIdx < 0 {
				return m, nil
			}
			next := m.historyIdx - 1
			if next < 0 {
				m.historyIdx = -1
				m.setQuery("")
			} else {
				m.historyIdx = next
				m.setQuery(m.cfg.History.Entries()[next].Label)
			}
			m.scriptOutput = nil
			m.recomputeResults()
			return m, nil

		case "ctrl+s":
			if m.cfg.History == nil {
				return m, nil
			}
			val := strings.TrimSpace(m.input.Value())
			if val == "" {
				return m, nil
			}
			m.cfg.History.Append(history.Entry{Label: val, ActionType: history.ActionTypeCopy})
			m.saveHistory()
			m.status = "saved"
			m.updateFooter()
			return m, clearStatusAfter(1500 * time.Millisecond)

		case "ctrl+x":
			// Delete the selected entry from history (only when history rows are shown).
			if m.cfg.History == nil || len(m.results) == 0 {
				return m, nil
			}
			result := m.results[m.selected]
			if result.Action.Type != ActionRecall || result.HistoryEntry == nil {
				return m, nil
			}
			if m.cfg.History.Remove(*result.HistoryEntry) {
				m.saveHistory()
				m.historyIdx = -1
				m.recomputeResults()
				// Keep the cursor near the deleted row instead of jumping to top.
				if m.selected >= len(m.results) && len(m.results) > 0 {
					m.selected = len(m.results) - 1
				}
				if m.selected < m.offset {
					m.offset = m.selected
				}
				m.widget.SetSelected(m.selected)
			}
			return m, nil

		case "?":
			m.helpMode = true
			return m, nil

		default:
			prev := m.input.Value()
			var cmd tea.Cmd
			m.widget, cmd = m.widget.Update(msg)
			newQuery := m.widget.Query()
			if newQuery != prev {
				*m.input.queryRef = newQuery
				m.scriptOutput = nil
				m.historyIdx = -1
			}
			m.recomputeResults()
			return m, cmd
		}

	case RatesFetchedMsg:
		m.recomputeResults()
		if msg.Err == nil && m.cfg.CurrencyCache != nil {
			ttl := m.cfg.CurrencyCache.TTL()
			if ttl > 0 {
				return m, ScheduleRatesTick(ttl)
			}
		}
		return m, nil

	case RatesTTLMsg:
		if m.cfg.CurrencyCache != nil {
			return m, FetchRatesCmd(m.cfg.CurrencyCache)
		}
		return m, nil

	case AppsReindexedMsg:
		switch {
		case msg.Err != nil:
			m.status = "reindex error: " + msg.Err.Error()
		case msg.Index != nil && m.cfg.AppsProvider != nil:
			m.cfg.AppsProvider.SetIndex(msg.Index)
			m.lastAppsIndexedAt = msg.Index.IndexedAt
			m.status = fmt.Sprintf("reindexed %d apps", len(msg.Index.Apps))
		default:
			m.status = ""
		}
		m.recomputeResults()
		m.updateFooter()
		cmds := []tea.Cmd{clearStatusAfter(1500 * time.Millisecond)}
		if m.cfg.AppsCachePath != "" {
			cmds = append(cmds, ScheduleAppsRefresh(30*time.Minute))
		}
		return m, tea.Batch(cmds...)

	case ReloadDoneMsg:
		var parts []string
		if msg.AppsErr != nil {
			parts = append(parts, "apps: "+msg.AppsErr.Error())
		} else if msg.AppsIndex != nil && m.cfg.AppsProvider != nil {
			m.cfg.AppsProvider.SetIndex(msg.AppsIndex)
			m.lastAppsIndexedAt = msg.AppsIndex.IndexedAt
			parts = append(parts, fmt.Sprintf("%d apps", len(msg.AppsIndex.Apps)))
		}
		if msg.SnippetsErr != nil {
			parts = append(parts, "snippets: "+msg.SnippetsErr.Error())
		} else if m.cfg.SnippetsProvider != nil {
			m.cfg.SnippetsProvider.SetSnippets(msg.Snippets)
			parts = append(parts, fmt.Sprintf("%d snippets", len(msg.Snippets)))
		}
		if len(parts) > 0 {
			m.status = "reloaded " + strings.Join(parts, ", ")
		} else {
			m.status = ""
		}
		m.recomputeResults()
		m.updateFooter()
		return m, clearStatusAfter(1500 * time.Millisecond)

	case AppsRefreshTickMsg:
		if m.cfg.AppsProvider != nil && m.cfg.HomeDir != "" {
			return m, ReindexCmd(m.cfg.HomeDir, m.cfg.AppsCachePath)
		}
		return m, nil

	case AppLaunchResultMsg:
		if msg.Err != nil {
			m.status = "launch error: " + msg.Err.Error()
			m.updateFooter()
			return m, nil
		}
		return m, m.resetAndHide()

	case ScriptRunMsg:
		m.status = ""
		if msg.Result.Err != nil {
			m.status = "script error: " + msg.Result.Stderr
			m.updateFooter()
			return m, nil
		}
		switch msg.Output {
		case scripts.OutputShow:
			lines := strings.Split(msg.Result.Stdout, "\n")
			out := make([]Result, 0, len(lines))
			for _, l := range lines {
				if l == "" {
					continue
				}
				out = append(out, Result{
					Title:  l,
					Icon:   IconRoleScript,
					Action: Action{Type: ActionCopy, Target: l},
				})
			}
			m.scriptOutput = out
			m.recomputeResults()
			return m, nil
		case scripts.OutputClipboard:
			if m.cfg.CopyText != nil {
				_ = m.cfg.CopyText(msg.Result.Stdout)
			}
		}
		// ignore or clipboard: reset and hide
		return m, m.resetAndHide()

	case CleanURLDoneMsg:
		if msg.Err != nil {
			m.status = "cleanurl error: " + msg.Err.Error()
			m.updateFooter()
			return m, nil
		}
		if m.cfg.ShowNotification != nil {
			_ = m.cfg.ShowNotification("blf", "Cleaned URL copied to clipboard")
		}
		return m, m.resetAndHide()

	case EnterAIPromptModeMsg:
		m.enterAIPromptMode(msg.Kind)
		return m, nil

	case clearStatusMsg:
		if m.status == "saved" || strings.HasPrefix(m.status, "reindexed ") || strings.HasPrefix(m.status, "reindex error:") || strings.HasPrefix(m.status, "reloaded ") {
			m.status = ""
			m.updateFooter()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.widget, cmd = m.widget.Update(msg)
	return m, cmd
}

func (m *Model) recomputeResults() {
	if m.scriptOutput != nil {
		m.results = m.scriptOutput
		if m.selected >= len(m.results) {
			m.selected = 0
			m.offset = 0
		}
		m.syncWidget()
		return
	}
	query := m.input.Value()
	// Empty input: show recent history instead of provider results.
	if query == "" && m.cfg.History != nil && m.cfg.History.Len() > 0 {
		entries := m.cfg.History.Entries()
		m.results = make([]Result, len(entries))
		for i := range entries {
			e := entries[i]
			icon := IconRoleHistory
			iconGlyph := ""
			subtitle := ""
			if e.ActionType == history.ActionTypeCopy {
				subtitle = m.historyHint(e.Label)
			} else if r, ok := m.lookupHistoryResult(e); ok {
				icon = r.Icon
				iconGlyph = r.IconGlyph
				subtitle = r.Subtitle
			}
			m.results[i] = Result{
				Title:        e.Label,
				Subtitle:     subtitle,
				Icon:         icon,
				IconGlyph:    iconGlyph,
				Action:       Action{Type: ActionRecall, Target: e.Label},
				HistoryEntry: &entries[i],
			}
		}
		if m.selected >= len(m.results) {
			m.selected = 0
			m.offset = 0
		}
		m.syncWidget()
		return
	}
	var all []Result
	for _, p := range m.cfg.Providers {
		all = append(all, p.Query(query)...)
	}
	var learnedRanks map[string]int
	if m.cfg.LearnedRank != nil {
		learnedRanks = m.cfg.LearnedRank.Counts(query)
	}
	ranked := Rank(all, learnedRanks)
	if len(ranked) > maxResults {
		ranked = ranked[:maxResults]
	}
	m.results = ranked
	if m.selected >= len(m.results) {
		m.selected = 0
		m.offset = 0
	}
	m.syncWidget()
}

// syncWidget keeps the fuzzyfinder widget in sync with m.results and
// m.selected, or — while in ai prompt mode with an empty input — with the
// clipboard preview lines instead. The preview replaces the whole viewport,
// so it is pinned to item 0 rather than tracking m.selected: navigation is
// already swallowed in mode (see Update), so there is no selection to track.
func (m *Model) syncWidget() {
	*m.resultsRef = m.results
	if m.aiPromptKind != "" && m.input.Value() == "" {
		*m.previewRef = m.clipboardPreviewLines()
		m.widget.SetItemCount(len(*m.previewRef))
		m.widget.SetSelected(0)
		return
	}
	*m.previewRef = nil
	m.widget.SetItemCount(max(len(m.results), 1))
	m.widget.SetSelected(m.selected)
}

// clipboardPreviewLines returns the lines that fill the result viewport in
// ai prompt mode with an empty input: a fixed header line followed by the
// clipboard snapshot's own lines.
func (m *Model) clipboardPreviewLines() []string {
	lines := []string{clipboardPreviewHeader}
	if m.clipboardSnapshot != "" {
		lines = append(lines, strings.Split(m.clipboardSnapshot, "\n")...)
	}
	return lines
}

// updateFooter updates the widget footer to reflect the current mode/status/
// configErr. It is called from several background message handlers (apps
// reindex, reload, the periodic apps refresh) that know nothing about modes,
// so the mode check comes first: while in ai prompt mode the footer always
// shows the mode's key legend, never the status message.
func (m *Model) updateFooter() {
	if m.aiPromptKind != "" {
		if m.aiPromptError != "" {
			m.widget.SetFooter(m.aiPromptError)
		} else {
			m.widget.SetFooter(aiPromptModeLegend)
		}
	} else if m.status != "" {
		m.widget.SetFooter(m.status)
	} else if m.cfg.ConfigErr != nil {
		m.widget.SetFooter("config: " + m.cfg.ConfigErr.Error())
	} else {
		m.widget.SetFooter("?: help")
	}
}

// enterAIPromptMode flips the launcher into ai prompt mode for kind: the
// input prompt names the kind and the footer switches to the mode's key
// legend. Called from Update on EnterAIPromptModeMsg, since a command's Run
// returns a tea.Cmd and cannot mutate the model directly.
//
// The clipboard is read once here, into m.clipboardSnapshot, rather than per
// keystroke — ReadClipboard shells out to a subprocess on macOS, and a later
// ticket sends this same snapshot so the preview can never lie about what
// was dispatched.
func (m *Model) enterAIPromptMode(kind AIPromptKind) {
	m.aiPromptKind = kind
	m.widget.SetPrompt(string(kind) + " ")
	m.clipboardSnapshot = ""
	if m.cfg.ReadClipboard != nil {
		if text, err := m.cfg.ReadClipboard(); err == nil {
			m.clipboardSnapshot = text
		}
	}
	m.syncWidget()
	m.updateFooter()
}

// exitAIPromptMode returns the launcher to normal operation, leaving it
// visible rather than hiding it, so a mistaken pick costs one key.
func (m *Model) exitAIPromptMode() {
	m.aiPromptKind = ""
	m.status = ""
	m.aiPromptError = ""
	m.widget.SetPrompt("")
	m.syncWidget()
	m.updateFooter()
}

// dispatchAIRun resolves Enter's input in ai prompt mode — typed text if
// there is any, otherwise the clipboard snapshot taken at mode entry — and
// fires an ai run in the background with the configured model and timeout,
// then exits the mode and takes the reset-and-hide path so the launcher is
// out of the way while the model is still thinking.
//
// With nothing to send (both the typed input and the snapshot are empty) it
// sets an inline error instead and returns nil, leaving the launcher in the
// mode with the cursor intact.
func (m *Model) dispatchAIRun() tea.Cmd {
	input := m.input.Value()
	if input == "" {
		input = m.clipboardSnapshot
	}
	if input == "" {
		m.aiPromptError = "nothing to send"
		m.updateFooter()
		return nil
	}

	kind := m.aiPromptKind
	execFn := m.cfg.AIExec
	model := m.cfg.AIModel
	timeout := m.cfg.AITimeout
	runCmd := func() tea.Msg {
		ctx := context.Background()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		result := ai.Invoke(ctx, execFn, model, ai.Kind(kind), input)
		return AIRunDoneMsg{Kind: kind, Input: input, Result: result}
	}

	m.exitAIPromptMode()
	return tea.Batch(runCmd, m.resetAndHide())
}

func (m *Model) act(r Result) (tea.Cmd, error) {
	switch r.Action.Type {
	case ActionRecall:
		// A history-direct-fire entry (launch/run/open) carries its original
		// action in HistoryEntry; re-dispatch through the same execution
		// paths below rather than populating the input (ADR 0006).
		if r.HistoryEntry != nil && r.HistoryEntry.ActionType != history.ActionTypeCopy {
			entry := *r.HistoryEntry
			return m.act(Result{
				Title:  entry.Label,
				Action: Action{Type: ActionType(entry.ActionType), Target: entry.Target},
			})
		}
		m.setQuery(r.Action.Target)
		m.historyIdx = -1
		m.scriptOutput = nil
		m.recomputeResults()
		// Return a no-op cmd so the Enter handler takes the async path and
		// does not reset the input or hide the terminal.
		return func() tea.Msg { return nil }, nil
	case ActionCopy:
		if m.cfg.CopyText == nil {
			return nil, fmt.Errorf("copy not available")
		}
		m.recordHistory(m.input.Value(), r)
		m.recordLearnedRank(m.input.Value(), r)
		return nil, m.cfg.CopyText(r.Action.Target)
	case ActionLaunch:
		if m.cfg.LaunchApp == nil {
			return nil, fmt.Errorf("launch not available")
		}
		m.recordHistory(m.input.Value(), r)
		m.recordLearnedRank(m.input.Value(), r)
		target := r.Action.Target
		launchFn := m.cfg.LaunchApp
		return func() tea.Msg {
			err := launchFn(target)
			return AppLaunchResultMsg{AppPath: target, Err: err}
		}, nil
	case ActionRun:
		if m.cfg.ScriptsProvider == nil {
			return nil, fmt.Errorf("scripts not available")
		}
		s, ok := m.cfg.ScriptsProvider.Find(r.Action.Target)
		if !ok {
			return nil, fmt.Errorf("script not found: %s", r.Action.Target)
		}
		m.recordHistory(m.input.Value(), r)
		m.recordLearnedRank(m.input.Value(), r)
		m.status = "running…"
		m.updateFooter()
		return ScriptRunCmd(s), nil
	case ActionCommand:
		if m.cfg.CommandsProvider == nil {
			return nil, fmt.Errorf("commands not available")
		}
		c, ok := m.cfg.CommandsProvider.Find(r.Action.Target)
		if !ok {
			return nil, fmt.Errorf("command not found: %s", r.Action.Target)
		}
		m.recordHistory(m.input.Value(), r)
		m.recordLearnedRank(m.input.Value(), r)
		m.setQuery("")
		m.historyIdx = -1
		m.scriptOutput = nil
		m.status = "running…"
		m.updateFooter()
		m.recomputeResults()
		return c.Run(), nil
	case ActionOpen:
		if m.cfg.OpenTarget == nil {
			return nil, fmt.Errorf("open not available")
		}
		m.recordHistory(m.input.Value(), r)
		m.recordLearnedRank(m.input.Value(), r)
		target := r.Action.Target
		openFn := m.cfg.OpenTarget
		return func() tea.Msg {
			err := openFn(target)
			return AppLaunchResultMsg{AppPath: target, Err: err}
		}, nil
	default:
		return nil, fmt.Errorf("action not yet implemented")
	}
}

// historyHint returns the dimmed-italic "= <result>" subtitle for a history
// row, computed live from the first provider that offers a hint, or "" if none.
func (m *Model) historyHint(query string) string {
	for _, p := range m.cfg.Providers {
		if hp, ok := p.(HintProvider); ok {
			if h := hp.Hint(query); h != "" {
				return h
			}
		}
	}
	return ""
}

// lookupHistoryResult re-derives the current icon/subtitle for a launch/run/
// open history entry by asking each TargetLookupProvider whether it still
// owns e's (ActionType, Target). Returns ok=false if no provider claims it
// (e.g. the app was removed or renamed since it was recorded), in which case
// callers should fall back to a generic display.
func (m *Model) lookupHistoryResult(e history.Entry) (Result, bool) {
	action := Action{Type: ActionType(e.ActionType), Target: e.Target}
	for _, p := range m.cfg.Providers {
		if lp, ok := p.(TargetLookupProvider); ok {
			if r, found := lp.LookupResult(action); found {
				return r, true
			}
		}
	}
	return Result{}, false
}

// recordHistory appends a history entry for the picked result and persists
// it if a path is configured. For ActionCopy (calc/unit/currency) the entry
// stores the raw query text, since those results have no identity
// independent of their query; for launch/run/open it stores the result's
// label and action (see ADR 0006).
func (m *Model) recordHistory(query string, r Result) {
	if m.cfg.History == nil {
		return
	}
	var entry history.Entry
	if r.Action.Type == ActionCopy {
		entry = history.Entry{Label: strings.TrimSpace(query), ActionType: history.ActionTypeCopy}
	} else {
		entry = history.Entry{Label: r.Title, ActionType: int(r.Action.Type), Target: r.Action.Target}
	}
	m.cfg.History.Append(entry)
	m.saveHistory()
}

// saveHistory persists history to disk if HistoryPath is set.
func (m *Model) saveHistory() {
	if m.cfg.History == nil || m.cfg.HistoryPath == "" {
		return
	}
	_ = m.cfg.History.Save(m.cfg.HistoryPath)
}

// recordLearnedRank records that result was picked for query, when it was not
// the top result at the time of picking, and persists it if a path is
// configured. No-op when learned rank is disabled or the pick was first.
func (m *Model) recordLearnedRank(query string, result Result) {
	trimmed := strings.TrimSpace(query)
	if m.cfg.LearnedRank == nil || m.selected == 0 || trimmed == "" {
		return
	}
	m.cfg.LearnedRank.Increment(trimmed, result.Action.Key())
	m.saveLearnedRank()
}

// saveLearnedRank persists learned ranks to disk if LearnedRankPath is set.
func (m *Model) saveLearnedRank() {
	if m.cfg.LearnedRank == nil || m.cfg.LearnedRankPath == "" {
		return
	}
	_ = m.cfg.LearnedRank.Save(m.cfg.LearnedRankPath)
}

// resetAndHide clears the launcher back to its empty state and returns a Cmd that
// hides the configured quick terminal.
//
// The hide is deferred by cfg.HideDelay (rather than called inline) to dodge a
// bubbletea v2 render race: Update stores the new view but the terminal is only
// flushed on a ~60fps ticker in a separate goroutine, and there is no synchronous
// flush API. Hiding immediately would let Kitty save the pre-reset buffer (the old
// input text), which it then restores on the next show — a visible flash before the
// next repaint clears it. Waiting one render tick ensures the cleared frame is
// flushed before we hide, so the buffer Kitty saves is already clean.
func (m *Model) resetAndHide() tea.Cmd {
	m.setQuery("")
	m.selected = 0
	m.offset = 0
	m.status = ""
	m.scriptOutput = nil
	m.historyIdx = -1
	// Repopulate the default recent list now (rather than on the next show), since
	// reshowing the quick terminal at the same size emits no WindowSizeMsg.
	m.recomputeResults()
	m.updateFooter()

	hide := m.cfg.HideTerminal
	if hide == nil {
		return nil
	}
	delay := m.cfg.HideDelay
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		_ = hide()
		return nil
	}
}

// clearStatusAfter returns a Cmd that sends clearStatusMsg after d.
func clearStatusAfter(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return clearStatusMsg{}
	}
}

// visibleResultRows returns how many result rows fit in the current terminal.
// Layout: border top (1) + input (1) + separator (1) + footer (1) + border bottom (1) = 5 overhead,
// or 3 (input + separator + footer) when the border is disabled.
func (m Model) visibleResultRows() int {
	overhead := 5
	if m.cfg.NoBorder {
		overhead = 3
	}
	n := m.height - overhead
	if n < 1 {
		n = 1
	}
	return n
}

func (m Model) View() tea.View {
	if m.helpMode {
		w := max(m.width, 14)
		h := max(m.height, 6)
		style := borderStyle
		if m.cfg.NoBorder {
			style = noBorderStyle
		}
		content := style.Width(w).Height(h).Render(m.renderHelp())
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	content := m.widget.View()
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// renderResultRow renders a single result row; used by the fuzzyfinder
// RenderRow callback. The active-row gutter is owned by the widget.
func renderResultRow(r Result, useNerdFont bool, selected bool) string {
	icon := ""
	if r.IconGlyph != "" && useNerdFont {
		icon = r.IconGlyph + " "
	} else {
		icon = Icon(r.Icon, useNerdFont)
	}

	return fuzzyfinder.RenderItem(fuzzyfinder.Item{
		Icon:        icon,
		Title:       r.Title,
		Subtitle:    r.Subtitle,
		MatchRanges: r.MatchRanges,
		Selected:    selected,
	})
}

func (m Model) renderHelp() string {
	type binding struct{ key, desc string }
	bindings := []binding{
		{"↑ / ↓", "select result"},
		{"ctrl+k / ctrl+j", "(aliases for ↑ / ↓)"},
		{"ctrl+p / ctrl+n", "(aliases for ↑ / ↓)"},
		{"enter", "act on selected result (copy / launch / run)"},
		{"ctrl+r", "recall previous history entry"},
		{"ctrl+f", "recall next history entry"},
		{"ctrl+s", "save current input to history"},
		{"ctrl+x", "delete selected history entry"},
		{"ctrl+shift+r", "reindex apps"},
		{"esc", "dismiss launcher and clear input"},
		{"?", "toggle this help"},
		{"ctrl+c", "quit launcher process"},
	}

	maxKeyWidth := 0
	for _, b := range bindings {
		if w := lipgloss.Width(b.key); w > maxKeyWidth {
			maxKeyWidth = w
		}
	}

	var sb strings.Builder
	sb.WriteString(helpTitleStyle.Render("blf launcher — help") + "\n\n")
	for _, b := range bindings {
		pad := strings.Repeat(" ", maxKeyWidth-lipgloss.Width(b.key))
		key := helpKeyStyle.Render(b.key) + pad
		desc := helpDescStyle.Render(b.desc)
		sb.WriteString("  " + key + "  " + desc + "\n")
	}
	sb.WriteString("\n" + helpHintStyle.Render("  press any key to close help"))
	return sb.String()
}
