package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/power"
	"github.com/spf13/cobra"
)

var powerReportBoldStyle = lipgloss.NewStyle().Bold(true)

// daemonEnvVar, when set to "1" in the process environment, marks this
// process as the already-re-exec'd daemon (see spawnDaemon) rather than a
// fresh `blf power start` invocation.
const daemonEnvVar = "BLF_POWER_DAEMON"

const sampleInterval = 30 * time.Second

var (
	stopPollInterval = 100 * time.Millisecond
	stopTimeout      = 5 * time.Second
)

func newPowerCmd(d deps) *cobra.Command {
	powerCmd := &cobra.Command{
		Use:   "power",
		Short: "Background power/battery monitor",
	}

	powerCmd.AddCommand(
		newPowerStartCmd(d),
		newPowerStopCmd(d),
		newPowerStatusCmd(d),
		newPowerReportCmd(d),
		newPowerWatchCmd(d),
	)

	return powerCmd
}

func newPowerStartCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the background power-sampling daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPowerStart(d)
		},
	}
}

func newPowerStopCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background power-sampling daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPowerStop(d)
		},
	}
}

func newPowerStatusCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the background power-sampling daemon's status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPowerStatus(d)
		},
	}
}

func newPowerReportCmd(d deps) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Summarize a window of sampled power/battery data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPowerReport(d, since)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "window to summarize, e.g. 24h, 7d, 3d")
	return cmd
}

func newPowerWatchCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Tail today's power samples live",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer stop()
			return runPowerWatch(ctx, d)
		},
	}
}

// watchPollInterval is how often runPowerWatch re-reads the log file looking
// for newly appended lines.
var watchPollInterval = 500 * time.Millisecond

var (
	watchTimeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	watchPowerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	watchThermalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	watchOnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	watchOffStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// runPowerWatch tails today's day-file, rendering samples as they're
// appended, until ctx is cancelled (SIGINT/SIGTERM). It reads the whole file
// on each poll (day-files are small) and tracks how many lines it has
// already rendered rather than a byte offset, so restarts never re-render.
func runPowerWatch(ctx context.Context, d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("power watch: %w", err)
	}

	path := logFilePath(homeDir, d.now())
	fmt.Fprintf(d.stdout, "blf power watch: %s\n", path)

	rendered := 0
	waiting := false
	for {
		data, err := d.readFile(path)
		if err != nil {
			if !waiting {
				fmt.Fprintln(d.stdout, "blf power watch: waiting for samples...")
				waiting = true
			}
		} else {
			waiting = false
			rendered = renderNewSamples(d.stdout, data, rendered)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(watchPollInterval):
		}
	}
}

// renderNewSamples decodes and renders the lines in data beyond the already
// rendered count, returning the updated count. Lines that fail to decode
// (e.g. a torn write caught mid-append) are skipped rather than aborting the
// watch.
func renderNewSamples(w io.Writer, data []byte, rendered int) int {
	lines := splitLogLines(data)
	if len(lines) <= rendered {
		return rendered
	}

	for _, line := range lines[rendered:] {
		sample, err := power.DecodeLine([]byte(line))
		if err != nil {
			continue
		}
		fmt.Fprintln(w, renderWatchSample(sample))
	}

	return len(lines)
}

func renderWatchSample(s power.Sample) string {
	ts := watchTimeStyle.Render(s.Ts.Format("15:04:05"))

	cpu := watchPowerStyle.Render(fmt.Sprintf("%.2fW", s.CPUPowerMW/1000))
	gpu := watchPowerStyle.Render(fmt.Sprintf("%.2fW", s.GPUPowerMW/1000))
	combined := watchPowerStyle.Render(fmt.Sprintf("%.2fW", s.CombinedPowerMW/1000))

	thermal := s.ThermalPressure
	if !strings.EqualFold(thermal, "nominal") {
		thermal = watchThermalStyle.Render(thermal)
	}

	state := watchOffStyle.Render("discharging")
	if s.BatteryCharging {
		state = watchOnStyle.Render("charging")
	} else if s.BatteryACConnected {
		state = watchOnStyle.Render("AC")
	}

	return fmt.Sprintf("%s  cpu=%s gpu=%s combined=%s thermal=%s battery=%d%% (%s)",
		ts, cpu, gpu, combined, thermal, s.BatteryPct, state)
}

func powerStateDir(homeDir string) string {
	return filepath.Join(config.XDGStateDir(homeDir), "power")
}

func pidFilePath(homeDir string) string {
	return filepath.Join(powerStateDir(homeDir), "daemon.pid")
}

func logFilePath(homeDir string, date time.Time) string {
	return filepath.Join(powerStateDir(homeDir), power.SamplesFileName(date))
}

