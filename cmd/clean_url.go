package cmd

import (
	"fmt"

	"github.com/elentok/blf/internal/cleanurl"
	"github.com/spf13/cobra"
)

func runCleanURL(rawURL string, useClipboard bool, d deps) error {
	if useClipboard {
		if err := cleanurl.RunClipboard(); err != nil {
			return err
		}
		return nil
	}

	fmt.Fprintln(d.stdout, cleanurl.CleanURL(rawURL))
	return nil
}

func newCleanURLCmd(d deps) *cobra.Command {
	var useClipboard bool

	cmd := &cobra.Command{
		Use:   "clean-url [url]",
		Short: "Unwrap redirect wrappers and strip tracking params from a URL",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case useClipboard && len(args) == 1:
				return fmt.Errorf("specify either <url> or --clipboard, not both")
			case !useClipboard && len(args) == 0:
				return fmt.Errorf("specify either <url> or --clipboard")
			}

			rawURL := ""
			if len(args) == 1 {
				rawURL = args[0]
			}
			return runCleanURL(rawURL, useClipboard, d)
		},
	}
	cmd.Flags().BoolVar(&useClipboard, "clipboard", false, "Read the URL from the clipboard and write the cleaned URL back")
	return cmd
}
