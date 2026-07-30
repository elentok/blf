package cmd

import (
	"github.com/elentok/blf/internal/claude"
	"github.com/elentok/blf/internal/claudehistory"
	"github.com/spf13/cobra"
)

func newClaudeCmd(d deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Claude Code utilities",
	}
	cmd.AddCommand(
		newClaudeHistoryCmd(),
		newClaudeStatuslineCmd(d),
	)
	return cmd
}

func newClaudeHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Browse Claude Code session history",
		RunE: func(cmd *cobra.Command, args []string) error {
			return claudehistory.Run("")
		},
	}
}

func newClaudeStatuslineCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:                "statusline",
		Short:              "Show Claude status line",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "--install" {
				return claude.InstallStatusLine(claudeInstallDepsFromCmd(d))
			}
			return claude.RunStatusLine(args, d.stdin, d.stdout)
		},
	}
}

func claudeInstallDepsFromCmd(d deps) claude.InstallDeps {
	return claude.InstallDeps{
		Stdout:      d.stdout,
		UserHomeDir: d.userHomeDir,
		ReadFile:    d.readFile,
		WriteFile:   d.writeFile,
		MkdirAll:    d.mkdirAll,
	}
}
