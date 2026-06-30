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
		newClaudeStatuslineAliasCmd(d),
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

func newClaudeStatuslineAliasCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:                "statusline",
		Short:              "Show Claude status line",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return claude.RunStatusLine(args, d.stdin, d.stdout)
		},
	}
}
