package cmd

import (
	"fmt"

	"github.com/elentok/blf/internal/beads"
	"github.com/spf13/cobra"
)

func newBeadsCmd(d deps) *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "beads",
		Short: "Browse Beads issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			adapter := beads.New(dir)
			if err := adapter.Check(); err != nil {
				return err
			}

			id, err := beads.Run(beads.ModelConfig{
				Lister:   adapter,
				Scope:    beads.ScopeActionable,
				CopyText: d.copyText,
			})
			if err != nil {
				return err
			}
			if id != "" {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "C", "", "Project directory (passed to bd -C)")
	return cmd
}
