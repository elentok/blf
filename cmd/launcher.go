package cmd

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/apps"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/history"
	"github.com/elentok/blf/internal/launcher/scripts"
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

			// Apps provider: load cached index immediately, reindex in background via Init().
			appsCachePath := filepath.Join(cacheDir, "apps.json")
			appsProvider := launcher.NewAppsProvider(cfg.Launcher.AppWeight)
			if idx, err := apps.Load(appsCachePath); err == nil && len(idx.Apps) > 0 {
				appsProvider.SetIndex(idx)
			}

			// History: load from state dir.
			stateDir := launcher.XDGStateDir(homeDir)
			historyPath := filepath.Join(stateDir, "launcher-history")
			launcherHistory := history.Load(historyPath)

			// Scripts provider: merge built-ins with user config, filter for platform.
			userScripts := make([]scripts.Script, 0, len(cfg.Launcher.Scripts))
			for _, sc := range cfg.Launcher.Scripts {
				userScripts = append(userScripts, scripts.Script{
					Name:     sc.Name,
					Icon:     sc.Icon,
					Type:     scripts.ScriptType(sc.Type),
					Platform: sc.Platform,
					Body:     sc.Body,
					Output:   scripts.OutputMode(sc.Output),
				})
			}
			allScripts := scripts.Merge(scripts.Builtins, userScripts)
			platformScripts := scripts.FilterForPlatform(allScripts)
			scriptsProvider := launcher.NewScriptsProvider(platformScripts, cfg.Launcher.ScriptWeight)

			m := launcher.NewModel(launcher.ModelConfig{
				Providers: []launcher.Provider{
					launcher.CalcProvider{},
					launcher.NewUnitsProvider(registry, currencyCache, cfg.Launcher.Currencies),
					appsProvider,
					scriptsProvider,
				},
				ConfigErr:     cfgErr,
				CopyText:      d.copyText,
				CurrencyCache: currencyCache,
				AppsProvider:    appsProvider,
				AppsCachePath:   appsCachePath,
				HomeDir:         homeDir,
				ScriptsProvider: scriptsProvider,
				History:         launcherHistory,
				HistoryPath:     historyPath,
				LaunchApp: func(appPath string) error {
					launchArgs := apps.LaunchArgs(apps.App{Path: appPath})
					_, err := d.runCommand(launchArgs[0], launchArgs[1:]...)
					return err
				},
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

func newLauncherReindexCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the application index",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := d.userHomeDir()
			if err != nil {
				return fmt.Errorf("reindex: get home dir: %w", err)
			}
			idx, err := apps.Reindex(homeDir)
			if err != nil {
				return fmt.Errorf("reindex: %w", err)
			}
			cacheDir := launcher.XDGCacheDir(homeDir)
			cachePath := filepath.Join(cacheDir, "apps.json")
			if err := apps.Save(cachePath, idx); err != nil {
				return fmt.Errorf("reindex: save: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "indexed %d apps → %s\n", len(idx.Apps), cachePath)
			return nil
		},
	}
}
