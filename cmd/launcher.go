package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/launcher"
	"github.com/elentok/blf/internal/launcher/apps"
	"github.com/elentok/blf/internal/launcher/currency"
	"github.com/elentok/blf/internal/launcher/directories"
	"github.com/elentok/blf/internal/launcher/history"
	"github.com/elentok/blf/internal/launcher/learnedrank"
	"github.com/elentok/blf/internal/launcher/scripts"
	"github.com/elentok/blf/internal/launcher/units"
	"github.com/elentok/blf/internal/platform"
	"github.com/spf13/cobra"
)

const (
	ghosttyQuickTerminalEnv          = "GHOSTTY_QUICK_TERMINAL"
	ghosttyToggleQuickTerminalScript = `tell application "Ghostty" to perform action "toggle_quick_terminal" on focused terminal of selected tab of front window`
)

func newLauncherCmd(d deps) *cobra.Command {
	var noBorder bool

	cmd := &cobra.Command{
		Use:   "launcher",
		Short: "Terminal launcher (runs inside a Kitty or Ghostty quick terminal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := d.userHomeDir()
			if err != nil {
				return fmt.Errorf("launcher: get home dir: %w", err)
			}

			cfg, cfgErr := config.LoadConfig(d.readFile, homeDir)

			// Currency cache
			cacheDir := config.XDGCacheDir(homeDir)
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
			stateDir := config.XDGStateDir(homeDir)
			historyPath := filepath.Join(stateDir, "launcher-history")
			launcherHistory := history.Load(historyPath)

			// Learned rank: load from state dir.
			learnedRankPath := filepath.Join(stateDir, "launcher-learned-ranks")
			launcherLearnedRank := learnedrank.Load(learnedRankPath)

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

			settingsProvider := launcher.NewSettingsProvider(cfg.Launcher.AppWeight)

			// Directory provider: merge built-ins with user config, expand "~".
			userDirs := make([]directories.Directory, 0, len(cfg.Launcher.Directories))
			for _, dc := range cfg.Launcher.Directories {
				userDirs = append(userDirs, directories.Directory{Name: dc.Name, Path: dc.Path})
			}
			mergedDirs := directories.Merge(directories.Builtins, userDirs)
			expandedDirs := make([]directories.Directory, 0, len(mergedDirs))
			for _, dir := range mergedDirs {
				p, err := expandTilde(dir.Path, d.userHomeDir)
				if err != nil {
					continue
				}
				expandedDirs = append(expandedDirs, directories.Directory{Name: dir.Name, Path: p})
			}
			directoryProvider := launcher.NewDirectoryProvider(expandedDirs, cfg.Launcher.DirectoryWeight)

			// Commands provider: built-in, hardcoded launcher commands (reload, cleanurl).
			commandsProvider := launcher.NewCommandsProvider(
				launcher.NewBuiltinCommands(homeDir, appsCachePath),
				cfg.Launcher.ScriptWeight,
			)

			m := launcher.NewModel(launcher.ModelConfig{
				Providers: []launcher.Provider{
					launcher.CalcProvider{},
					launcher.NewUnitsProvider(registry, currencyCache, cfg.Launcher.Currencies),
					appsProvider,
					scriptsProvider,
					settingsProvider,
					directoryProvider,
					commandsProvider,
				},
				ConfigErr:        cfgErr,
				CopyText:         d.copyText,
				CurrencyCache:    currencyCache,
				AppsProvider:     appsProvider,
				AppsCachePath:    appsCachePath,
				HomeDir:          homeDir,
				ScriptsProvider:  scriptsProvider,
				CommandsProvider: commandsProvider,
				History:          launcherHistory,
				HistoryPath:      historyPath,
				LearnedRank:      launcherLearnedRank,
				LearnedRankPath:  learnedRankPath,
				LaunchApp: func(appPath string) error {
					launchArgs := apps.LaunchArgs(apps.App{Path: appPath})
					_, err := d.runCommand(launchArgs[0], launchArgs[1:]...)
					return err
				},
				OpenTarget: func(target string) error {
					return platform.OpenURL(target)
				},
				ShowNotification: platform.ShowNotification,
				HideTerminal:     launcherHideTerminal(d),
				// Defer the hide by one render tick so the cleared frame is flushed
				// before Kitty saves the buffer (see Model.resetAndHide).
				HideDelay:   50 * time.Millisecond,
				UseNerdFont: true,
				NoBorder:    noBorder,
			})

			p := tea.NewProgram(m)
			_, err = p.Run()
			return err
		},
	}

	cmd.Flags().BoolVar(&noBorder, "no-border", false, "don't render the outer frame/border")
	cmd.AddCommand(newLauncherReindexCmd(d))
	return cmd
}

// launcherHideTerminal returns the hide command for the quick terminal that
// launched BLF. Ghostty sets GHOSTTY_QUICK_TERMINAL only for its quick
// terminal; all other environments retain the existing Kitty behavior.
func launcherHideTerminal(d deps) func() error {
	if _, ok := d.lookupEnv(ghosttyQuickTerminalEnv); ok {
		return func() error {
			_, err := d.runCommand("osascript", "-e", ghosttyToggleQuickTerminalScript)
			return err
		}
	}

	return func() error {
		_, err := d.runCommand("kitten", "quick-access-terminal", "--instance-group", "quick")
		return err
	}
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
			cacheDir := config.XDGCacheDir(homeDir)
			cachePath := filepath.Join(cacheDir, "apps.json")
			if err := apps.Save(cachePath, idx); err != nil {
				return fmt.Errorf("reindex: save: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "indexed %d apps → %s\n", len(idx.Apps), cachePath)
			return nil
		},
	}
}
