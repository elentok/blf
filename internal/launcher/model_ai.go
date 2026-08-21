package launcher

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher/ai"
)

// aiPromptFields holds the ai-prompt-mode state embedded anonymously in
// Model, so its fields promote as m.aiPromptKind etc. while their
// declaration lives here rather than in model.go.
type aiPromptFields struct {
	aiPromptKind      AIPromptKind // "" = not in ai prompt mode; else the kind that entered it
	clipboardSnapshot string       // clipboard contents read once at ai prompt mode entry
	aiPromptError     string       // inline error shown in the footer in ai prompt mode; cleared on the next keystroke
}

// AIRunDoneMsg carries the outcome of one ai run dispatched from ai prompt
// mode's Enter handler; see handleAIRunDone for how completion is handled.
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
	lines := []string{clipboardPreviewHeaderStyle.Render(clipboardPreviewHeader), ""}
	if m.clipboardSnapshot != "" {
		lines = append(lines, strings.Split(m.clipboardSnapshot, "\n")...)
	}
	return lines
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

// handleAIRunDone processes a completed ai run. It always appends the run to
// the runs store and shows a notification titled by the kind; on success it
// also copies the response to the clipboard, and on failure the title
// carries a failure marker instead and the clipboard is left untouched, so a
// failure can never destroy what the user had copied. No launcher-history
// entry is written — runs are surfaced through a separate path.
//
// The store write happens here, in the update loop, rather than in the
// goroutine that produced msg — bubbletea serialises Update calls, which is
// what makes unbounded concurrent runs safe against racing on the store
// file.
//
// Results are recomputed only when the input is empty: a run completing
// after the reset-and-hide path must repopulate the pre-populated list (see
// resetAndHide), but a run completing while the user has a query typed must
// leave their in-progress results alone.
func (m *Model) handleAIRunDone(msg AIRunDoneMsg) {
	run := ai.Run{
		ID:        newRunID(),
		Timestamp: time.Now(),
		Kind:      ai.Kind(msg.Kind),
		Input:     msg.Input,
		Response:  msg.Result.Response,
		Status:    msg.Result.Status,
	}
	if m.cfg.AIRunsStore != nil {
		m.cfg.AIRunsStore.Append(run)
		m.saveAIRuns()
	}

	title := string(msg.Kind)
	body := msg.Result.Response
	if msg.Result.Status == ai.StatusSuccess {
		if m.cfg.CopyText != nil {
			_ = m.cfg.CopyText(msg.Result.Response)
		}
	} else {
		title = "✗ " + string(msg.Kind)
		if msg.Result.Err != nil {
			body = msg.Result.Err.Error()
		}
	}
	if m.cfg.ShowNotification != nil {
		_ = m.cfg.ShowNotification(title, body)
	}

	if m.input.Value() == "" {
		m.recomputeResults()
	}
}

// saveAIRuns persists the ai runs store to disk if AIRunsStorePath is set.
func (m *Model) saveAIRuns() {
	if m.cfg.AIRunsStore == nil || m.cfg.AIRunsStorePath == "" {
		return
	}
	_ = m.cfg.AIRunsStore.Save(m.cfg.AIRunsStorePath)
}

// newRunID returns a random hex id for a runs-store record.
func newRunID() string {
	var b [8]byte
	_, _ = crand.Read(b[:])
	return hex.EncodeToString(b[:])
}
