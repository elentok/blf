// Package herdr talks to the herdr terminal multiplexer over its Unix
// socket (ADR 0010: the socket, not the herdr CLI, because spawning herdr
// costs ~4ms per call against ~0.1ms for the socket).
package herdr

import "time"

// TestedVersion is the herdr version this client's reply shapes were
// verified against. It appears in the strict-decoding error so a user can
// tell a real protocol drift from a bug.
const TestedVersion = "0.9.3"

// requestTimeout bounds one request end to end (dial, write, read). herdr
// requests are a few tenths of a millisecond normally; 200ms with no retry
// catches a dead or hung server without stalling a fish prompt.
const requestTimeout = 200 * time.Millisecond

// PingResult is herdr's reply to ping.
type PingResult struct {
	Version  string
	Protocol int
}

// LayoutInfo is a tab's layout: which pane is focused, plus the tab/workspace
// it belongs to.
type LayoutInfo struct {
	WorkspaceID   string
	TabID         string
	FocusedPaneID string
	Zoomed        bool
}

// PaneInfo is one pane. Cwd, ForegroundCwd, Agent, Label, Title,
// TerminalTitle, TerminalTitleStripped, DisplayAgent and RestoreError are
// omitted by herdr when unset and read as "".
type PaneInfo struct {
	PaneID      string
	TerminalID  string
	WorkspaceID string
	TabID       string
	Focused     bool
	AgentStatus string
	Revision    int

	Cwd                   string
	ForegroundCwd         string
	Agent                 string
	Label                 string
	Title                 string
	TerminalTitle         string
	TerminalTitleStripped string
	DisplayAgent          string
	RestoreError          string
}

// TabInfo is one tab.
type TabInfo struct {
	TabID       string
	WorkspaceID string
	Number      int
	Label       string
	Focused     bool
	PaneCount   int
	AgentStatus string
}

// WorkspaceInfo is one workspace.
type WorkspaceInfo struct {
	WorkspaceID string
	Number      int
	Label       string
	Focused     bool
	PaneCount   int
	TabCount    int
	ActiveTabID string
	AgentStatus string
}

// SnapshotResult is the reply to session.snapshot: every workspace, tab and
// pane in the session, plus each tab's layout.
type SnapshotResult struct {
	Version    string
	Protocol   int
	Workspaces []WorkspaceInfo
	Tabs       []TabInfo
	Panes      []PaneInfo
	Layouts    []LayoutInfo
}

// Client talks to a herdr server over its socket. Commands take it behind
// their deps struct so tests can inject a fake instead of a real socket.
type Client interface {
	// Ping checks the server is reachable and reports its version/protocol.
	Ping() (PingResult, error)
	// PaneLayout reports which pane is focused in paneID's tab.
	PaneLayout(paneID string) (LayoutInfo, error)
	// PaneGet reads one pane.
	PaneGet(paneID string) (PaneInfo, error)
	// TabGet reads one tab.
	TabGet(tabID string) (TabInfo, error)
	// TabRename sets tabID's label. herdr stores an empty label as given;
	// callers that mean "skip" must not call this with label == "".
	TabRename(tabID, label string) error
	// SessionSnapshot reads the whole session in one call.
	SessionSnapshot() (SnapshotResult, error)
	// WorkspaceList lists every workspace, in herdr's UI order.
	WorkspaceList() ([]WorkspaceInfo, error)
	// WorkspaceFocus focuses a workspace. A no-op success when already focused.
	WorkspaceFocus(workspaceID string) error
	// WorkspaceCreate creates a workspace with label and an optional starting
	// directory (cwd == "" lets herdr pick its default), and focuses it.
	WorkspaceCreate(label, cwd string) (WorkspaceInfo, error)
}

// New returns a Client that talks to the herdr server listening on
// socketPath (normally $HERDR_SOCKET_PATH).
func New(socketPath string) Client {
	return &socketClient{socketPath: socketPath}
}
