package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/elentok/blf/internal/platform"
	"github.com/elentok/blf/internal/tmuxlinks"
	"github.com/elentok/blf/internal/tmuxtargets"
	"github.com/spf13/cobra"
)

type deps struct {
	stdout                  io.Writer
	stderr                  io.Writer
	stdin                   io.Reader
	lookupEnv               func(string) (string, bool)
	openURL                 func(string) error
	copyText                func(string) error
	runTmuxLinks            func(string) error
	runTargets              func(popup bool, target string) error
	lookPath                func(string) (string, error)
	runCommand              func(string, ...string) ([]byte, error)
	runCommandWithoutStderr func(string, ...string) ([]byte, error)
	fileExists              func(string) (bool, error)
	removeFile              func(string) error
	readFile                func(string) ([]byte, error)
	readDir                 func(string) ([]os.DirEntry, error)
	writeFile               func(string, []byte, os.FileMode) error
	mkdirAll                func(string, os.FileMode) error
	executablePath          func() (string, error)
	getwd                   func() (string, error)
	userHomeDir             func() (string, error)
	now                     func() time.Time
	signalProcess           func(pid int, sig syscall.Signal) error
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
		// runCommandWithoutStderr is for commands that fork a long-lived child
		// which inherits stderr (e.g. wl-copy, whose daemon child holds the
		// clipboard until the next write). Output() would capture stderr via an
		// os.Pipe and block in Wait() until that write end closes, which never
		// happens while the child lives, hanging us forever. Setting Stderr to
		// an *os.File hands the fd straight through, so Wait() returns when the
		// direct child exits.
		runCommandWithoutStderr: func(name string, args ...string) ([]byte, error) {
			cmd := exec.Command(name, args...)
			cmd.Stderr = os.Stderr
			return cmd.Output()
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
		signalProcess: func(pid int, sig syscall.Signal) error {
			return syscall.Kill(pid, sig)
		},
	}
}

func Execute(args []string) error {
	return execute(args, defaultDeps())
}

func execute(args []string, d deps) error {
	root := &cobra.Command{
		Use:           "blf",
		Short:         "blf - Blazingly Fast CLI utilities",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       getVersion(),
	}
	root.SetOut(d.stdout)
	root.SetErr(d.stderr)

	root.AddCommand(
		newTmuxCmd(d),
		newOpenCmd(d),
		newCopyCmd(d),
		newCopyRefCmd(d),
		newNPMScriptsCmd(d),
		newQueryStringCmd(d),
		newCalCmd(d),
		newSumCmd(d),
		newClaudeCmd(d),
		newKittyCmd(d),
		newDimPathCmd(d),
		newCleanURLCmd(d),
		newLauncherCmd(d),
		newBeadsCmd(d),
		newConfigCmd(d),
		newPowerCmd(d),
		newVersionCmd(d),
	)

	root.SetArgs(args)
	return root.Execute()
}

func newOpenCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "open <url>",
		Short: "Open a URL in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := d.openURL(args[0]); err != nil {
				return fmt.Errorf("open url: %w", err)
			}
			return nil
		},
	}
}

func newCopyCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "copy <text>",
		Short: "Copy text to clipboard (use - to read from stdin)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := copyInput(args, d)
			if err != nil {
				return err
			}
			if err := d.copyText(text); err != nil {
				return fmt.Errorf("copy text: %w", err)
			}
			return nil
		},
	}
}

// copyInput returns the text to copy for the `copy` command: stdin (trimmed of
// trailing newlines) when the sole argument is "-", otherwise the arguments
// joined with spaces.
func copyInput(args []string, d deps) (string, error) {
	if len(args) == 1 && args[0] == "-" {
		data, err := io.ReadAll(d.stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		text := strings.TrimRight(string(data), "\r\n")
		if text == "" {
			return "", fmt.Errorf("copy: empty input")
		}
		return text, nil
	}
	return strings.Join(args, " "), nil
}

func newCopyRefCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "copy-ref <file>...",
		Short: "Copy file references to clipboard",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopyRef(args, d)
		},
	}
}

func newNPMScriptsCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "npm-scripts",
		Short: "List npm scripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNPMScripts(d)
		},
	}
}

func newQueryStringCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:     "querystring <querystring|-> [key]",
		Aliases: []string{"qs"},
		Short:   "Parse and query URL query strings",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQueryString(args, d)
		},
	}
}

func newCalCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "cal [date]",
		Short: "Show a calendar",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCal(args, d)
		},
	}
}

func newSumCmd(d deps) *cobra.Command {
	var echo bool

	cmd := &cobra.Command{
		Use:   "sum",
		Short: "Sum numbers from stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSumWithEcho(echo, d)
		},
	}
	cmd.Flags().BoolVarP(&echo, "echo", "e", false, "Echo each line")
	return cmd
}

func newVersionCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(d.stdout)
		},
	}
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
