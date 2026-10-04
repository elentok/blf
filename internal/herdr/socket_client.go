package herdr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

// socketClient is the production Client: one new connection per request,
// matching herdr's request-reply-per-line protocol (no multiplexing needed).
type socketClient struct {
	socketPath string
	seq        atomic.Int64
}

type request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type replyEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *replyError     `json:"error"`
}

type replyError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type paneIDParams struct {
	PaneID string `json:"pane_id"`
}

type tabIDParams struct {
	TabID string `json:"tab_id"`
}

type tabRenameParams struct {
	TabID string `json:"tab_id"`
	Label string `json:"label"`
}

type workspaceIDParams struct {
	WorkspaceID string `json:"workspace_id"`
}

type workspaceCreateParams struct {
	Label string `json:"label"`
	Cwd   string `json:"cwd,omitempty"`
	Focus bool   `json:"focus"`
}

type pluginIDParams struct {
	PluginID string `json:"plugin_id"`
}

// notUnderstood is the exact error strict decoding returns for a reply
// missing a field herdr always sends.
func notUnderstood() error {
	return fmt.Errorf("herdr socket reply not understood (blf tested with herdr %s); run blf herdr doctor", TestedVersion)
}

// do sends one request and returns its result as a field map, ready for the
// reqXxx/optXxx/child helpers below. The whole round trip (dial, write,
// read) is bounded by requestTimeout; there is no retry.
func (c *socketClient) do(method string, params any) (map[string]json.RawMessage, error) {
	deadline := time.Now().Add(requestTimeout)

	dialer := net.Dialer{Deadline: deadline}
	conn, err := dialer.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("herdr: connect %s: %w", c.socketPath, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("herdr: set deadline: %w", err)
	}

	id := fmt.Sprintf("blf-%d", c.seq.Add(1))
	data, err := json.Marshal(request{ID: id, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("herdr: build %s request: %w", method, err)
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("herdr: send %s request: %w", method, err)
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("herdr: read %s reply: %w", method, err)
	}

	var env replyEnvelope
	if err := json.Unmarshal([]byte(line), &env); err != nil {
		return nil, notUnderstood()
	}
	if env.Error != nil {
		return nil, fmt.Errorf("herdr: %s: %s", env.Error.Code, env.Error.Message)
	}

	return asObject(env.Result)
}

func asObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, notUnderstood()
	}
	return m, nil
}

// child extracts the named nested object from m, e.g. result.pane.
func child(m map[string]json.RawMessage, key string) (map[string]json.RawMessage, error) {
	raw, ok := m[key]
	if !ok {
		return nil, notUnderstood()
	}
	return asObject(raw)
}

func reqString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", notUnderstood()
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", notUnderstood()
	}
	return s, nil
}

// optString reads an optional field, read as "" when herdr omits it.
func optString(m map[string]json.RawMessage, key string) (string, error) {
	raw, ok := m[key]
	if !ok {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", notUnderstood()
	}
	return s, nil
}

func reqInt(m map[string]json.RawMessage, key string) (int, error) {
	raw, ok := m[key]
	if !ok {
		return 0, notUnderstood()
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, notUnderstood()
	}
	return n, nil
}

func reqBool(m map[string]json.RawMessage, key string) (bool, error) {
	raw, ok := m[key]
	if !ok {
		return false, notUnderstood()
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, notUnderstood()
	}
	return b, nil
}

func reqArray(m map[string]json.RawMessage, key string) ([]json.RawMessage, error) {
	raw, ok := m[key]
	if !ok {
		return nil, notUnderstood()
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, notUnderstood()
	}
	return arr, nil
}

