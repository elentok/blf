package kitty

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// Status is the working/waiting/idle state of an agent window. When the agent
// reports its own state via the AGENT_STATE user var that value is
// authoritative (see statusForAgent); otherwise status falls back to the
// window's OSC title (see detectStatus), which can only distinguish working
// from idle — never waiting.
type Status string

const (
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusIdle    Status = "idle"
)

// agentStateUserVar is the Kitty per-window user var an agent sets to report
// its own status; when present and recognized it overrides the title signal.
const agentStateUserVar = "AGENT_STATE"

// Agent is a Kitty window running a known AI coding agent. The JSON field names
// are the stable contract consumed by external callers (e.g. the nvim
// send-to-agent feature).
type Agent struct {
	ID      int    `json:"id"`
	Name    string `json:"agent"`
	Status  Status `json:"status"`
	Dir     string `json:"dir"`
	Title   string `json:"title"`
	Session string `json:"session"`

	// LastFocusedAt drives recency sorting only; it is not part of the JSON
	// contract.
	LastFocusedAt float64 `json:"-"`
}

// knownAgents are the command words that identify an agent window. Matching is
// always against a whole command word (a path component basename), never a
// substring of the full cmdline.
var knownAgents = map[string]struct{}{
	"claude":       {},
	"codex":        {},
	"opencode":     {},
	"cursor-agent": {},
}

var (
	workingStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	waitingStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	idleStatusStyle    = lipgloss.NewStyle().Faint(true)
	agentNameStyle     = lipgloss.NewStyle().Faint(true)
	titleStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

// Nerd Font status glyphs (Material Design convention, nf-md-*). Working has no
// glyph (a plain space) so the listing stays quiet while an agent is busy.
const (
	workingGlyph = " "
	waitingGlyph = " "
	idleGlyph    = " "
)

// ListAgents lists every agent window across all OS windows and sessions,
// drops the currently-focused window (identified via KITTY_WINDOW_ID), and
// sorts working agents first, then by most-recently focused.
func ListAgents(d Deps) ([]Agent, error) {
	windows, err := ListOSWindows(d)
	if err != nil {
		return nil, err
	}

	currentID := currentWindowID(d)

	agents := []Agent{}
	for _, osWindow := range windows {
		for _, tab := range osWindow.Tabs {
			for _, window := range tab.Windows {
				if window.ID == currentID {
					continue
				}

				name, ok := detectAgentName(window)
				if !ok {
					continue
				}

				agents = append(agents, Agent{
					ID:            window.ID,
					Name:          name,
					Status:        statusForAgent(name, window.Title, window.UserVars),
					Dir:           agentDir(window),
					Title:         cleanTitle(window.Title),
					Session:       agentSession(window, tab),
					LastFocusedAt: window.LastFocusedAt,
				})
			}
		}
	}

	sortAgents(agents)
	return agents, nil
}

// detectAgentName decides whether a window is running a known agent and which
// one. The primary signal is the last reported cmdline; the backup is each
// foreground process's cmdline. In both cases a token matches only when its
// path basename is exactly a known agent name, so a path such as
// /private/tmp/claude-501/... never counts while a shell wrapper like
// `/bin/sh /usr/bin/command claude` does.
func detectAgentName(window Window) (string, bool) {
	if name, ok := matchAgentToken(strings.Fields(window.LastReportedCmdline)); ok {
		return name, true
	}

	for _, proc := range window.ForegroundProcesses {
		if name, ok := matchAgentToken(proc.Cmdline); ok {
			return name, true
		}
	}

	return "", false
}

func matchAgentToken(tokens []string) (string, bool) {
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		base := filepath.Base(token)
		if _, ok := knownAgents[base]; ok {
			return base, true
		}
	}
	return "", false
}

// statusForAgent reports an agent's status, combining two signals:
//
//   - The OSC title's leading braille spinner is the *live* "working" signal —
//     the agent emits it continuously while generating, and drops it when it
//     stops. It overrides AGENT_STATE because that var is event-driven (set from
//     hooks) and goes stale when a working→idle transition is missed.
//   - The AGENT_STATE user var is the only source of **waiting** (the title
//     can't express it), so it owns status whenever there is no live spinner.
//
// Precedence: live spinner → working; else a recognized AGENT_STATE var; else
// the title fallback. OpenCode has no title signal, so it never reports working
// from the title and reads idle without a user var (known limitation).
func statusForAgent(name, title string, userVars map[string]string) Status {
	if name != "opencode" && detectStatus(title) == StatusWorking {
		return StatusWorking
	}
	if state, ok := statusFromUserVars(userVars); ok {
		return state
	}
	if name == "opencode" {
		return StatusIdle
	}
	return detectStatus(title)
}

