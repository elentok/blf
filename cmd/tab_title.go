package cmd

import (
	"fmt"

	"github.com/elentok/blf/internal/tabtitle"
	"github.com/spf13/cobra"
)

func runTabTitle(args []string, d deps) error {
	if v, _ := d.lookupEnv("VIMRUNTIME"); v != "" {
		return nil
	}

	var cmd string
	if len(args) > 0 {
		cmd = args[0]
	}

	pwd, err := d.getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	home, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	fmt.Fprintln(d.stdout, tabtitle.Title(cmd, pwd, home))
	return nil
}

func newTabTitleCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "tab-title [cmd]",
		Short: "Print the tab title for the current directory and command",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTabTitle(args, d)
		},
	}
}
