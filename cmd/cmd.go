package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/elentok/blf/internal/platform"
	"github.com/elentok/blf/internal/tmuxlinks"
	"github.com/elentok/blf/internal/tmuxtargets"
)

type deps struct {
	stdout       io.Writer
	stderr       io.Writer
	openURL      func(string) error
	copyText     func(string) error
	runTmuxLinks func(string) error
	runTargets   func([]string) error
	fileExists   func(string) (bool, error)
	readFile     func(string) ([]byte, error)
}

func defaultDeps() deps {
	return deps{
		stdout:       os.Stdout,
		stderr:       os.Stderr,
		openURL:      platform.OpenURL,
		copyText:     platform.CopyText,
		runTmuxLinks: tmuxlinks.RunMenu,
		runTargets:   tmuxtargets.Execute,
		fileExists:   fileExists,
		readFile:     os.ReadFile,
	}
}

func Execute(args []string) error {
	return execute(args, defaultDeps())
}

func execute(args []string, d deps) error {
	if len(args) == 0 {
		printUsage(d.stderr)
		return fmt.Errorf("missing command")
	}

	switch args[0] {
	case "tmux-links":
		return runTmuxLinks(args[1:], d)
	case "open":
		return runOpen(args[1:], d)
	case "copy":
		return runCopy(args[1:], d)
	case "tmux-targets":
		return d.runTargets(args[1:])
	case "npm-scripts":
		return runNPMScripts(d)
	case "version", "-v", "--version":
		return runVersion(d.stdout)
	case "help", "-h", "--help":
		printUsage(d.stdout)
		return nil
	default:
		printUsage(d.stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "blf - Blazingly Fast CLI utilities")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  blf tmux-links <open|copy>")
	fmt.Fprintln(w, "  blf tmux-targets")
	fmt.Fprintln(w, "  blf npm-scripts")
	fmt.Fprintln(w, "  blf open <url>")
	fmt.Fprintln(w, "  blf copy <text>")
	fmt.Fprintln(w, "  blf version")
	fmt.Fprintln(w)
}

func runTmuxLinks(args []string, d deps) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: blf tmux-links <open|copy>")
	}
	mode := args[0]
	if mode != tmuxlinks.ModeOpen && mode != tmuxlinks.ModeCopy {
		return fmt.Errorf("invalid tmux-links mode %q (expected open or copy)", mode)
	}
	return d.runTmuxLinks(mode)
}

func runOpen(args []string, d deps) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: blf open <url>")
	}
	if err := d.openURL(args[0]); err != nil {
		return fmt.Errorf("open url: %w", err)
	}
	return nil
}

func runCopy(args []string, d deps) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: blf copy <text>")
	}
	text := strings.Join(args, " ")
	if err := d.copyText(text); err != nil {
		return fmt.Errorf("copy text: %w", err)
	}
	return nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return !info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
