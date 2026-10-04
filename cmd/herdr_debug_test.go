package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// herdrDebugTestDeps returns a deps set up with real filesystem operations
// rooted at a fresh temp "home" dir (XDG_STATE_HOME cleared so
// config.XDGStateDir falls back to <home>/.local/state/blf).
func herdrDebugTestDeps(t *testing.T) (deps, *strings.Builder) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", "")
	home := t.TempDir()
	out := &strings.Builder{}

	return deps{
		stdout:      out,
		stderr:      &strings.Builder{},
		fileExists:  fileExists,
		readFile:    os.ReadFile,
		writeFile:   os.WriteFile,
		removeFile:  os.Remove,
		mkdirAll:    os.MkdirAll,
		appendFile:  appendFile,
		userHomeDir: func() (string, error) { return home, nil },
		now:         func() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) },
	}, out
}

func TestHerdrDebugOnCreatesMarkerAndTruncatesLog(t *testing.T) {
	d, out := herdrDebugTestDeps(t)
	home, err := d.userHomeDir()
	if err != nil {
		t.Fatalf("userHomeDir: %v", err)
	}

	// Pre-seed a non-empty log to verify `on` truncates it.
	if err := d.mkdirAll(filepath.Dir(herdrDebugLogPath(home)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := d.writeFile(herdrDebugLogPath(home), []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	if err := execute([]string{"herdr", "debug", "on"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	exists, err := d.fileExists(herdrDebugMarkerPath(home))
	if err != nil || !exists {
		t.Fatalf("marker exists = %v, err = %v, want marker present", exists, err)
	}

	log, err := d.readFile(herdrDebugLogPath(home))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(log) != 0 {
		t.Errorf("log = %q, want empty (truncated)", log)
	}

	if !strings.Contains(out.String(), "on") {
		t.Errorf("output = %q, want it to mention \"on\"", out.String())
	}
}

func TestHerdrDebugOnTruncatesWhenAlreadyOn(t *testing.T) {
	d, _ := herdrDebugTestDeps(t)
	home, _ := d.userHomeDir()

	if err := execute([]string{"herdr", "debug", "on"}, d); err != nil {
		t.Fatalf("first debug on: %v", err)
	}
	if err := d.appendFile(herdrDebugLogPath(home), []byte("line1\n")); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := execute([]string{"herdr", "debug", "on"}, d); err != nil {
		t.Fatalf("second debug on: %v", err)
	}

	log, err := d.readFile(herdrDebugLogPath(home))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(log) != 0 {
		t.Errorf("log = %q, want empty after re-running debug on", log)
	}
}

func TestHerdrDebugOffRemovesMarkerKeepsLog(t *testing.T) {
	d, out := herdrDebugTestDeps(t)
	home, _ := d.userHomeDir()

	if err := execute([]string{"herdr", "debug", "on"}, d); err != nil {
		t.Fatalf("debug on: %v", err)
	}
	if err := d.appendFile(herdrDebugLogPath(home), []byte("line1\n")); err != nil {
		t.Fatalf("append: %v", err)
	}

	if err := execute([]string{"herdr", "debug", "off"}, d); err != nil {
		t.Fatalf("debug off: %v", err)
	}

	exists, err := d.fileExists(herdrDebugMarkerPath(home))
	if err != nil {
		t.Fatalf("fileExists: %v", err)
	}
	if exists {
		t.Error("marker still exists after debug off")
	}

	log, err := d.readFile(herdrDebugLogPath(home))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(log) != "line1\n" {
		t.Errorf("log = %q, want it kept after debug off", log)
	}

	if !strings.Contains(out.String(), "off") {
		t.Errorf("output = %q, want it to mention \"off\"", out.String())
	}
}

func TestHerdrDebugOffWhenAlreadyOffIsNotAnError(t *testing.T) {
	d, _ := herdrDebugTestDeps(t)

	if err := execute([]string{"herdr", "debug", "off"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
}

func TestHerdrDebugNoArgPrintsStateAndPath(t *testing.T) {
	d, out := herdrDebugTestDeps(t)
	home, _ := d.userHomeDir()

	if err := execute([]string{"herdr", "debug"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "off") {
		t.Errorf("output = %q, want state \"off\"", out.String())
	}
	if !strings.Contains(out.String(), herdrDebugLogPath(home)) {
		t.Errorf("output = %q, want the log path %q", out.String(), herdrDebugLogPath(home))
	}

	out.Reset()
	if err := execute([]string{"herdr", "debug", "on"}, d); err != nil {
		t.Fatalf("debug on: %v", err)
	}
	out.Reset()

	if err := execute([]string{"herdr", "debug"}, d); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if !strings.Contains(out.String(), "on") {
		t.Errorf("output = %q, want state \"on\"", out.String())
	}
}

func TestHerdrDebugUnknownArgumentIsUsageError(t *testing.T) {
	d, _ := herdrDebugTestDeps(t)

	if err := execute([]string{"herdr", "debug", "bogus"}, d); err == nil {
		t.Fatal("execute returned nil error, want a usage error")
	}
}
