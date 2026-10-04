package herdrsync

import (
	"fmt"

	"github.com/elentok/blf/internal/herdr"
)

// TabPlan is one tab's sync plan: its current label and the label blf would
// set from its active pane, computed without renaming it. Reused by
// full-session sync and by doctor's dry-run table (ticket 10), so they never
// compute this differently.
type TabPlan struct {
	TabID         string
	ActivePaneID  string
	Agent         string
	TerminalTitle string
	Label         string
	NewLabel      string
}

// PlanTabs computes every tab's TabPlan from one session snapshot. A tab
// whose layout or active pane is missing from the snapshot is reported as an
// error and left out of the returned plans; the rest are still computed.
func PlanTabs(
	snapshot herdr.SnapshotResult,
	label func(pane herdr.PaneInfo, home string) string,
	home string,
) ([]TabPlan, []error) {
	panesByID := make(map[string]herdr.PaneInfo, len(snapshot.Panes))
	for _, pane := range snapshot.Panes {
		panesByID[pane.PaneID] = pane
	}

	layoutByTab := make(map[string]herdr.LayoutInfo, len(snapshot.Layouts))
	for _, layout := range snapshot.Layouts {
		layoutByTab[layout.TabID] = layout
	}

	plans := make([]TabPlan, 0, len(snapshot.Tabs))
	var errs []error
	for _, tab := range snapshot.Tabs {
		layout, ok := layoutByTab[tab.TabID]
		if !ok {
			errs = append(errs, fmt.Errorf("tab %s: no layout in snapshot", tab.TabID))
			continue
		}

		pane, ok := panesByID[layout.FocusedPaneID]
		if !ok {
			errs = append(errs, fmt.Errorf("tab %s: active pane %s not in snapshot", tab.TabID, layout.FocusedPaneID))
			continue
		}

		plans = append(plans, TabPlan{
			TabID:         tab.TabID,
			ActivePaneID:  pane.PaneID,
			Agent:         pane.Agent,
			TerminalTitle: pane.TerminalTitle,
			Label:         tab.Label,
			NewLabel:      label(pane, home),
		})
	}

	return plans, errs
}
