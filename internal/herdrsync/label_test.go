package herdrsync

import (
	"strings"
	"testing"

	"github.com/elentok/blf/internal/herdr"
)

func TestLabel(t *testing.T) {
	const home = "/home/david"

	tests := []struct {
		name string
		pane herdr.PaneInfo
		want string
	}{
		{
			name: "agent pane uses stripped title",
			pane: herdr.PaneInfo{Agent: "claude", TerminalTitleStripped: "Fix the bug"},
			want: "Fix the bug",
		},
		{
			name: "agent pane strips leading spinner and star",
			pane: herdr.PaneInfo{Agent: "claude", TerminalTitleStripped: "⠋⠙ ✳ Fix the bug"},
			want: "Fix the bug",
		},
		{
			name: "agent pane cuts to 25 runes",
			pane: herdr.PaneInfo{Agent: "claude", TerminalTitleStripped: strings.Repeat("a", 30)},
			want: strings.Repeat("a", 13) + "…" + strings.Repeat("a", 12),
		},
		{
			name: "agent pane empty title falls back to agent and foreground_cwd",
			pane: herdr.PaneInfo{Agent: "claude", ForegroundCwd: "/home/david/dev/blf", Cwd: "/home/david"},
			want: "claude: blf",
		},
		{
			name: "agent pane empty title with only spinner falls back to agent and foreground_cwd",
			pane: herdr.PaneInfo{Agent: "claude", TerminalTitleStripped: "⠋⠙ ✳  ", ForegroundCwd: "/home/david/dev/blf"},
			want: "claude: blf",
		},
		{
			name: "agent pane empty title and empty foreground_cwd falls back to cwd",
			pane: herdr.PaneInfo{Agent: "claude", Cwd: "/home/david/dev/blf"},
			want: "claude: blf",
		},
		{
			name: "agent pane fallback dir is not cut a second time past 25",
			pane: herdr.PaneInfo{Agent: "claude", Cwd: "/home/david"},
			want: "claude: ~",
		},
		{
			name: "shell pane uses title as is",
			pane: herdr.PaneInfo{TerminalTitleStripped: "nvim: blf"},
			want: "nvim: blf",
		},
		{
			name: "shell pane cuts to 40 runes",
			pane: herdr.PaneInfo{TerminalTitleStripped: strings.Repeat("b", 50)},
			want: strings.Repeat("b", 20) + "…" + strings.Repeat("b", 20),
		},
		{
			name: "shell pane empty title falls back to cwd",
			pane: herdr.PaneInfo{Cwd: "/home/david/dev/blf"},
			want: "blf",
		},
		{
			name: "shell pane empty title and cwd equal to home is tilde",
			pane: herdr.PaneInfo{Cwd: home},
			want: "~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Label(tt.pane, home)
			if got != tt.want {
				t.Errorf("Label(%+v, %q) = %q, want %q", tt.pane, home, got, tt.want)
			}
		})
	}
}
