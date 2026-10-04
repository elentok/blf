package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/elentok/blf/internal/herdr"
	"github.com/elentok/blf/internal/herdrplugin"
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
	Name   string
	Status Status
	Detail string
}

// Report is doctor's full result, a plain data structure so ticket 10 can
// later render it as JSON instead of (or alongside) text.
type Report struct {
	Checks []Check
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
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check blf's herdr integration (read-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHerdrDoctor(d)
		},
	}
}

func runHerdrDoctor(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr doctor: %w", err)
	}

	report := Report{}

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
		doctorSocketChecks(&report, client, ping)
	}

	doctorEnvironmentChecks(&report, d, homeDir, insideHerdr, socketPath)

	if socketOK {
		doctorPluginLinkedCheck(&report, client, homeDir)
	}

	doctorManifestChecks(&report, d, homeDir)

	renderReport(d.stdout, report)

	if report.Failed() {
		return errDoctorFailed
	}
	return nil
}

// doctorSocketChecks runs the checks that need a reachable, pinged socket:
// the server version and the two reply-field-presence checks.
func doctorSocketChecks(report *Report, client herdr.Client, ping herdr.PingResult) {
	if herdrplugin.VersionAtLeast(ping.Version, herdrplugin.MinHerdrVersion) {
		report.add("server version", StatusPass,
			fmt.Sprintf("%s >= %s", ping.Version, herdrplugin.MinHerdrVersion))
	} else {
		report.add("server version", StatusFail,
			fmt.Sprintf("%s < %s; restart the herdr server", ping.Version, herdrplugin.MinHerdrVersion))
	}

	if _, err := client.SessionSnapshot(); err != nil {
		report.add("session.snapshot reply", StatusFail, err.Error())
	} else {
		report.add("session.snapshot reply", StatusPass, "layouts, tabs, panes present")
	}

	if _, err := client.WorkspaceList(); err != nil {
		report.add("workspace.list reply", StatusFail, err.Error())
	} else {
		report.add("workspace.list reply", StatusPass, "fields present")
	}
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

func renderReport(w io.Writer, report Report) {
	for _, c := range report.Checks {
		fmt.Fprintf(w, "[%s] %s", strings.ToUpper(string(c.Status)), c.Name)
		if c.Detail != "" {
			fmt.Fprintf(w, ": %s", c.Detail)
		}
		fmt.Fprintln(w)
	}
}
