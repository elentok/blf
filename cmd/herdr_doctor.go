package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/elentok/blf/internal/herdr"
	"github.com/elentok/blf/internal/herdrplugin"
	"github.com/elentok/blf/internal/herdrsync"
	"github.com/spf13/cobra"
)

// errDoctorFailed signals that runHerdrDoctor already printed its report
// (one line per check) to stdout; callers should exit 1 without printing
// anything else. Mirrors errTabsFailed.
var errDoctorFailed = errors.New("")

// Status is one Check's outcome.
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusInfo Status = "info"
)

// Check is one doctor line.
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// TabRow is one row of doctor's dry-run table: herdrsync.TabPlan rendered for
// display, with `action` replacing a rename blf would never actually make.
type TabRow struct {
	TabID         string `json:"tab_id"`
	ActivePaneID  string `json:"active_pane_id"`
	Agent         string `json:"agent"`
	TerminalTitle string `json:"terminal_title"`
	Label         string `json:"label"`
	NewLabel      string `json:"new_label"`
	Action        string `json:"action"`
}

// FailedHook is one non-succeeded entry among the last 10 of `herdr plugin
// log list`. Information only; never fails the report.
type FailedHook struct {
	Time   string `json:"time"`
	Event  string `json:"event"`
	Detail string `json:"detail"`
}

// Report is doctor's full result, a plain data structure so it can be
// rendered as text or as JSON (ticket 10).
type Report struct {
	Checks      []Check
	Tabs        []TabRow
	FailedHooks []FailedHook
}

func (r *Report) add(name string, status Status, detail string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail})
}

// Failed reports whether any check in the report failed.
func (r Report) Failed() bool {
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			return true
		}
	}
	return false
}

func newHerdrDoctorCmd(d deps) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check blf's herdr integration (read-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHerdrDoctor(d, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the report as JSON")
	return cmd
}

func runHerdrDoctor(d deps, jsonOutput bool) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr doctor: %w", err)
	}

	report := Report{Tabs: []TabRow{}, FailedHooks: []FailedHook{}}

	socketPath, ok := d.lookupEnv("HERDR_SOCKET_PATH")
	insideHerdr := ok && socketPath != ""

	var client herdr.Client
	var ping herdr.PingResult
	socketOK := false

	if insideHerdr {
		client = d.newHerdrClient(socketPath)
		ping, err = client.Ping()
		if err != nil {
			report.add("socket", StatusFail, err.Error())
		} else {
			report.add("socket", StatusInfo,
				fmt.Sprintf("herdr %s (blf tested with %s)", ping.Version, herdr.TestedVersion))
			socketOK = true
		}
	}

	if socketOK {
		doctorSocketChecks(&report, d, client, ping, homeDir)
	}

	doctorEnvironmentChecks(&report, d, homeDir, insideHerdr, socketPath)

	if socketOK {
		doctorPluginLinkedCheck(&report, client, homeDir)
	}

	doctorManifestChecks(&report, d, homeDir)

	doctorFailedHooksCheck(&report, d)

	if jsonOutput {
		renderReportJSON(d.stdout, report)
	} else {
		renderReport(d.stdout, report)
	}

	if report.Failed() {
		return errDoctorFailed
	}
	return nil
}

// doctorSocketChecks runs the checks that need a reachable, pinged socket:
// the server version, the two reply-field-presence checks, and (from the
// same snapshot) the dry-run tab table, via herdrsync.PlanTabs so it never
// computes a tab's plan differently than a real sync would (ticket 04).
func doctorSocketChecks(report *Report, d deps, client herdr.Client, ping herdr.PingResult, home string) {
	if herdrplugin.VersionAtLeast(ping.Version, herdrplugin.MinHerdrVersion) {
		report.add("server version", StatusPass,
			fmt.Sprintf("%s >= %s", ping.Version, herdrplugin.MinHerdrVersion))
	} else {
		report.add("server version", StatusFail,
			fmt.Sprintf("%s < %s; restart the herdr server", ping.Version, herdrplugin.MinHerdrVersion))
	}

	snapshot, err := client.SessionSnapshot()
	if err != nil {
		report.add("session.snapshot reply", StatusFail, err.Error())
	} else {
		report.add("session.snapshot reply", StatusPass, "layouts, tabs, panes present")
		report.Tabs = doctorTabRows(snapshot, d.herdrLabel, home)
	}

	if _, err := client.WorkspaceList(); err != nil {
		report.add("workspace.list reply", StatusFail, err.Error())
	} else {
		report.add("workspace.list reply", StatusPass, "fields present")
	}
}

