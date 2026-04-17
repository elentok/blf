package kitty

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

const sessionPreviewWindow = "right,60%,wrap,<70(down,50%,wrap)"
const fzfNavigationBind = "ctrl-j:down,ctrl-k:up"
const sessionFooter = "ctrl-d: delete"

func pickSession(sessions []Session, d Deps) (string, error) {
	previewCmd, err := sessionPreviewCommand(d)
	if err != nil {
		return "", err
	}
	deleteCmd, err := sessionDeleteCommand(d)
	if err != nil {
		return "", err
	}
	reloadCmd, err := sessionChoicesCommand(d)
	if err != nil {
		return "", err
	}

	cmd := exec.Command(
		"fzf",
		"--layout=reverse",
		"--bind", fzfNavigationBind,
		"--delimiter", "\t",
		"--with-nth", "2",
		"--footer", sessionFooter,
		"--preview", previewCmd,
		"--preview-window", sessionPreviewWindow,
		"--bind", "ctrl-d:execute-silent("+deleteCmd+")+reload("+reloadCmd+")",
	)
	cmd.Stdin = strings.NewReader(formatSessionChoices(sessions))
	cmd.Stderr = d.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return "", nil
		}
		return "", fmt.Errorf("run fzf: %w", err)
	}

	return parseSessionSelection(stdout.String())
}

func sessionPreviewCommand(d Deps) (string, error) {
	return sessionSubcommand(d, PreviewSessionCmd+" {1}")
}

func sessionChoicesCommand(d Deps) (string, error) {
	return sessionSubcommand(d, ListSessionChoicesCmd)
}

func sessionDeleteCommand(d Deps) (string, error) {
	return sessionSubcommand(d, DeleteSessionFileCmd+" {1}")
}

func sessionSubcommand(d Deps, args string) (string, error) {
	if d.ExecutablePath == nil {
		return "", errors.New("executable path resolver is not configured")
	}

	exe, err := d.ExecutablePath()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return shellQuote(exe) + " kitty " + args, nil
}

func parseSessionSelection(line string) (string, error) {
	plain := strings.TrimSpace(line)
	if plain == "" {
		return "", nil
	}

	path, _, found := strings.Cut(plain, "\t")
	if !found {
		return "", fmt.Errorf("invalid kitty session selection %q", plain)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("invalid kitty session selection %q", plain)
	}
	return path, nil
}

func pickOSWindow(windows []OSWindow, d Deps) (string, error) {
	cmd := exec.Command("fzf", "--layout=reverse", "--bind", fzfNavigationBind, "--ansi")
	cmd.Stdin = strings.NewReader(FormatOSWindows(windows))
	cmd.Stderr = d.Stderr

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return "", nil
		}
		return "", fmt.Errorf("run fzf: %w", err)
	}

	return parseOSWindowID(stdout.String())
}

func parseOSWindowID(line string) (string, error) {
	plain := ansiPattern.ReplaceAllString(strings.TrimSpace(line), "")
	if plain == "" {
		return "", nil
	}

	id, _, found := strings.Cut(plain, ":")
	if !found {
		return "", fmt.Errorf("invalid kitty os window selection %q", plain)
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("invalid kitty os window selection %q", plain)
	}

	return id, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
