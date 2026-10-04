package main

import (
	"fmt"
	"os"

	"github.com/elentok/blf/cmd"
)

func main() {
	if err := cmd.Execute(os.Args[1:]); err != nil {
		// An empty message means the command already printed its own error
		// lines to stderr (e.g. herdr's one-line-per-failed-tab sync).
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "Error:", msg)
		}
		os.Exit(1)
	}
}
