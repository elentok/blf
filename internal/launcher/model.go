package launcher

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/elentok/blf/internal/fuzzyfinder"
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
	OpenTarget      func(string) error // optional; opens a file/URL via `open` (no -a)
	UseNerdFont     bool
	CurrencyCache   *currency.Cache  // optional; nil disables currency refresh
	AppsProvider    *AppsProvider    // optional; nil disables app search
	AppsCachePath   string           // path to apps.json; empty disables refresh
	HomeDir         string           // used by ReindexCmd
	ScriptsProvider *ScriptsProvider // optional; nil disables script execution
	History         *history.History // optional; nil disables history
	HistoryPath     string           // path to persist history; empty skips persistence
	HideDelay       time.Duration    // delay before hiding the terminal (see resetAndHide); 0 = immediate
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
	status            string    // transient status / error message
	lastAppsIndexedAt time.Time // mtime of last loaded apps cache
	scriptOutput      []Result  // non-nil after a "show" script; overrides provider results
	historyIdx        int       // -1 = not navigating; >=0 = index into history entries
}

// NewModel creates a launcher Model ready to run.
func NewModel(cfg ModelConfig) Model {
	queryRef := new(string)
	resultsRef := new([]Result)
	widthRef := new(int)

	m := Model{
		cfg:        cfg,
		historyIdx: -1,
		resultsRef: resultsRef,
		widthRef:   widthRef,
	}

	useNerdFont := cfg.UseNerdFont
	m.widget = fuzzyfinder.New(fuzzyfinder.Config{
		RenderRow: func(i int, selected bool) string {
			results := *resultsRef
			if i >= len(results) {
				return ""
			}
			return renderResultRow(results[i], useNerdFont, selected)
		},
		Footer:    "?: help",
		ItemCount: 0,
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
		switch key {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Reset and hide the quick terminal (ADR 0002)
			if len(m.results) > 0 {
				role := m.results[0].Icon
				if role == IconRoleCalc || role == IconRoleUnit || role == IconRoleCurrency {
					m.recordHistory(m.input.Value())
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
			m.setQuery(entries[next])
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
				m.setQuery(m.cfg.History.Entries()[next])
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
			m.updateFooter()
			return m, clearStatusAfter(1500 * time.Millisecond)

		case "ctrl+x":
			// Delete the selected entry from history (only when history rows are shown).
			if m.cfg.History == nil || len(m.results) == 0 {
				return m, nil
			}
			result := m.results[m.selected]
			if result.Action.Type != ActionRecall {
				return m, nil
			}
			if m.cfg.History.Remove(result.Action.Target) {
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
		m.status = ""
		if msg.Err == nil && msg.Index != nil && m.cfg.AppsProvider != nil {
			m.cfg.AppsProvider.SetIndex(msg.Index)
			m.lastAppsIndexedAt = msg.Index.IndexedAt
		}
		m.recomputeResults()
		m.updateFooter()
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

	case clearStatusMsg:
		if m.status == "saved" {
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
		m.syncWidget()
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
	m.syncWidget()
}

// syncWidget keeps the fuzzyfinder widget in sync with m.results and m.selected.
func (m *Model) syncWidget() {
	*m.resultsRef = m.results
	m.widget.SetItemCount(max(len(m.results), 1))
	m.widget.SetSelected(m.selected)
}

// updateFooter updates the widget footer to reflect the current status/configErr.
func (m *Model) updateFooter() {
	if m.status != "" {
		m.widget.SetFooter(m.status)
	} else if m.cfg.ConfigErr != nil {
		m.widget.SetFooter("config: " + m.cfg.ConfigErr.Error())
	} else {
		m.widget.SetFooter("?: help")
	}
}

func (m *Model) act(r Result) (tea.Cmd, error) {
	switch r.Action.Type {
	case ActionRecall:
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
		m.updateFooter()
		return ScriptRunCmd(s), nil
	case ActionOpen:
		if m.cfg.OpenTarget == nil {
			return nil, fmt.Errorf("open not available")
		}
		m.recordHistory(m.input.Value())
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

// resetAndHide clears the launcher back to its empty state and returns a Cmd that
// hides the Kitty quick terminal.
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
// Layout: border top (1) + input (1) + separator (1) + footer (1) + border bottom (1) = 5 overhead.
func (m Model) visibleResultRows() int {
	n := m.height - 5
	if n < 1 {
		n = 1
	}
	return n
}

func (m Model) View() tea.View {
	if m.helpMode {
		w := max(m.width, 14)
		h := max(m.height, 6)
		content := borderStyle.Width(w).Height(h).Render(m.renderHelp())
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
