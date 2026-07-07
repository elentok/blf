package beads

import (
	tea "charm.land/bubbletea/v2"
)

// Run launches the beads picker TUI and returns the selected issue id, or ""
// if the user quit without picking one.
func Run(cfg ModelConfig) (string, error) {
	p := tea.NewProgram(NewModel(cfg))
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	return finalModel.(Model).SelectedID(), nil
}