// statusFromUserVars returns the agent-reported status when AGENT_STATE holds a
// recognized value; ok is false for a missing, empty, or unrecognized var so
// the caller falls back to the title signal.
func statusFromUserVars(userVars map[string]string) (Status, bool) {
	switch Status(userVars[agentStateUserVar]) {
	case StatusWorking:
		return StatusWorking, true
	case StatusWaiting:
		return StatusWaiting, true
	case StatusIdle:
		return StatusIdle, true
	default:
		return "", false
	}
}

// detectStatus derives status from a window title only: a leading braille
// spinner rune (U+2800-U+28FF) means working, anything else means idle.
func detectStatus(title string) Status {
	for _, r := range title {
		if r == ' ' || r == '\t' {
			continue
		}
		if r >= 0x2800 && r <= 0x28FF {
			return StatusWorking
		}
		return StatusIdle
	}
	return StatusIdle
}

// cleanTitle strips leading status decorations (a braille spinner — the
// transient working marker — or Claude's "✳" prefix) and surrounding whitespace
// so the displayed/serialized title is the agent's task summary without noise.
func cleanTitle(title string) string {
	trimmed := strings.TrimLeft(title, " \t")
	for len(trimmed) > 0 {
		r, size := utf8.DecodeRuneInString(trimmed)
		if (r >= 0x2800 && r <= 0x28FF) || r == '✳' {
			trimmed = strings.TrimLeft(trimmed[size:], " \t")
			continue
		}
		break
	}
	return trimmed
}

func agentDir(window Window) string {
	cwd := window.Cwd
	if strings.TrimSpace(cwd) == "" {
		for _, proc := range window.ForegroundProcesses {
			if strings.TrimSpace(proc.Cwd) != "" {
				cwd = proc.Cwd
				break
			}
		}
	}
	if strings.TrimSpace(cwd) == "" {
		return ""
	}
	return filepath.Base(cwd)
}

func agentSession(window Window, tab Tab) string {
	if strings.TrimSpace(window.SessionName) != "" {
		return window.SessionName
	}
	return tab.SessionName
}

func currentWindowID(d Deps) int {
	if d.LookupEnv == nil {
		return -1
	}
	value, ok := d.LookupEnv("KITTY_WINDOW_ID")
	if !ok {
		return -1
	}
	id, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return -1
	}
	return id
}

// statusRank orders the status tiers for sorting: waiting (most urgent) first,
// then working, then idle.
func statusRank(status Status) int {
	switch status {
	case StatusWaiting:
		return 0
	case StatusWorking:
		return 1
	default:
		return 2
	}
}

func sortAgents(agents []Agent) {
	sort.SliceStable(agents, func(i, j int) bool {
		left, right := agents[i], agents[j]
		leftRank, rightRank := statusRank(left.Status), statusRank(right.Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return left.LastFocusedAt > right.LastFocusedAt
	})
}

// FormatAgents renders a readable, one-per-line listing for `list-agents`.
func FormatAgents(agents []Agent) string {
	var b strings.Builder
	for _, agent := range agents {
		b.WriteString(formatAgentRow(agent))
		b.WriteByte('\n')
	}
	return b.String()
}

// formatAgentRow is the visible row: <status> <dir>: <title(blue)> (<agent(dim)>).
func formatAgentRow(agent Agent) string {
	return fmt.Sprintf("%s %s: %s (%s)",
		statusGlyph(agent.Status),
		agent.Dir,
		titleStyle.Render(agent.Title),
		agentNameStyle.Render(agent.Name),
	)
}

func statusGlyph(status Status) string {
	switch status {
	case StatusWorking:
		return workingStatusStyle.Render(workingGlyph)
	case StatusWaiting:
		return waitingStatusStyle.Render(waitingGlyph)
	default:
		return idleStatusStyle.Render(idleGlyph)
	}
}

// RenderAgentPreview returns a screen snapshot of an agent window for the fzf
// preview pane.
func RenderAgentPreview(id string, d Deps) (string, error) {
	if d.RunCommand == nil {
		return "", errors.New("kitty command runner is not configured")
	}

	out, err := d.RunCommand("kitty", "@", "get-text", "--match", "id:"+id, "--extent", "screen")
	if err != nil {
		return "", fmt.Errorf("get text for kitty agent window %s: %w", id, err)
	}
	return string(out), nil
}

// ListAgentsCommand backs `blf kitty list-agents [--json]`.
func ListAgentsCommand(asJSON bool, d Deps) error {
	agents, err := ListAgents(d)
	if err != nil {
		return err
	}

	if asJSON {
		out, err := json.MarshalIndent(agents, "", "  ")
		if err != nil {
			return err
		}
		_, err = d.Stdout.Write(append(out, '\n'))
		return err
	}

	_, err = io.WriteString(d.Stdout, FormatAgents(agents))
	return err
}
