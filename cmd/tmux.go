package cmd

import (
	"fmt"

	"github.com/elentok/blf/internal/tmuxlinks"
	"github.com/spf13/cobra"
)

func newTmuxCmd(d deps) *cobra.Command {
	tmux := &cobra.Command{
		Use:   "tmux",
		Short: "Tmux utilities",
	}

	tmux.AddCommand(
		newTmuxLinksCmd(d),
		newTmuxTargetsCmd(d),
	)

	return tmux
}

func newTmuxLinksCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "links <open|copy>",
		Short: "Open or copy tmux links",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := args[0]
			if mode != tmuxlinks.ModeOpen && mode != tmuxlinks.ModeCopy {
				return fmt.Errorf("invalid tmux links mode %q (expected open or copy)", mode)
			}
			return d.runTmuxLinks(mode)
		},
	}
}

func newTmuxTargetsCmd(d deps) *cobra.Command {
	var popup bool
	var target string

	cmd := &cobra.Command{
		Use:   "targets",
		Short: "Show tmux targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return d.runTargets(popup, target)
		},
	}
	cmd.Flags().BoolVar(&popup, "popup", false, "Show in popup")
	cmd.Flags().StringVar(&target, "target", "", "Target pane")
	return cmd
}