// doctorTabRows renders every tab's herdrsync.TabPlan as a dry-run row: `ok`
// when blf wouldn't change the label, `would rename` otherwise. Never calls
// TabRename. A tab PlanTabs couldn't compute (missing layout/pane) is left
// out, same as a real sync would report it as a failed tab rather than guess.
func doctorTabRows(snapshot herdr.SnapshotResult, labelFn func(herdr.PaneInfo, string) string, home string) []TabRow {
	plans, _ := herdrsync.PlanTabs(snapshot, labelFn, home)

	rows := make([]TabRow, 0, len(plans))
	for _, p := range plans {
		action := "ok"
		if p.NewLabel != "" && p.NewLabel != p.Label {
			action = "would rename"
		}
		rows = append(rows, TabRow{
			TabID:         p.TabID,
			ActivePaneID:  p.ActivePaneID,
			Agent:         p.Agent,
			TerminalTitle: p.TerminalTitle,
			Label:         p.Label,
			NewLabel:      p.NewLabel,
			Action:        action,
		})
	}
	return rows
}

// doctorEnvironmentChecks prints one informational line per environment
// variable, plus blf's own path/version and the debug log state.
// HERDR_SOCKET_PATH is the one check here that fails the run: unset, it's
// also the "outside herdr" signal, so no separate failing line is added for
// that elsewhere.
func doctorEnvironmentChecks(report *Report, d deps, homeDir string, insideHerdr bool, socketPath string) {
	report.add("HERDR_ENV", StatusInfo, envDetail(d, "HERDR_ENV"))

	if insideHerdr {
		report.add("HERDR_SOCKET_PATH", StatusPass, socketPath)
	} else {
		report.add("HERDR_SOCKET_PATH", StatusFail, "not set; not running inside herdr")
	}

	report.add("HERDR_TAB_ID", StatusInfo, envDetail(d, "HERDR_TAB_ID"))
	report.add("HERDR_PANE_ID", StatusInfo, envDetail(d, "HERDR_PANE_ID"))
	report.add("VIMRUNTIME", StatusInfo, envDetail(d, "VIMRUNTIME"))

	blfPath, err := resolveBlfPath(d)
	if err != nil {
		report.add("blf", StatusInfo, err.Error())
	} else {
		report.add("blf", StatusInfo, fmt.Sprintf("%s (%s)", blfPath, getVersion()))
	}

	exists, err := d.fileExists(herdrDebugMarkerPath(homeDir))
	if err != nil {
		report.add("debug log", StatusInfo, err.Error())
	} else if exists {
		report.add("debug log", StatusInfo, fmt.Sprintf("on (%s)", herdrDebugLogPath(homeDir)))
	} else {
		report.add("debug log", StatusInfo, "off")
	}
}

func envDetail(d deps, name string) string {
	if v, ok := d.lookupEnv(name); ok && v != "" {
		return v
	}
	return "(not set)"
}

// doctorPluginLinkedCheck checks that blf's plugin is linked with herdr: an
// entry in plugin.list whose plugin_root is the plugin folder. An empty
// array means "not linked", not a transport error.
func doctorPluginLinkedCheck(report *Report, client herdr.Client, homeDir string) {
	pluginDir := herdrplugin.Dir(homeDir)

	plugins, err := client.PluginList(herdrplugin.ID)
	if err != nil {
		report.add("plugin linked", StatusFail, err.Error())
		return
	}

	for _, p := range plugins {
		if p.PluginRoot == pluginDir {
			report.add("plugin linked", StatusPass, pluginDir)
			return
		}
	}

	report.add("plugin linked", StatusFail, "not linked; run `blf herdr install-plugin`")
}

