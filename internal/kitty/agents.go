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

// Status is the coarse working/idle state of an agent window. It is derived
// solely from the window's OSC title (see detectStatus); there is deliberately
// no "waiting"/"blocked" state.
type Status string

const (
	StatusWorking Status = "working"
	StatusIdle    Status = "idle"
)

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
	idleStatusStyle    = lipgloss.NewStyle().Faint(true)
	agentNameStyle     = lipgloss.NewStyle().Faint(true)
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
					Status:        statusForAgent(name, window.Title),
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

// statusForAgent reports whether an agent is working or idle. OpenCode has no
// title status signal, so it always reads idle (known limitation).
func statusForAgent(name, title string) Status {
	if name == "opencode" {
		return StatusIdle
	}
	return detectStatus(title)
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

// cleanTitle strips a leading braille spinner (the transient working marker) and
// surrounding whitespace so the displayed/serialized title is the agent's task
// summary without status noise.
func cleanTitle(title string) string {
	trimmed := strings.TrimLeft(title, " \t")
	for len(trimmed) > 0 {
		r, size := utf8.DecodeRuneInString(trimmed)
		if r >= 0x2800 && r <= 0x28FF {
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

func sortAgents(agents []Agent) {
	sort.SliceStable(agents, func(i, j int) bool {
		left, right := agents[i], agents[j]
		leftWorking := left.Status == StatusWorking
		rightWorking := right.Status == StatusWorking
		if leftWorking != rightWorking {
			return leftWorking
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

// formatAgentChoices renders the fzf input: each line is the Kitty window id in
// a hidden tab-delimited field followed by the visible row (sessions-picker
// --delimiter/--with-nth pattern).
func formatAgentChoices(agents []Agent) string {
	var b strings.Builder
	for _, agent := range agents {
		b.WriteString(strconv.Itoa(agent.ID))
		b.WriteByte('\t')
		b.WriteString(formatAgentRow(agent))
		b.WriteByte('\n')
	}
	return b.String()
}

// formatAgentRow is the visible row: <status>  <dir>  <title>  <agent(dim)>.
func formatAgentRow(agent Agent) string {
	return strings.Join([]string{
		statusGlyph(agent.Status),
		agent.Dir,
		agent.Title,
		agentNameStyle.Render(agent.Name),
	}, "  ")
}

func statusGlyph(status Status) string {
	if status == StatusWorking {
		return workingStatusStyle.Render("●")
	}
	return idleStatusStyle.Render("○")
}

// parseAgentSelection extracts the Kitty window id from a selected fzf line
// (round-trips formatAgentChoices).
func parseAgentSelection(line string) (string, error) {
	plain := ansiPattern.ReplaceAllString(strings.TrimSpace(line), "")
	if plain == "" {
		return "", nil
	}
	id, _, found := strings.Cut(plain, "\t")
	if !found {
		return "", invalidAgentSelection(plain)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", invalidAgentSelection(plain)
	}
	if _, err := strconv.Atoi(id); err != nil {
		return "", invalidAgentSelection(plain)
	}
	return id, nil
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

func invalidAgentSelection(selection string) error {
	return fmt.Errorf("invalid kitty agent selection %q", selection)
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
