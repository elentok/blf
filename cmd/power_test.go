package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/elentok/blf/internal/power"
)

// powerTestDeps returns a deps set up with real filesystem operations rooted
// at a fresh temp "home" dir (XDG_STATE_HOME cleared so config.XDGStateDir
// falls back to <home>/.local/state/blf), a fixed now, and a stubbed
// executablePath/signalProcess so start/stop/status can be exercised without
// touching the real daemon.
func powerTestDeps(t *testing.T, now time.Time) (deps, *strings.Builder, *strings.Builder) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", "")
	home := t.TempDir()
	out := &strings.Builder{}
	errOut := &strings.Builder{}

	return deps{
		stdout:         out,
		stderr:         errOut,
		lookupEnv:      os.LookupEnv,
		fileExists:     fileExists,
		readFile:       os.ReadFile,
		writeFile:      os.WriteFile,
		removeFile:     os.Remove,
		readDir:        os.ReadDir,
		mkdirAll:       os.MkdirAll,
		userHomeDir:    func() (string, error) { return home, nil },
		executablePath: func() (string, error) { return "/bin/echo", nil },
		now:            func() time.Time { return now },
		signalProcess:  func(pid int, sig syscall.Signal) error { return nil },
	}, out, errOut
}

func writePidFile(t *testing.T, homeDir string, info power.PidInfo) {
	t.Helper()
	path := pidFilePath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir pidfile dir: %v", err)
	}
	if err := os.WriteFile(path, power.FormatPidFile(info), 0o644); err != nil {
		t.Fatalf("write pidfile: %v", err)
	}
}