func decodeLayoutInfo(m map[string]json.RawMessage) (LayoutInfo, error) {
	workspaceID, err := reqString(m, "workspace_id")
	if err != nil {
		return LayoutInfo{}, err
	}
	tabID, err := reqString(m, "tab_id")
	if err != nil {
		return LayoutInfo{}, err
	}
	focusedPaneID, err := reqString(m, "focused_pane_id")
	if err != nil {
		return LayoutInfo{}, err
	}
	zoomed, err := reqBool(m, "zoomed")
	if err != nil {
		return LayoutInfo{}, err
	}
	return LayoutInfo{WorkspaceID: workspaceID, TabID: tabID, FocusedPaneID: focusedPaneID, Zoomed: zoomed}, nil
}

func decodePaneInfo(m map[string]json.RawMessage) (PaneInfo, error) {
	paneID, err := reqString(m, "pane_id")
	if err != nil {
		return PaneInfo{}, err
	}
	terminalID, err := reqString(m, "terminal_id")
	if err != nil {
		return PaneInfo{}, err
	}
	workspaceID, err := reqString(m, "workspace_id")
	if err != nil {
		return PaneInfo{}, err
	}
	tabID, err := reqString(m, "tab_id")
	if err != nil {
		return PaneInfo{}, err
	}
	focused, err := reqBool(m, "focused")
	if err != nil {
		return PaneInfo{}, err
	}
	agentStatus, err := reqString(m, "agent_status")
	if err != nil {
		return PaneInfo{}, err
	}
	revision, err := reqInt(m, "revision")
	if err != nil {
		return PaneInfo{}, err
	}

	pane := PaneInfo{
		PaneID:      paneID,
		TerminalID:  terminalID,
		WorkspaceID: workspaceID,
		TabID:       tabID,
		Focused:     focused,
		AgentStatus: agentStatus,
		Revision:    revision,
	}

	for field, dest := range map[string]*string{
		"cwd":                     &pane.Cwd,
		"foreground_cwd":          &pane.ForegroundCwd,
		"agent":                   &pane.Agent,
		"label":                   &pane.Label,
		"title":                   &pane.Title,
		"terminal_title":          &pane.TerminalTitle,
		"terminal_title_stripped": &pane.TerminalTitleStripped,
		"display_agent":           &pane.DisplayAgent,
		"restore_error":           &pane.RestoreError,
	} {
		v, err := optString(m, field)
		if err != nil {
			return PaneInfo{}, err
		}
		*dest = v
	}

	return pane, nil
}

func decodeTabInfo(m map[string]json.RawMessage) (TabInfo, error) {
	tabID, err := reqString(m, "tab_id")
	if err != nil {
		return TabInfo{}, err
	}
	workspaceID, err := reqString(m, "workspace_id")
	if err != nil {
		return TabInfo{}, err
	}
	number, err := reqInt(m, "number")
	if err != nil {
		return TabInfo{}, err
	}
	label, err := reqString(m, "label")
	if err != nil {
		return TabInfo{}, err
	}
	focused, err := reqBool(m, "focused")
	if err != nil {
		return TabInfo{}, err
	}
	paneCount, err := reqInt(m, "pane_count")
	if err != nil {
		return TabInfo{}, err
	}
	agentStatus, err := reqString(m, "agent_status")
	if err != nil {
		return TabInfo{}, err
	}
	return TabInfo{
		TabID:       tabID,
		WorkspaceID: workspaceID,
		Number:      number,
		Label:       label,
		Focused:     focused,
		PaneCount:   paneCount,
		AgentStatus: agentStatus,
	}, nil
}

func decodeWorkspaceInfo(m map[string]json.RawMessage) (WorkspaceInfo, error) {
	workspaceID, err := reqString(m, "workspace_id")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	number, err := reqInt(m, "number")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	label, err := reqString(m, "label")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	focused, err := reqBool(m, "focused")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	paneCount, err := reqInt(m, "pane_count")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	tabCount, err := reqInt(m, "tab_count")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	activeTabID, err := reqString(m, "active_tab_id")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	agentStatus, err := reqString(m, "agent_status")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	return WorkspaceInfo{
		WorkspaceID: workspaceID,
		Number:      number,
		Label:       label,
		Focused:     focused,
		PaneCount:   paneCount,
		TabCount:    tabCount,
		ActiveTabID: activeTabID,
		AgentStatus: agentStatus,
	}, nil
}

