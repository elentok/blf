package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLauncherCmd(d deps) *cobra.Command {
	launcher := &cobra.Command{
		Use:   "launcher",
		Short: "Terminal launcher (runs inside Kitty quick terminal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("launcher TUI not yet implemented")
		},
	}

	launcher.AddCommand(newLauncherReindexCmd(d))

	return launcher
}

func newLauncherReindexCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the application index",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("launcher reindex not yet implemented")
		},
	}
}
