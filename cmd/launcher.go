package cmd

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/units"
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

			// Currency cache
			cacheDir := launcher.XDGCacheDir(homeDir)
			currencyCache := currency.NewCache(
				filepath.Join(cacheDir, "currency.json"),
				nil, // use default HTTP client
				nil, // use time.Now
			)

			// Units registry with optional user-defined groups merged in
			registry := units.NewBuiltinRegistry()
			for _, ug := range cfg.Launcher.UnitGroups {
				g := &units.Group{Name: ug.Name}
				for _, u := range ug.Units {
					g.Units = append(g.Units, &units.Unit{
						Name:    u.Name,
						Symbols: u.Symbols,
						Factor:  u.Factor,
						Offset:  u.Offset,
					})
				}
				registry.AddGroup(g)
			}

			m := launcher.NewModel(launcher.ModelConfig{
				Providers: []launcher.Provider{
					launcher.CalcProvider{},
					launcher.NewUnitsProvider(registry, currencyCache),
				},
				ConfigErr:     cfgErr,
				CopyText:      d.copyText,
				CurrencyCache: currencyCache,
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