// liveDaemon reads and validates the pidfile at path. A missing pidfile, a
// malformed one, or one whose pid is no longer alive is all treated the same
// way: self-healed (any on-disk pidfile is removed) and reported as "not
// running", never as an error.
func liveDaemon(d deps, path string) (power.PidInfo, bool, error) {
	exists, err := d.fileExists(path)
	if err != nil {
		return power.PidInfo{}, false, err
	}
	if !exists {
		return power.PidInfo{}, false, nil
	}

	data, err := d.readFile(path)
	if err != nil {
		return power.PidInfo{}, false, err
	}

	info, err := power.ParsePidFile(data)
	if err != nil || !power.IsProcessAlive(info.PID, d.signalProcess) {
		if rmErr := d.removeFile(path); rmErr != nil {
			return power.PidInfo{}, false, rmErr
		}
		return power.PidInfo{}, false, nil
	}

	return info, true, nil
}

func runPowerStart(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("power start: %w", err)
	}

	if v, _ := d.lookupEnv(daemonEnvVar); v == "1" {
		return runDaemonLoop(d, homeDir)
	}

	info, alive, err := liveDaemon(d, pidFilePath(homeDir))
	if err != nil {
		return fmt.Errorf("power start: %w", err)
	}
	if alive {
		return fmt.Errorf("blf power: already running (pid %d, since %s)", info.PID, info.StartTime.Format(time.RFC3339))
	}

	if err := spawnDaemon(d, homeDir); err != nil {
		return fmt.Errorf("power start: %w", err)
	}

	fmt.Fprintln(d.stdout, "blf power: started")
	return nil
}

// spawnDaemon re-execs the current binary as `blf power start` with
// daemonEnvVar set, detached into its own session (SysProcAttr.Setsid) with
// stdout/stderr redirected to a log file, and writes the pidfile. It does not
// wait for the child — the child keeps running after this process exits.
func spawnDaemon(d deps, homeDir string) error {
	dir := powerStateDir(homeDir)
	if err := d.mkdirAll(dir, 0o755); err != nil {
		return err
	}

	exe, err := d.executablePath()
	if err != nil {
		return err
	}

	debugLog, err := openAppendFile(filepath.Join(dir, "daemon.log"))
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "power", "start")
	cmd.Env = append(os.Environ(), daemonEnvVar+"=1")
	cmd.Stdin = nil
	cmd.Stdout = debugLog
	cmd.Stderr = debugLog
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	info := power.PidInfo{PID: cmd.Process.Pid, StartTime: d.now()}
	return d.writeFile(pidFilePath(homeDir), power.FormatPidFile(info), 0o644)
}

// runDaemonLoop is the body of the re-exec'd daemon process: prunes old
// day-files, then samples powermetrics+ioreg every sampleInterval until
// SIGTERM/SIGINT, removing its own pidfile as the last shutdown step.
func runDaemonLoop(d deps, homeDir string) error {
	dir := powerStateDir(homeDir)
	if err := d.mkdirAll(dir, 0o755); err != nil {
		return err
	}

	pruneOldSamples(d, dir)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	currentDay := d.now()
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return d.removeFile(pidFilePath(homeDir))
		case tick := <-ticker.C:
			if !sameDay(tick, currentDay) {
				currentDay = tick
				pruneOldSamples(d, dir)
			}
			runSampleTick(d, homeDir, tick)
		}
	}
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func pruneOldSamples(d deps, dir string) {
	entries, err := d.readDir(dir)
	if err != nil {
		return
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	for _, name := range power.FilesToPrune(names, d.now(), power.RetentionDays) {
		_ = d.removeFile(filepath.Join(dir, name))
	}
}

func runSampleTick(d deps, homeDir string, now time.Time) {
	pmOut, err := d.runCommand("sudo", "-n", "powermetrics", "-n", "1", "-i", "1000",
		"--samplers", "cpu_power,gpu_power,tasks,thermal", "--show-process-energy", "-f", "plist")
	if err != nil {
		return
	}
	pm, err := power.ParsePowermetrics(pmOut)
	if err != nil {
		return
	}

	ioregOut, err := d.runCommand("ioreg", "-rc", "AppleSmartBattery", "-a")
	if err != nil {
		return
	}
	batt, err := power.ParseIoreg(ioregOut)
	if err != nil {
		return
	}

	sample := power.BuildSample(now, pm, batt)
	line, err := power.EncodeLine(sample)
	if err != nil {
		return
	}

	appendToLogFile(logFilePath(homeDir, now), line)
}

// appendToLogFile appends raw bytes to path, creating it if needed. Uses a
// direct os.OpenFile (not d.writeFile, which overwrites) since JSONL lines
// must accumulate across ticks.
func appendToLogFile(path string, line []byte) {
	f, err := openAppendFile(path)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(line)
}

// openAppendFile opens path for appending, creating it if needed.
func openAppendFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// splitLogLines splits raw day-file bytes into JSONL lines, dropping the
// trailing newline. Returns nil for empty data (rather than a single ""
// entry), so callers can use len() to tell "no lines yet" from "one line".
func splitLogLines(data []byte) []string {
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func runPowerStop(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("power stop: %w", err)
	}

	pidPath := pidFilePath(homeDir)
	info, alive, err := liveDaemon(d, pidPath)
	if err != nil {
		return fmt.Errorf("power stop: %w", err)
	}
	if !alive {
		fmt.Fprintln(d.stdout, "blf power: not running")
		return nil
	}

	if err := d.signalProcess(info.PID, syscall.SIGTERM); err != nil {
		return fmt.Errorf("power stop: %w", err)
	}

	deadline := time.Now().Add(stopTimeout)
	for time.Now().Before(deadline) {
		if !power.IsProcessAlive(info.PID, d.signalProcess) {
			fmt.Fprintln(d.stdout, "blf power: stopped")
			return nil
		}
		time.Sleep(stopPollInterval)
	}

	if err := d.signalProcess(info.PID, syscall.SIGKILL); err != nil {
		return fmt.Errorf("power stop: %w", err)
	}
	_ = d.removeFile(pidPath)
	fmt.Fprintln(d.stdout, "blf power: stopped (force-killed)")
	return nil
}

func runPowerStatus(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("power status: %w", err)
	}

	info, alive, err := liveDaemon(d, pidFilePath(homeDir))
	if err != nil {
		return fmt.Errorf("power status: %w", err)
	}
	if !alive {
		fmt.Fprintln(d.stdout, "blf power: not running")
		return nil
	}

	fmt.Fprintf(d.stdout, "blf power: running (pid %d, since %s)\n", info.PID, info.StartTime.Format(time.RFC3339))

	logPath := logFilePath(homeDir, d.now())
	fmt.Fprintf(d.stdout, "log: %s\n", logPath)
	fmt.Fprintf(d.stdout, "last sample: %s\n", lastSampleTime(d, logPath))
	return nil
}