// decodeWorkspaceListEntry decodes a workspace.list/workspace.create entry,
// which carries only the five fields herdr sends for those methods (unlike
// session.snapshot's workspaces, decodeWorkspaceInfo leaves PaneCount,
// TabCount and AgentStatus at their zero value here).
func decodeWorkspaceListEntry(m map[string]json.RawMessage) (WorkspaceInfo, error) {
	workspaceID, err := reqString(m, "workspace_id")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	number, err := reqInt(m, "number")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	label, err := reqString(m, "label")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	focused, err := reqBool(m, "focused")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	activeTabID, err := reqString(m, "active_tab_id")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	return WorkspaceInfo{
		WorkspaceID: workspaceID,
		Number:      number,
		Label:       label,
		Focused:     focused,
		ActiveTabID: activeTabID,
	}, nil
}

// decodePluginInfo decodes one plugin.list entry. plugin_id is read
// optionally since the spec only confirms plugin_root; plugin_root is the
// field doctor's "plugin linked" check actually relies on.
func decodePluginInfo(m map[string]json.RawMessage) (PluginInfo, error) {
	pluginRoot, err := reqString(m, "plugin_root")
	if err != nil {
		return PluginInfo{}, err
	}
	pluginID, err := optString(m, "plugin_id")
	if err != nil {
		return PluginInfo{}, err
	}
	return PluginInfo{PluginID: pluginID, PluginRoot: pluginRoot}, nil
}

func decodeSnapshotResult(m map[string]json.RawMessage) (SnapshotResult, error) {
	version, err := reqString(m, "version")
	if err != nil {
		return SnapshotResult{}, err
	}
	protocol, err := reqInt(m, "protocol")
	if err != nil {
		return SnapshotResult{}, err
	}

	workspacesRaw, err := reqArray(m, "workspaces")
	if err != nil {
		return SnapshotResult{}, err
	}
	workspaces := make([]WorkspaceInfo, 0, len(workspacesRaw))
	for _, raw := range workspacesRaw {
		obj, err := asObject(raw)
		if err != nil {
			return SnapshotResult{}, err
		}
		ws, err := decodeWorkspaceInfo(obj)
		if err != nil {
			return SnapshotResult{}, err
		}
		workspaces = append(workspaces, ws)
	}

	tabsRaw, err := reqArray(m, "tabs")
	if err != nil {
		return SnapshotResult{}, err
	}
	tabs := make([]TabInfo, 0, len(tabsRaw))
	for _, raw := range tabsRaw {
		obj, err := asObject(raw)
		if err != nil {
			return SnapshotResult{}, err
		}
		tab, err := decodeTabInfo(obj)
		if err != nil {
			return SnapshotResult{}, err
		}
		tabs = append(tabs, tab)
	}

	panesRaw, err := reqArray(m, "panes")
	if err != nil {
		return SnapshotResult{}, err
	}
	panes := make([]PaneInfo, 0, len(panesRaw))
	for _, raw := range panesRaw {
		obj, err := asObject(raw)
		if err != nil {
			return SnapshotResult{}, err
		}
		pane, err := decodePaneInfo(obj)
		if err != nil {
			return SnapshotResult{}, err
		}
		panes = append(panes, pane)
	}

	layoutsRaw, err := reqArray(m, "layouts")
	if err != nil {
		return SnapshotResult{}, err
	}
	layouts := make([]LayoutInfo, 0, len(layoutsRaw))
	for _, raw := range layoutsRaw {
		obj, err := asObject(raw)
		if err != nil {
			return SnapshotResult{}, err
		}
		layout, err := decodeLayoutInfo(obj)
		if err != nil {
			return SnapshotResult{}, err
		}
		layouts = append(layouts, layout)
	}

	return SnapshotResult{
		Version:    version,
		Protocol:   protocol,
		Workspaces: workspaces,
		Tabs:       tabs,
		Panes:      panes,
		Layouts:    layouts,
	}, nil
}

