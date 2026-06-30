package claudehistory

import (
	tea "charm.land/bubbletea/v2"
)

// Run launches the claude history TUI.
func Run(projectsRoot string) error {
	m := New(projectsRoot)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