// doctorManifestChecks is filesystem-only, so it runs whether or not blf is
// inside herdr. It regenerates the "expected" manifest from the on-disk
// one's own blf path (via ExtractBlfPath), so a path-only difference (a dev
// build checked against a Homebrew-installed manifest) doesn't fail the
// check, and then checks that same path exists and is executable.
func doctorManifestChecks(report *Report, d deps, homeDir string) {
	manifestPath := herdrplugin.ManifestPath(homeDir)

	onDisk, err := d.readFile(manifestPath)
	if err != nil {
		report.add("manifest", StatusFail,
			fmt.Sprintf("%v; run `blf herdr install-plugin`", err))
		return
	}

	blfPath, err := herdrplugin.ExtractBlfPath(string(onDisk))
	if err != nil {
		report.add("manifest", StatusFail,
			fmt.Sprintf("%v; run `blf herdr install-plugin`", err))
		return
	}

	expected := herdrplugin.GenerateManifest(blfPath)
	if string(onDisk) == expected {
		report.add("manifest", StatusPass, manifestPath)
	} else {
		report.add("manifest", StatusFail,
			fmt.Sprintf("%s differs from expected; run `blf herdr install-plugin`", manifestPath))
	}

	exists, err := d.fileExists(blfPath)
	if err == nil && exists && d.isExecutable(blfPath) {
		report.add("blf path", StatusPass, blfPath)
	} else {
		report.add("blf path", StatusFail, fmt.Sprintf("%s: not an executable file", blfPath))
	}
}

// herdrPluginLogEntry is one entry of `herdr plugin log list`'s reply. Event
// is set for a hook-triggered run, ActionID for a plugin-action run; exactly
// one of the two is non-empty.
type herdrPluginLogEntry struct {
	Event         string `json:"event"`
	ActionID      string `json:"action_id"`
	Status        string `json:"status"`
	StartedUnixMs int64  `json:"started_unix_ms"`
	Stderr        string `json:"stderr"`
	Error         string `json:"error"`
}

type herdrPluginLogListReply struct {
	Result struct {
		Logs []herdrPluginLogEntry `json:"logs"`
	} `json:"result"`
}

// doctorFailedHooksCheck shells out to the herdr CLI (fine off the hot path)
// for the non-succeeded runs among the last 10 plugin command log entries.
// Information only: a missing `herdr` binary or an unparsable reply just
// means no failed hooks are reported, never a failing check.
func doctorFailedHooksCheck(report *Report, d deps) {
	out, err := d.runCommand("herdr", "plugin", "log", "list", "--plugin", herdrplugin.ID, "--limit", "10")
	if err != nil {
		return
	}

	var reply herdrPluginLogListReply
	if err := json.Unmarshal(out, &reply); err != nil {
		return
	}

	entries := reply.Result.Logs
	if len(entries) > 10 {
		entries = entries[len(entries)-10:]
	}

	for _, e := range entries {
		if e.Status == "succeeded" || e.Status == "running" || e.Status == "" {
			continue
		}

		event := e.Event
		if event == "" {
			event = e.ActionID
		}
		detail := e.Error
		if detail == "" {
			detail = e.Stderr
		}

		report.FailedHooks = append(report.FailedHooks, FailedHook{
			Time:   time.UnixMilli(e.StartedUnixMs).UTC().Format(time.RFC3339),
			Event:  event,
			Detail: detail,
		})
	}
}

func renderReport(w io.Writer, report Report) {
	for _, c := range report.Checks {
		fmt.Fprintf(w, "[%s] %s", strings.ToUpper(string(c.Status)), c.Name)
		if c.Detail != "" {
			fmt.Fprintf(w, ": %s", c.Detail)
		}
		fmt.Fprintln(w)
	}

	renderTabs(w, report.Tabs)
	renderFailedHooks(w, report.FailedHooks)
}

func renderTabs(w io.Writer, tabs []TabRow) {
	if len(tabs) == 0 {
		return
	}

	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TAB\tACTIVE PANE\tAGENT\tTERMINAL TITLE\tLABEL\tNEW LABEL\tACTION")
	for _, t := range tabs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			t.TabID, t.ActivePaneID, t.Agent, t.TerminalTitle, t.Label, t.NewLabel, t.Action)
	}
	tw.Flush()
}

func renderFailedHooks(w io.Writer, hooks []FailedHook) {
	if len(hooks) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Failed hook runs:")
	for _, h := range hooks {
		fmt.Fprintf(w, "[INFO] %s %s: %s\n", h.Time, h.Event, h.Detail)
	}
}

// jsonReport is Report's JSON shape: `ok` summarizes report.Failed() since
// that's a method, not a stored field.
type jsonReport struct {
	OK          bool         `json:"ok"`
	Checks      []Check      `json:"checks"`
	Tabs        []TabRow     `json:"tabs"`
	FailedHooks []FailedHook `json:"failed_hooks"`
}

func renderReportJSON(w io.Writer, report Report) error {
	return json.NewEncoder(w).Encode(jsonReport{
		OK:          !report.Failed(),
		Checks:      report.Checks,
		Tabs:        report.Tabs,
		FailedHooks: report.FailedHooks,
	})
}
