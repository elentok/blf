package tmuxutil

import (
	"os"
	"strings"
)

const defaultDelayMS = "5000"

type Runner func(name string, args ...string) error

func DisplayMessage(run Runner, msg string) {
	if run == nil {
		return
	}
	if os.Getenv("TMUX") == "" {
		return
	}
	clean := sanitize(msg)
	if clean == "" {
		return
	}
	_ = run("tmux", "display-message", "-d", defaultDelayMS, clean)
}

func DisplayToolMessage(run Runner, tool, msg string) {
	prefix := "blf " + strings.TrimSpace(tool) + ": "
	raw := sanitize(msg)
	if raw == "" {
		return
	}
	if strings.HasPrefix(raw, prefix) {
		DisplayMessage(run, raw)
		return
	}
	DisplayMessage(run, prefix+raw)
}

func DisplayToolError(run Runner, tool string, err error) {
	if err == nil {
		return
	}
	DisplayToolMessage(run, tool, err.Error())
}

func sanitize(msg string) string {
	return strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))
}
