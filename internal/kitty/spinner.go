package kitty

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type spinnerTickMsg struct{}

// Spinner manages an animated frame sequence across bubbletea value copies.
// frameRef is heap-allocated so it survives model copies.
type Spinner struct {
	frames   []string
	frameRef *int
	interval time.Duration
}

func newSpinner(frames []string, interval time.Duration) Spinner {
	return Spinner{frames: frames, frameRef: new(int), interval: interval}
}

// Frame returns the current frame string.
func (s Spinner) Frame() string {
	return s.frames[*s.frameRef%len(s.frames)]
}

// Advance moves to the next frame.
func (s Spinner) Advance() {
	*s.frameRef = (*s.frameRef + 1) % len(s.frames)
}

// TickCmd returns a Cmd that fires a spinnerTickMsg after the interval.
func (s Spinner) TickCmd() tea.Cmd {
	return func() tea.Msg {
		<-time.After(s.interval)
		return spinnerTickMsg{}
	}
}
