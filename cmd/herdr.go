package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHerdrCmd(d deps) *cobra.Command {
	herdrCmd := &cobra.Command{
		Use:   "herdr",
		Short: "Herdr terminal multiplexer integration",
	}

	herdrCmd.AddCommand(
		newHerdrSyncTabTitleCmd(d),
	)

	return herdrCmd
}

func newHerdrSyncTabTitleCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "sync-tab-title",
		Short: "Sync the herdr tab label from its active pane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHerdrSyncTabTitle(d)
		},
	}
}

// runHerdrSyncTabTitle implements event mode: run by a herdr hook with
// HERDR_TAB_ID set, it syncs that one tab's label from its active pane.
// A full-session sync (no HERDR_TAB_ID, e.g. pane.closed or startup) is
// ticket 04; until that lands, a missing tab id is a silent no-op here.
func runHerdrSyncTabTitle(d deps) error {
	herdrEnv, _ := d.lookupEnv("HERDR_ENV")
	socketPath, _ := d.lookupEnv("HERDR_SOCKET_PATH")
	if herdrEnv == "" || socketPath == "" {
		return nil
	}

	tabID, _ := d.lookupEnv("HERDR_TAB_ID")
	if tabID == "" {
		return nil
	}

	paneID, _ := d.lookupEnv("HERDR_PANE_ID")

	client := d.newHerdrClient(socketPath)

	layout, err := client.PaneLayout(paneID)
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	activePane, err := client.PaneGet(layout.FocusedPaneID)
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	home, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: get home dir: %w", err)
	}

	label := d.herdrLabel(activePane, home)
	if label == "" {
		return nil
	}

	tab, err := client.TabGet(tabID)
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	if tab.Label == label {
		return nil
	}

	if err := client.TabRename(tabID, label); err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	return nil
}
