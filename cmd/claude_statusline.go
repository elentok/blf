package cmd

import "github.com/elentok/blf/internal/claude"

func runClaudeStatusLine(args []string, d deps) error {
	return claude.RunStatusLine(args, d.stdin, d.stdout)
}
