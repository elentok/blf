package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/elentok/blf/internal/platform"
	"github.com/elentok/blf/internal/tmuxlinks"
	"github.com/elentok/blf/internal/tmuxtargets"
)

type deps struct {
	stdout         io.Writer
	stderr         io.Writer
	stdin          io.Reader
	lookupEnv      func(string) (string, bool)
	openURL        func(string) error
	copyText       func(string) error
	runTmuxLinks   func(string) error
	runTargets     func([]string) error
	lookPath       func(string) (string, error)
	runCommand     func(string, ...string) ([]byte, error)
	fileExists     func(string) (bool, error)
	removeFile     func(string) error
	readFile       func(string) ([]byte, error)
	readDir        func(string) ([]os.DirEntry, error)
	writeFile      func(string, []byte, os.FileMode) error
	mkdirAll       func(string, os.FileMode) error
	executablePath func() (string, error)
	getwd          func() (string, error)
	userHomeDir    func() (string, error)
	now            func() time.Time
}

func defaultDeps() deps {
	return deps{
		stdout:       os.Stdout,
		stderr:       os.Stderr,
		stdin:        os.Stdin,
		lookupEnv:    os.LookupEnv,
		openURL:      platform.OpenURL,
		copyText:     platform.CopyText,
		runTmuxLinks: tmuxlinks.RunMenu,
		runTargets:   tmuxtargets.Execute,
		lookPath:     exec.LookPath,
		runCommand: func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		},
		fileExists:     fileExists,
		removeFile:     os.Remove,
		readFile:       os.ReadFile,
		readDir:        os.ReadDir,
		writeFile:      os.WriteFile,
		mkdirAll:       os.MkdirAll,
		executablePath: os.Executable,
		getwd:          os.Getwd,
		userHomeDir:    os.UserHomeDir,
		now:            time.Now,
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
	case "copy-ref":
		return runCopyRef(args[1:], d)
	case "tmux-targets":
		return d.runTargets(args[1:])
	case "npm-scripts":
		return runNPMScripts(d)
	case "querystring", "qs":
		return runQueryString(args[1:], d)
	case "cal":
		return runCal(args[1:], d)
	case "sum":
		return runSum(args[1:], d)
	case "claude-statusline":
		return runClaudeStatusLine(args[1:], d)
	case "kitty":
		return runKitty(args[1:], d)
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
	fmt.Fprintln(w, "  blf querystring <querystring|-> [key]")
	fmt.Fprintln(w, "  blf qs <querystring|-> [key]")
	fmt.Fprintln(w, "  blf cal [date]")
	fmt.Fprintln(w, "  blf sum [-e|--echo]")
	fmt.Fprintln(w, "  blf claude-statusline [--silent] [--demo]")
	fmt.Fprintln(w, "  blf kitty <ls|list-os-windows|goto-os-window|targets|new-session|sessions|delete-session|doctor> [id]")
	fmt.Fprintln(w, "  blf open <url>")
	fmt.Fprintln(w, "  blf copy <text>")
	fmt.Fprintln(w, "  blf copy-ref <file>...")
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
