package scripts

import (
	"bytes"
	"runtime"
	"strings"
	"os/exec"
)

// OutputMode controls what happens with a script's stdout after it runs.
type OutputMode string

const (
	OutputIgnore    OutputMode = "ignore"
	OutputShow      OutputMode = "show"
	OutputClipboard OutputMode = "clipboard"
)

// ScriptType is the interpreter used to run a script.
type ScriptType string

const (
	TypeBash      ScriptType = "bash"
	TypeOsascript ScriptType = "osascript"
)

// Script is a named runnable action.
type Script struct {
	Name     string
	Icon     string     // optional nerd-font glyph; empty = use IconRoleScript
	Type     ScriptType
	Platform string     // "mac", "linux", or "" for both
	Body     string
	Output   OutputMode
}

// RunResult is the outcome of an async script execution.
type RunResult struct {
	Stdout string
	Stderr string
	Err    error
}

// Builtins are the scripts shipped with blf. User config adds or overrides them.
var Builtins = []Script{
	{
		Name:     "playpause",
		Type:     TypeOsascript,
		Platform: "mac",
		Body:     `tell application "Spotify" to playpause`,
		Output:   OutputIgnore,
	},
	{
		Name:   "cleanurl",
		Type:   TypeBash,
		Body:   "blf clean-url --clipboard",
		Output: OutputIgnore,
	},
}

// Merge returns the built-in scripts with user overrides applied.
// User scripts with a name matching a built-in replace the built-in; new names
// are appended.
func Merge(builtins, user []Script) []Script {
	result := make([]Script, len(builtins))
	copy(result, builtins)
	for _, u := range user {
		found := false
		for i, b := range result {
			if strings.EqualFold(b.Name, u.Name) {
				result[i] = u
				found = true
				break
			}
		}
		if !found {
			result = append(result, u)
		}
	}
	return result
}

// FilterForPlatform returns scripts compatible with the current OS.
func FilterForPlatform(ss []Script) []Script {
	var out []Script
	for _, s := range ss {
		if s.Platform == "" {
			out = append(out, s)
			continue
		}
		switch runtime.GOOS {
		case "darwin":
			if s.Platform == "mac" {
				out = append(out, s)
			}
		default:
			if s.Platform == "linux" {
				out = append(out, s)
			}
		}
	}
	return out
}

// Run executes s and returns its stdout, stderr, and any exit error.
func Run(s Script) RunResult {
	var cmd *exec.Cmd
	switch s.Type {
	case TypeOsascript:
		cmd = exec.Command("osascript")
		cmd.Stdin = strings.NewReader(s.Body)
	default:
		cmd = exec.Command("bash", "-c", s.Body)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return RunResult{
		Stdout: strings.TrimRight(stdout.String(), "\n"),
		Stderr: strings.TrimRight(stderr.String(), "\n"),
		Err:    err,
	}
}
