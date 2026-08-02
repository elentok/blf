package cmd

import (
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
