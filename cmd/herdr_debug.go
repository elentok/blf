package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elentok/blf/internal/config"
	"github.com/spf13/cobra"
)

func newHerdrDebugCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "debug [on|off]",
		Short: "Show or set herdr sync debug logging",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return runHerdrDebugStatus(d)
			}
			switch args[0] {
			case "on":
				return runHerdrDebugOn(d)
			case "off":
				return runHerdrDebugOff(d)
			default:
				return fmt.Errorf("herdr debug: unknown argument %q, want \"on\" or \"off\"", args[0])
			}
		},
	}
}

func herdrDebugMarkerPath(homeDir string) string {
	return filepath.Join(config.XDGStateDir(homeDir), "herdr-debug.on")
}

func herdrDebugLogPath(homeDir string) string {
	return filepath.Join(config.XDGStateDir(homeDir), "herdr-debug.log")
}

func runHerdrDebugOn(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr debug on: %w", err)
	}

	if err := d.mkdirAll(config.XDGStateDir(homeDir), 0o755); err != nil {
		return fmt.Errorf("herdr debug on: %w", err)
	}

	if err := d.writeFile(herdrDebugMarkerPath(homeDir), []byte{}, 0o644); err != nil {
		return fmt.Errorf("herdr debug on: %w", err)
	}

	// Truncate (not append) even when already on, so a fresh `debug on`
	// always starts a clean log.
	if err := d.writeFile(herdrDebugLogPath(homeDir), []byte{}, 0o644); err != nil {
		return fmt.Errorf("herdr debug on: %w", err)
	}

	fmt.Fprintln(d.stdout, "blf herdr debug: on")
	return nil
}

func runHerdrDebugOff(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr debug off: %w", err)
	}

	if err := d.removeFile(herdrDebugMarkerPath(homeDir)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("herdr debug off: %w", err)
	}

	fmt.Fprintln(d.stdout, "blf herdr debug: off")
	return nil
}

func runHerdrDebugStatus(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr debug: %w", err)
	}

	exists, err := d.fileExists(herdrDebugMarkerPath(homeDir))
	if err != nil {
		return fmt.Errorf("herdr debug: %w", err)
	}

	state := "off"
	if exists {
		state = "on"
	}

	fmt.Fprintf(d.stdout, "blf herdr debug: %s\n", state)
	fmt.Fprintf(d.stdout, "log: %s\n", herdrDebugLogPath(homeDir))
	return nil
}

// logHerdrSync appends one tab-separated debug line when herdr debug logging
// is on, and is a no-op otherwise. mode is "event", "full-session" or "fish".
// note carries a skip reason or error message; pass "" for a plain rename.
func logHerdrSync(d deps, mode, tab, oldLabel, newLabel, note string) {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return
	}

	exists, err := d.fileExists(herdrDebugMarkerPath(homeDir))
	if err != nil || !exists {
		return
	}

	line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\n",
		d.now().Format(time.RFC3339), mode, tab, oldLabel, newLabel, note)
	_ = d.appendFile(herdrDebugLogPath(homeDir), []byte(line))
}
