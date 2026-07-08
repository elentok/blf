package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/pkg/browser"
)

var runCommand = func(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func OpenURL(url string) error {
	return browser.OpenURL(url)
}

func CopyText(text string) error {
	return clipboard.WriteAll(text)
}

func ReadClipboardText() (string, error) {
	return clipboard.ReadAll()
}

// ShowNotification displays a system notification with the given title and message.
func ShowNotification(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(
			`display notification "%s" with title "%s"`,
			escapeAppleScriptString(message),
			escapeAppleScriptString(title),
		)
		return runCommand("osascript", "-e", script)
	case "linux":
		return runCommand("notify-send", title, message)
	default:
		return fmt.Errorf("ShowNotification: unsupported platform %q", runtime.GOOS)
	}
}

// escapeAppleScriptString escapes backslashes and double quotes so text can
// be safely embedded inside an AppleScript double-quoted string literal.
func escapeAppleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
