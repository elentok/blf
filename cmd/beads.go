package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
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

			id, err := beads.Run(beads.ModelConfig{
				Lister:     adapter,
				Preview:    adapter,
				Mutator:    adapter,
				Scope:      beads.ScopeActionable,
				CopyText:   d.copyText,
				EditIssue:  func(id string) tea.Cmd { return beads.EditIssueCmd(dir, id) },
				GraphIssue: func(id string) tea.Cmd { return beads.GraphIssueCmd(dir, id) },
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
