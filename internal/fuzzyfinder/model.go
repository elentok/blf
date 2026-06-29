package fuzzyfinder

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// Config holds the configuration for a Model.
type Config struct {
	// RenderRow renders the content of row i (no gutter, no trailing newline).
	// selected is true when i is the selected index; the widget renders the
	// active-row gutter marker itself, so most rows ignore selected. Consumers
	// with the common icon/title/subtitle shape should build an Item and call
	// RenderItem rather than styling content by hand.
	RenderRow func(i int, selected bool) string
	// Footer is displayed in the footer bar.
	Footer string
	// ItemCount is the initial number of items.
	ItemCount int
}

// Model is an embedded TUI widget that renders a query input box, a scrollable
// ranked list, and border/separator/footer chrome. It is index-based and
// type-agnostic: the consumer provides a RenderRow callback and manages the
// items slice itself.
//
// The consumer's Update must see every KeyMsg first, handle its own bindings
// (enter, esc, ctrl+c, …), then forward remaining messages to widget.Update.
// The widget only consumes up/down/ctrl-k/ctrl-j for navigation and everything
// else for the textinput.
type Model struct {
	cfg       Config
	input     textinput.Model
	selected  int
	offset    int
	itemCount int
	width     int
	height    int
}

// New returns a Model ready to embed.
func New(cfg Config) Model {
	ti := textinput.New()
	ti.Prompt = inputPromptStyle.Render("> ")
	_ = ti.Focus()
	return Model{
		cfg:       cfg,
		input:     ti,
		itemCount: cfg.ItemCount,
	}
}

// Init returns the Cmd required to start the widget (textinput cursor blink).
// The consumer should include this in its own Init via tea.Batch.
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles widget-owned messages. Navigation keys (up/down/ctrl-k/ctrl-j)
// move the selection and adjust the viewport. All other messages are forwarded
// to the textinput.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p", "ctrl+k":
			if m.selected > 0 {
				m.selected--
				if m.selected < m.offset {
					m.offset = m.selected
				}
			}
			return m, nil

		case "down", "ctrl+n", "ctrl+j":
			if m.selected < m.itemCount-1 {
				m.selected++
				visible := m.visibleRows()
				if m.selected >= m.offset+visible {
					m.offset = m.selected - visible + 1
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the full widget (border + input + separator + rows + footer)
// as a string for embedding in a consumer's tea.View.
func (m Model) View() string {
	w := max(m.width-4, 1) // border (2) + padding (2)

	var sb strings.Builder

	sb.WriteString(m.input.View() + "\n")
	sb.WriteString(separatorStyle.Render(strings.Repeat("─", w)) + "\n")

	visible := m.visibleRows()
	end := min(m.offset+visible, m.itemCount)
	for i := m.offset; i < end; i++ {
		content := ""
		if m.cfg.RenderRow != nil {
			content = m.cfg.RenderRow(i, i == m.selected)
		}
		sb.WriteString(gutter(i == m.selected) + content + "\n")
	}

	// Blank lines to pin the footer at the bottom of the frame.
	rendered := end - m.offset
	for range visible - rendered {
		sb.WriteString("\n")
	}

	footer := m.cfg.Footer
	sb.WriteString(helpBarStyle.Width(w).Render(footer))

	bw := max(m.width, 14)
	bh := max(m.height, 6)
	return borderStyle.Width(bw).Height(bh).Render(sb.String())
}

// Query returns the current text input value.
func (m Model) Query() string {
	return m.input.Value()
}

// Selected returns the current selection index.
func (m Model) Selected() int {
	return m.selected
}

// SetSelected moves the selection to idx, clamped to [0, itemCount-1], and
// adjusts the viewport so the selection is visible.
func (m *Model) SetSelected(idx int) {
	if m.itemCount == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= m.itemCount {
		idx = m.itemCount - 1
	}
	m.selected = idx
	if m.selected < m.offset {
		m.offset = m.selected
	}
	visible := m.visibleRows()
	if m.selected >= m.offset+visible {
		m.offset = m.selected - visible + 1
	}
}

// SetItemCount updates the number of items and clamps the selection if needed.
func (m *Model) SetItemCount(n int) {
	m.itemCount = n
	if n == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	if m.selected >= n {
		m.selected = n - 1
	}
	if m.offset > m.selected {
		m.offset = m.selected
	}
}

// SetSize sets the widget's rendering dimensions. Call this when the consumer
// assigns the widget a sub-region (e.g. the left pane of a split layout) rather
// than forwarding WindowSizeMsg directly.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetQuery sets the text input value without processing it as a key event.
func (m *Model) SetQuery(s string) {
	m.input.SetValue(s)
}

// Offset returns the current viewport scroll offset.
func (m Model) Offset() int {
	return m.offset
}

// SetFooter updates the footer text rendered at the bottom of the widget.
func (m *Model) SetFooter(s string) {
	m.cfg.Footer = s
}

// visibleRows returns how many result rows fit within the current height.
// Overhead: border-top(1) + input(1) + separator(1) + footer(1) + border-bottom(1) = 5.
func (m Model) visibleRows() int {
	return max(m.height-5, 1)
}
