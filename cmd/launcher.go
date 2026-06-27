package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher"
	"github.com/spf13/cobra"
)

func newLauncherCmd(d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launcher",
		Short: "Terminal launcher (runs inside Kitty quick terminal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := d.userHomeDir()
			if err != nil {
				return fmt.Errorf("launcher: get home dir: %w", err)
			}

			cfg, cfgErr := launcher.LoadConfig(d.readFile, homeDir)
			_ = cfg // cfg will feed providers in later tasks (scripts weight etc.)

			m := launcher.NewModel(launcher.ModelConfig{
				Providers: []launcher.Provider{
					launcher.CalcProvider{},
				},
				ConfigErr: cfgErr,
				CopyText:  d.copyText,
				HideTerminal: func() error {
					_, err := d.runCommand("kitten", "quick-access-terminal", "--instance-group", "quick")
					return err
				},
				UseNerdFont: true,
			})

			p := tea.NewProgram(m)
			_, err = p.Run()
			return err
		},
	}

	cmd.AddCommand(newLauncherReindexCmd(d))
	return cmd
}

func newLauncherReindexCmd(_ deps) *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the application index",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("launcher reindex not yet implemented")
		},
	}
}
