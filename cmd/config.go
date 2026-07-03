package cmd

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/editfile"
	"github.com/spf13/cobra"
)

func newConfigCmd(d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the blf config file",
	}

	cmd.AddCommand(newConfigEditCmd(d))
	return cmd
}

func newConfigEditCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the blf config file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEdit(d)
		},
	}
}

func runConfigEdit(d deps) error {
	homeDir, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("config edit: get home dir: %w", err)
	}

	path := config.XDGConfigPath(homeDir)

	exists, err := d.fileExists(path)
	if err != nil {
		return fmt.Errorf("config edit: check %q: %w", path, err)
	}
	if !exists {
		if err := seedDefaultConfig(path, d); err != nil {
			return fmt.Errorf("config edit: seed %q: %w", path, err)
		}
	}

	return editfile.Open(path, editfile.Deps{
		Stdin:     d.stdin,
		Stdout:    d.stdout,
		Stderr:    d.stderr,
		LookupEnv: d.lookupEnv,
	})
}

func seedDefaultConfig(path string, d deps) error {
	if err := d.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(config.DefaultConfig()); err != nil {
		return fmt.Errorf("encode defaults: %w", err)
	}

	return d.writeFile(path, buf.Bytes(), 0o644)
}
