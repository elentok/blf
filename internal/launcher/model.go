package launcher

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/history"
	"github.com/elentok/blf/internal/launcher/scripts"
)

const maxResults = 200

// ModelConfig holds injectable dependencies for the launcher model.
type ModelConfig struct {
	Providers       []Provider
	ConfigErr       error
	CopyText        func(string) error
	HideTerminal    func() error
	LaunchApp       func(string) error // optional; launches an app by path
	UseNerdFont     bool
	CurrencyCache   *currency.Cache  // optional; nil disables currency refresh
	AppsProvider    *AppsProvider    // optional; nil disables app search
	AppsCachePath   string           // path to apps.json; empty disables refresh
	HomeDir         string           // used by ReindexCmd
	ScriptsProvider *ScriptsProvider // optional; nil disables script execution
	History         *history.History // optional; nil disables history
	HistoryPath     string           // path to persist history; empty skips persistence
}

// clearStatusMsg is sent after a delay to erase a transient status message.
type clearStatusMsg struct{}

// Model is the bubbletea model for the launcher TUI.
type Model struct {
	cfg               ModelConfig
	input             textinput.Model
	results           []Result
	selected          int
	offset            int // viewport scroll offset into results
	width             int
	height            int
	helpMode          bool
	status            string    // transient status / error message
	lastAppsIndexedAt time.Time // mtime of last loaded apps cache
	scriptOutput      []Result  // non-nil after a "show" script; overrides provider results
	historyIdx        int       // -1 = not navigating; >=0 = index into history entries
}