func runPowerReport(d deps, since string) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("power report: %w", err)
	}

	duration, err := power.ParseSinceWindow(since)
	if err != nil {
		return fmt.Errorf("power report: %w", err)
	}

	now := d.now()
	samples := readSamplesInWindow(d, homeDir, now, duration)

	if len(samples) == 0 {
		fmt.Fprintf(d.stdout, "no samples found in the last %s (is the daemon running? try 'blf power status')\n", since)
		return nil
	}

	report := power.BuildReport(samples, duration, now)
	fmt.Fprint(d.stdout, renderPowerReport(since, report))
	return nil
}

// readSamplesInWindow reads every day-file overlapping [now-since, now],
// decodes each line, and returns the samples whose Ts falls within that
// window, sorted ascending by Ts. Missing or unreadable day-files are
// skipped (the daemon may not have been running for the full window).
func readSamplesInWindow(d deps, homeDir string, now time.Time, since time.Duration) []power.Sample {
	windowStart := now.Add(-since)

	var samples []power.Sample
	for _, name := range power.DayFilesForWindow(now, since) {
		data, err := d.readFile(filepath.Join(powerStateDir(homeDir), name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			sample, err := power.DecodeLine([]byte(line))
			if err != nil {
				continue
			}
			if sample.Ts.Before(windowStart) || sample.Ts.After(now) {
				continue
			}
			samples = append(samples, sample)
		}
	}

	sort.Slice(samples, func(i, j int) bool {
		return samples[i].Ts.Before(samples[j].Ts)
	})
	return samples
}

func renderPowerReport(since string, r power.Report) string {
	var b strings.Builder

	header := fmt.Sprintf("Power report — last %s (%s → %s)", since,
		r.WindowStart.Format("2006-01-02 15:04"), r.WindowEnd.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "%s\n\n", powerReportBoldStyle.Render(header))

	fmt.Fprintln(&b, powerReportBoldStyle.Render("Battery"))
	fmt.Fprintf(&b, "  Net change:      %+d%%  (%d%% → %d%%)\n", r.Battery.NetChangePct, r.Battery.StartPct, r.Battery.EndPct)
	if r.Battery.HasDischargeRate {
		fmt.Fprintf(&b, "  Discharge rate:  %.1f%%/hour  (while unplugged)\n", r.Battery.DischargeRatePctPerHour)
	} else {
		fmt.Fprintln(&b, "  Discharge rate:  n/a (no discharging samples in window)")
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, powerReportBoldStyle.Render("Top energy consumers"))
	for i, p := range r.TopProcesses {
		fmt.Fprintf(&b, "  %d. %-20s %8.1f  energy impact   (%d/%d ticks)\n",
			i+1, p.Name, p.MeanEnergyImpact, p.TicksPresent, r.TicksInWindow)
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, powerReportBoldStyle.Render("Thermal"))
	parts := make([]string, len(r.Thermal))
	for i, t := range r.Thermal {
		parts[i] = fmt.Sprintf("%s: %.0f%% of samples", t.Value, t.Percent)
	}
	fmt.Fprintln(&b, "  "+strings.Join(parts, " · "))

	return b.String()
}

func lastSampleTime(d deps, logPath string) string {
	data, err := d.readFile(logPath)
	if err != nil {
		return "none yet"
	}

	lines := splitLogLines(data)
	if len(lines) == 0 {
		return "none yet"
	}
	last := lines[len(lines)-1]

	sample, err := power.DecodeLine([]byte(last))
	if err != nil {
		return "none yet"
	}
	return sample.Ts.Format(time.RFC3339)
}