func (c *socketClient) Ping() (PingResult, error) {
	m, err := c.do("ping", struct{}{})
	if err != nil {
		return PingResult{}, err
	}
	version, err := reqString(m, "version")
	if err != nil {
		return PingResult{}, err
	}
	protocol, err := reqInt(m, "protocol")
	if err != nil {
		return PingResult{}, err
	}
	return PingResult{Version: version, Protocol: protocol}, nil
}

func (c *socketClient) PaneLayout(paneID string) (LayoutInfo, error) {
	m, err := c.do("pane.layout", paneIDParams{PaneID: paneID})
	if err != nil {
		return LayoutInfo{}, err
	}
	obj, err := child(m, "layout")
	if err != nil {
		return LayoutInfo{}, err
	}
	return decodeLayoutInfo(obj)
}

func (c *socketClient) PaneGet(paneID string) (PaneInfo, error) {
	m, err := c.do("pane.get", paneIDParams{PaneID: paneID})
	if err != nil {
		return PaneInfo{}, err
	}
	obj, err := child(m, "pane")
	if err != nil {
		return PaneInfo{}, err
	}
	return decodePaneInfo(obj)
}

func (c *socketClient) TabGet(tabID string) (TabInfo, error) {
	m, err := c.do("tab.get", tabIDParams{TabID: tabID})
	if err != nil {
		return TabInfo{}, err
	}
	obj, err := child(m, "tab")
	if err != nil {
		return TabInfo{}, err
	}
	return decodeTabInfo(obj)
}

func (c *socketClient) TabRename(tabID, label string) error {
	_, err := c.do("tab.rename", tabRenameParams{TabID: tabID, Label: label})
	return err
}

func (c *socketClient) SessionSnapshot() (SnapshotResult, error) {
	m, err := c.do("session.snapshot", struct{}{})
	if err != nil {
		return SnapshotResult{}, err
	}
	obj, err := child(m, "snapshot")
	if err != nil {
		return SnapshotResult{}, err
	}
	return decodeSnapshotResult(obj)
}

func (c *socketClient) WorkspaceList() ([]WorkspaceInfo, error) {
	m, err := c.do("workspace.list", struct{}{})
	if err != nil {
		return nil, err
	}
	raw, err := reqArray(m, "workspaces")
	if err != nil {
		return nil, err
	}
	workspaces := make([]WorkspaceInfo, 0, len(raw))
	for _, r := range raw {
		obj, err := asObject(r)
		if err != nil {
			return nil, err
		}
		ws, err := decodeWorkspaceListEntry(obj)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (c *socketClient) WorkspaceFocus(workspaceID string) error {
	_, err := c.do("workspace.focus", workspaceIDParams{WorkspaceID: workspaceID})
	return err
}

func (c *socketClient) WorkspaceCreate(label, cwd string) (WorkspaceInfo, error) {
	m, err := c.do("workspace.create", workspaceCreateParams{Label: label, Cwd: cwd, Focus: true})
	if err != nil {
		return WorkspaceInfo{}, err
	}
	obj, err := child(m, "workspace")
	if err != nil {
		return WorkspaceInfo{}, err
	}
	return decodeWorkspaceListEntry(obj)
}

func (c *socketClient) PluginList(pluginID string) ([]PluginInfo, error) {
	m, err := c.do("plugin.list", pluginIDParams{PluginID: pluginID})
	if err != nil {
		return nil, err
	}
	raw, err := reqArray(m, "plugins")
	if err != nil {
		return nil, err
	}
	plugins := make([]PluginInfo, 0, len(raw))
	for _, r := range raw {
		obj, err := asObject(r)
		if err != nil {
			return nil, err
		}
		p, err := decodePluginInfo(obj)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}