// NewModel creates a launcher Model ready to run.
func NewModel(cfg ModelConfig) Model {
	ti := textinput.New()
	ti.Prompt = inputPromptStyle.Render("> ")
	_ = ti.Focus()

	return Model{
		cfg:        cfg,
		input:      ti,
		historyIdx: -1,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
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
		switch key {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Reset and hide the quick terminal (ADR 0002)
			m.input.Reset()
			m.results = nil
			m.selected = 0
			m.offset = 0
			m.status = ""
			m.scriptOutput = nil
			m.historyIdx = -1
			if m.cfg.HideTerminal != nil {
				_ = m.cfg.HideTerminal()
			}
			return m, nil

		case "up", "ctrl+k":
			if m.selected > 0 {
				m.selected--
				if m.selected < m.offset {
					m.offset = m.selected
				}
			}
			return m, nil

		case "down", "ctrl+j":
			if m.selected < len(m.results)-1 {
				m.selected++
				visibleRows := m.visibleResultRows()
				if m.selected >= m.offset+visibleRows {
					m.offset = m.selected - visibleRows + 1
				}
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
				return m, nil
			}
			if cmd != nil {
				// Async action (e.g. launch): wait for result before hiding.
				return m, cmd
			}
			// Sync action success: reset and hide.
			m.input.Reset()
			m.results = nil
			m.selected = 0
			m.offset = 0
			m.status = ""
			if m.cfg.HideTerminal != nil {
				_ = m.cfg.HideTerminal()
			}
			return m, nil

		case "ctrl+r":
			if m.cfg.AppsProvider != nil && m.cfg.HomeDir != "" {
				m.status = "reindexing apps…"
				return m, ReindexCmd(m.cfg.HomeDir, m.cfg.AppsCachePath)
			}
			return m, nil

		case "ctrl+p":
			if m.cfg.History == nil || m.cfg.History.Len() == 0 {
				return m, nil
			}
			entries := m.cfg.History.Entries()
			next := m.historyIdx + 1
			if next >= len(entries) {
				next = len(entries) - 1
			}
			m.historyIdx = next
			m.input.SetValue(entries[next])
			m.scriptOutput = nil
			m.recomputeResults()
			return m, nil

		case "ctrl+n":
			if m.cfg.History == nil || m.historyIdx < 0 {
				return m, nil
			}
			next := m.historyIdx - 1
			if next < 0 {
				m.historyIdx = -1
				m.input.SetValue("")
			} else {
				m.historyIdx = next
				m.input.SetValue(m.cfg.History.Entries()[next])
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
			m.cfg.History.Append(val)
			m.saveHistory()
			m.status = "saved"
			return m, clearStatusAfter(1500 * time.Millisecond)

		case "?":
			m.helpMode = true
			return m, nil

		default:
			prev := m.input.Value()
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if m.input.Value() != prev {
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
		m.status = ""
		if msg.Err == nil && msg.Index != nil && m.cfg.AppsProvider != nil {
			m.cfg.AppsProvider.SetIndex(msg.Index)
			m.lastAppsIndexedAt = msg.Index.IndexedAt
		}
		m.recomputeResults()
		if m.cfg.AppsCachePath != "" {
			return m, ScheduleAppsRefresh(30 * time.Minute)
		}
		return m, nil

	case AppsRefreshTickMsg:
		if m.cfg.AppsProvider != nil && m.cfg.HomeDir != "" {
			return m, ReindexCmd(m.cfg.HomeDir, m.cfg.AppsCachePath)
		}
		return m, nil

	case AppLaunchResultMsg:
		if msg.Err != nil {
			m.status = "launch error: " + msg.Err.Error()
			return m, nil
		}
		m.input.Reset()
		m.results = nil
		m.selected = 0
		m.offset = 0
		m.status = ""
		if m.cfg.HideTerminal != nil {
			_ = m.cfg.HideTerminal()
		}
		return m, nil

	case ScriptRunMsg:
		m.status = ""
		if msg.Result.Err != nil {
			m.status = "script error: " + msg.Result.Stderr
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
		m.input.Reset()
		m.results = nil
		m.selected = 0
		m.offset = 0
		m.scriptOutput = nil
		if m.cfg.HideTerminal != nil {
			_ = m.cfg.HideTerminal()
		}
		return m, nil

	case clearStatusMsg:
		if m.status == "saved" {
			m.status = ""
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) recomputeResults() {
	if m.scriptOutput != nil {
		m.results = m.scriptOutput
		if m.selected >= len(m.results) {
			m.selected = 0
			m.offset = 0
		}
		return
	}
	query := m.input.Value()
	// Empty input: show recent history instead of provider results.
	if query == "" && m.cfg.History != nil && m.cfg.History.Len() > 0 {
		entries := m.cfg.History.Entries()
		m.results = make([]Result, len(entries))
		for i, e := range entries {
			m.results[i] = Result{
				Title:  e,
				Icon:   IconRoleHistory,
				Action: Action{Type: ActionRecall, Target: e},
			}
		}
		if m.selected >= len(m.results) {
			m.selected = 0
			m.offset = 0
		}
		return
	}
	var all []Result
	for _, p := range m.cfg.Providers {
		all = append(all, p.Query(query)...)
	}
	ranked := Rank(all)
	if len(ranked) > maxResults {
		ranked = ranked[:maxResults]
	}
	m.results = ranked
	if m.selected >= len(m.results) {
		m.selected = 0
		m.offset = 0
	}
}

func (m *Model) act(r Result) (tea.Cmd, error) {
	switch r.Action.Type {
	case ActionRecall:
		m.input.SetValue(r.Action.Target)
		m.historyIdx = -1
		m.scriptOutput = nil
		m.recomputeResults()
		return nil, nil // sync, don't hide
	case ActionCopy:
		if m.cfg.CopyText == nil {
			return nil, fmt.Errorf("copy not available")
		}
		m.recordHistory(m.input.Value())
		return nil, m.cfg.CopyText(r.Action.Target)
	case ActionLaunch:
		if m.cfg.LaunchApp == nil {
			return nil, fmt.Errorf("launch not available")
		}
		m.recordHistory(m.input.Value())
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
		m.recordHistory(m.input.Value())
		m.status = "running…"
		return ScriptRunCmd(s), nil
	default:
		return nil, fmt.Errorf("action not yet implemented")
	}
}

// recordHistory appends query to history and persists it if a path is configured.
func (m *Model) recordHistory(query string) {
	if m.cfg.History == nil {
		return
	}
	m.cfg.History.Append(strings.TrimSpace(query))
	m.saveHistory()
}

// saveHistory persists history to disk if HistoryPath is set.
func (m *Model) saveHistory() {
	if m.cfg.History == nil || m.cfg.HistoryPath == "" {
		return
	}
	_ = m.cfg.History.Save(m.cfg.HistoryPath)
}

// clearStatusAfter returns a Cmd that sends clearStatusMsg after d.
func clearStatusAfter(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return clearStatusMsg{}
	}
}

// visibleResultRows returns how many result rows fit in the current terminal.
func (m Model) visibleResultRows() int {
	// Layout: border top (1) + input (1) + separator (1) + footer (1) + border bottom (1) = 5 overhead
	// Plus config-error row if present
	overhead := 5
	if m.cfg.ConfigErr != nil {
		overhead++
	}
	n := m.height - overhead
	if n < 1 {
		n = 1
	}
	return n
}

func (m Model) View() tea.View {
	if m.helpMode {
		content := m.renderHelp()
		v := tea.NewView(content)
		v.AltScreen = true
		return v
	}

	inner := m.renderInner()
	w := m.width
	if w < 14 {
		w = 14
	}
	h := m.height
	if h < 6 {
		h = 6
	}
	content := borderStyle.Width(w).Height(h).Render(inner)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) renderInner() string {
	var sb strings.Builder

	// Config error notice (non-blocking)
	if m.cfg.ConfigErr != nil {
		sb.WriteString(errorStyle.Render("config: "+m.cfg.ConfigErr.Error()) + "\n")
	}

	// Input line
	sb.WriteString(m.input.View() + "\n")

	// Separator
	w := m.width - 4 // border (2) + padding (2)
	if w < 1 {
		w = 1
	}
	sb.WriteString(separatorStyle.Render(strings.Repeat("─", w)) + "\n")

	// Results viewport
	visibleRows := m.visibleResultRows()
	end := m.offset + visibleRows
	if end > len(m.results) {
		end = len(m.results)
	}
	for i := m.offset; i < end; i++ {
		sb.WriteString(m.renderResult(m.results[i], i == m.selected) + "\n")
	}

	// Blank lines to push footer to the bottom of the frame
	rendered := end - m.offset
	for range visibleRows - rendered {
		sb.WriteString("\n")
	}

	// Status / help footer
	footer := m.status
	if footer == "" {
		if m.input.Value() == "" && m.cfg.History != nil && m.cfg.History.Len() > 0 {
			footer = "Recent  ↑↓ select  enter: recall  ctrl+p/n: navigate  esc: dismiss"
		} else {
			footer = "↑↓ select  enter: act  ctrl+s: save  ctrl+p/n: history  esc: dismiss  ?: help"
		}
	}
	sb.WriteString(helpBarStyle.Width(w).Render(footer))

	return sb.String()
}

func (m Model) renderResult(r Result, selected bool) string {
	icon := ""
	if r.IconGlyph != "" && m.cfg.UseNerdFont {
		icon = r.IconGlyph + " "
	} else {
		icon = Icon(r.Icon, m.cfg.UseNerdFont)
	}
	w := m.width - 4
	if w < 10 {
		w = 10
	}

	// Title with fuzzy-match highlights
	title := m.renderTitle(r, selected)

	subtitle := ""
	if r.Subtitle != "" {
		subtitle = " " + subtitleStyle.Render(r.Subtitle)
	}

	line := icon + title + subtitle

	if selected {
		return resultSelectedStyle.Width(w).Render(line)
	}
	return resultNormalStyle.Width(w).Render(line)
}

// renderTitle renders the result title, highlighting fuzzy match positions.
func (m Model) renderTitle(r Result, selected bool) string {
	if len(r.MatchRanges) == 0 {
		return r.Title
	}
	// Build a set of highlight positions
	pos := make(map[int]bool, len(r.MatchRanges))
	for _, p := range r.MatchRanges {
		pos[p] = true
	}
	runes := []rune(r.Title)
	var sb strings.Builder
	for i, ch := range runes {
		s := string(ch)
		if pos[i] {
			if selected {
				sb.WriteString(resultSelectedHighlightStyle.Render(s))
			} else {
				sb.WriteString(resultHighlightStyle.Render(s))
			}
		} else {
			sb.WriteString(s)
		}
	}
	return sb.String()
}

func (m Model) renderHelp() string {
	lines := []string{
		helpTitleStyle.Render("blf launcher — help"),
		"",
		"  ↑ / ↓       select result",
		"  enter        act on selected result (copy / launch / run)",
		"  ctrl+p       recall previous history entry",
		"  ctrl+n       recall next history entry",
		"  ctrl+s       save current input to history",
		"  ctrl+r       reindex apps",
		"  esc          dismiss launcher and clear input",
		"  ?            toggle this help",
		"  ctrl+c       quit launcher process",
		"",
		helpHintStyle.Render("  press any key to close help"),
	}
	return strings.Join(lines, "\n")
}
