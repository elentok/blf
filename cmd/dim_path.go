package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

var dimPathStyle = lipgloss.NewStyle().Faint(true)

func runDimPath(d deps) error {
	useColor := os.Getenv("NO_COLOR") == ""

	scanner := bufio.NewScanner(d.stdin)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.LastIndex(line, "/")
		if idx < 0 {
			fmt.Fprintln(d.stdout, line)
			continue
		}
		dir := line[:idx+1]
		file := line[idx+1:]
		if useColor {
			dir = dimPathStyle.Render(dir)
		}
		fmt.Fprintf(d.stdout, "%s%s\n", dir, file)
	}
	return scanner.Err()
}

func newDimPathCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "dim-path",
		Short: "Dim the directory portion of file paths from stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDimPath(d)
		},
	}
}