func TestPowerStatusNotRunningWhenNoPidfile(t *testing.T) {
	d, out, _ := powerTestDeps(t, time.Now())

	if err := execute([]string{"power", "status"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: not running\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestPowerStopNotRunningWhenNoPidfile(t *testing.T) {
	d, out, _ := powerTestDeps(t, time.Now())

	if err := execute([]string{"power", "stop"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: not running\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestPowerStopSelfHealsStalePidfile(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)
	d.signalProcess = func(pid int, sig syscall.Signal) error { return syscall.ESRCH }

	home, _ := d.userHomeDir()
	start := now.Add(-time.Hour)
	writePidFile(t, home, power.PidInfo{PID: 99999, StartTime: start})

	if err := execute([]string{"power", "stop"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: not running\n" {
		t.Fatalf("stdout = %q", got)
	}
	if exists, _ := fileExists(pidFilePath(home)); exists {
		t.Fatal("stale pidfile was not self-healed (removed)")
	}
}

func TestPowerStatusSelfHealsStalePidfile(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)
	d.signalProcess = func(pid int, sig syscall.Signal) error { return syscall.ESRCH }

	home, _ := d.userHomeDir()
	writePidFile(t, home, power.PidInfo{PID: 99999, StartTime: now.Add(-time.Hour)})

	if err := execute([]string{"power", "status"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: not running\n" {
		t.Fatalf("stdout = %q", got)
	}
	if exists, _ := fileExists(pidFilePath(home)); exists {
		t.Fatal("stale pidfile was not self-healed (removed)")
	}
}

func TestPowerStartAlreadyRunning(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, _, _ := powerTestDeps(t, now)
	d.signalProcess = func(pid int, sig syscall.Signal) error { return nil } // alive

	home, _ := d.userHomeDir()
	start := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	writePidFile(t, home, power.PidInfo{PID: 4242, StartTime: start})

	err := execute([]string{"power", "start"}, d)
	wantErr := "blf power: already running (pid 4242, since 2026-08-02T09:00:00Z)"
	if err == nil || err.Error() != wantErr {
		t.Fatalf("err = %v, want %q", err, wantErr)
	}
}

func TestPowerStartSpawnsDaemonAndWritesPidfile(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	if err := execute([]string{"power", "start"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: started\n" {
		t.Fatalf("stdout = %q", got)
	}

	home, _ := d.userHomeDir()
	data, err := os.ReadFile(pidFilePath(home))
	if err != nil {
		t.Fatalf("read pidfile: %v", err)
	}
	info, err := power.ParsePidFile(data)
	if err != nil {
		t.Fatalf("parse pidfile: %v", err)
	}
	if info.PID <= 0 {
		t.Errorf("pidfile pid = %d, want > 0", info.PID)
	}
	if !info.StartTime.Equal(now) {
		t.Errorf("pidfile start time = %v, want %v", info.StartTime, now)
	}
}

func TestPowerStopGracefulExit(t *testing.T) {
	origInterval, origTimeout := stopPollInterval, stopTimeout
	stopPollInterval, stopTimeout = 5*time.Millisecond, 200*time.Millisecond
	defer func() { stopPollInterval, stopTimeout = origInterval, origTimeout }()

	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	livenessChecks := 0
	d.signalProcess = func(pid int, sig syscall.Signal) error {
		if sig != 0 {
			return nil // SIGTERM accepted
		}
		livenessChecks++
		if livenessChecks > 1 {
			return syscall.ESRCH // dead after the initial "is it running" check
		}
		return nil
	}

	home, _ := d.userHomeDir()
	writePidFile(t, home, power.PidInfo{PID: 4242, StartTime: now.Add(-time.Hour)})

	if err := execute([]string{"power", "stop"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: stopped\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestPowerStopForceKillsAfterTimeout(t *testing.T) {
	origInterval, origTimeout := stopPollInterval, stopTimeout
	stopPollInterval, stopTimeout = 5*time.Millisecond, 30*time.Millisecond
	defer func() { stopPollInterval, stopTimeout = origInterval, origTimeout }()

	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	var sentSignals []syscall.Signal
	d.signalProcess = func(pid int, sig syscall.Signal) error {
		sentSignals = append(sentSignals, sig)
		return nil // always "alive"/accepted, never dies on its own
	}

	home, _ := d.userHomeDir()
	writePidFile(t, home, power.PidInfo{PID: 4242, StartTime: now.Add(-time.Hour)})

	if err := execute([]string{"power", "stop"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if got := out.String(); got != "blf power: stopped (force-killed)\n" {
		t.Fatalf("stdout = %q", got)
	}
	if exists, _ := fileExists(pidFilePath(home)); exists {
		t.Fatal("pidfile should have been removed as the force-kill fallback")
	}

	foundKill := false
	for _, sig := range sentSignals {
		if sig == syscall.SIGKILL {
			foundKill = true
		}
	}
	if !foundKill {
		t.Errorf("SIGKILL was never sent; signals = %v", sentSignals)
	}
}

func TestPowerStatusRunningWithLastSample(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	home, _ := d.userHomeDir()
	start := now.Add(-2 * time.Hour)
	writePidFile(t, home, power.PidInfo{PID: 4242, StartTime: start})

	logPath := logFilePath(home, now)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	sample := power.BuildSample(now.Add(-30*time.Second), power.PowermetricsResult{}, power.BatteryInfo{})
	line, err := power.EncodeLine(sample)
	if err != nil {
		t.Fatalf("EncodeLine: %v", err)
	}
	if err := os.WriteFile(logPath, line, 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if err := execute([]string{"power", "status"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	want := "blf power: running (pid 4242, since 2026-08-02T08:00:00Z)\n" +
		"log: " + logPath + "\n" +
		"last sample: 2026-08-02T09:59:30Z\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func writeSampleLine(t *testing.T, homeDir string, sample power.Sample) {
	t.Helper()
	logPath := logFilePath(homeDir, sample.Ts)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	line, err := power.EncodeLine(sample)
	if err != nil {
		t.Fatalf("EncodeLine: %v", err)
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open log file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		t.Fatalf("write log line: %v", err)
	}
}

func TestPowerReportNoSamplesInWindow(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	if err := execute([]string{"power", "report"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	want := "no samples found in the last 24h (is the daemon running? try 'blf power status')\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestPowerReportInvalidSince(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, _, _ := powerTestDeps(t, now)

	err := execute([]string{"power", "report", "--since", "1w"}, d)
	if err == nil {
		t.Fatal("execute returned no error, want one for invalid --since")
	}
}

func TestPowerReportSummarizesWindow(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)
	home, _ := d.userHomeDir()

	writeSampleLine(t, home, power.BuildSample(now.Add(-2*time.Hour), power.PowermetricsResult{
		ThermalPressure: "Nominal",
		CPUPowerMW:      1000,
		GPUPowerMW:      2000,
		Processes:       []power.ProcessSample{{PID: 1, Name: "WindowServer", EnergyImpactPerS: 800}},
	}, power.BatteryInfo{CurrentCapacity: 67, IsCharging: false}))
	writeSampleLine(t, home, power.BuildSample(now.Add(-time.Hour), power.PowermetricsResult{
		ThermalPressure: "Nominal",
		CPUPowerMW:      2000,
		GPUPowerMW:      4000,
		Processes:       []power.ProcessSample{{PID: 1, Name: "WindowServer", EnergyImpactPerS: 900}},
	}, power.BatteryInfo{CurrentCapacity: 33, IsCharging: false}))

	// Outside the default 24h window -- must not affect the report.
	writeSampleLine(t, home, power.BuildSample(now.Add(-48*time.Hour), power.PowermetricsResult{
		ThermalPressure: "Nominal",
		Processes:       []power.ProcessSample{{PID: 1, Name: "Excluded", EnergyImpactPerS: 99999}},
	}, power.BatteryInfo{CurrentCapacity: 100, IsCharging: false}))

	if err := execute([]string{"power", "report"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Power report",
		"CPU:  1.50W",
		"GPU:  3.00W",
		"Net change:      -34%  (67% → 33%)",
		"1. WindowServer",
		"850.0  energy impact   (2/2 ticks)",
		"Discharge rate:  34.0%/hour",
		"Nominal: 100% of samples",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Excluded") {
		t.Errorf("stdout contains sample outside the window; got:\n%s", got)
	}
}

func TestPowerStatusRunningWithNoSampleYet(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	home, _ := d.userHomeDir()
	writePidFile(t, home, power.PidInfo{PID: 4242, StartTime: now.Add(-time.Minute)})

	if err := execute([]string{"power", "status"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	logPath := logFilePath(home, now)
	want := "blf power: running (pid 4242, since 2026-08-02T09:59:00Z)\n" +
		"log: " + logPath + "\n" +
		"last sample: none yet\n"
	if got := out.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func withWatchPollInterval(t *testing.T, interval time.Duration) {
	t.Helper()
	orig := watchPollInterval
	watchPollInterval = interval
	t.Cleanup(func() { watchPollInterval = orig })
}

func TestPowerWatchRendersExistingSamples(t *testing.T) {
	withWatchPollInterval(t, 5*time.Millisecond)

	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)
	home, _ := d.userHomeDir()

	logPath := logFilePath(home, now)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	sample := power.BuildSample(now, power.PowermetricsResult{
		CPUPowerMW: 1000, GPUPowerMW: 500, CombinedPowerMW: 1500, ThermalPressure: "Nominal",
	}, power.BatteryInfo{CurrentCapacity: 80, IsCharging: true})
	line, err := power.EncodeLine(sample)
	if err != nil {
		t.Fatalf("EncodeLine: %v", err)
	}
	if err := os.WriteFile(logPath, line, 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := runPowerWatch(ctx, d); err != nil {
		t.Fatalf("runPowerWatch returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{logPath, "10:00:00", "cpu=", "1.00W", "gpu=", "0.50W", "combined=", "1.50W", "battery=80%", "charging"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q; got %q", want, got)
		}
	}
}

func TestPowerWatchWaitsForMissingLogFile(t *testing.T) {
	withWatchPollInterval(t, 5*time.Millisecond)

	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := runPowerWatch(ctx, d); err != nil {
		t.Fatalf("runPowerWatch returned error: %v", err)
	}

	if got := out.String(); !strings.Contains(got, "waiting for samples") {
		t.Errorf("stdout = %q, want a waiting-for-samples message", got)
	}
}

func TestPowerWatchPicksUpAppendedSamples(t *testing.T) {
	withWatchPollInterval(t, 5*time.Millisecond)

	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	d, out, _ := powerTestDeps(t, now)
	home, _ := d.userHomeDir()
	logPath := logFilePath(home, now)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runPowerWatch(ctx, d) }()

	time.Sleep(15 * time.Millisecond)

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}
	sample := power.BuildSample(now, power.PowermetricsResult{
		CPUPowerMW: 2000, GPUPowerMW: 100, CombinedPowerMW: 2100, ThermalPressure: "Heavy",
	}, power.BatteryInfo{CurrentCapacity: 55})
	line, err := power.EncodeLine(sample)
	if err != nil {
		t.Fatalf("EncodeLine: %v", err)
	}
	if err := os.WriteFile(logPath, line, 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	time.Sleep(25 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runPowerWatch returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"waiting for samples", "cpu=", "2.00W", "battery=55%", "discharging", "Heavy"} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q; got %q", want, got)
		}
	}
}
