package beads

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

func bdArgs(dir string, args ...string) []string {
	if dir == "" {
		return args
	}
	return append([]string{"-C", dir}, args...)
}

// EditIssueCmd shells out to `bd edit <id>` and returns to the TUI afterward.
func EditIssueCmd(dir, id string) tea.Cmd {
	cmd := editIssueCommand(dir, id)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return shellFinishedMsg{issueID: id, err: err, refresh: true}
	})
}

// GraphIssueCmd shells out to `bd graph <id> --compact` and returns to the
// TUI when the process exits.
func GraphIssueCmd(dir, id string) tea.Cmd {
	cmd := graphIssueCommand(dir, id)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return shellFinishedMsg{issueID: id, err: err, refresh: false}
	})
}

func editIssueCommand(dir, id string) *exec.Cmd {
	return exec.Command("bd", bdArgs(dir, "edit", id)...)
}

func graphIssueCommand(dir, id string) *exec.Cmd {
	script := `set -e
if [ -n "$1" ]; then
	cd "$1"
fi
if command -v less >/dev/null 2>&1; then
	exec sh -lc 'bd graph "$1" --compact | less -R' sh "$2"
fi
exec bd graph "$2" --compact`
	return exec.Command("sh", "-lc", script, "sh", dir, id)
}
