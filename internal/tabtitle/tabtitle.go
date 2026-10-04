// Package tabtitle computes the tab title shown in a terminal's tab/pane
// title, porting the logic from the fish function `_tab_title`
// (~/.dotfiles/core/fish/conf.d/tab-title.fish). It is pure: callers pass in
// the command, cwd and home directory, and nothing here touches the
// environment.
package tabtitle

import (
	"math"
	"path/filepath"
	"strings"
)

const (
	separator = "⠀"

	iconNvim    = ""
	iconGxLazyg = ""
	iconTest    = ""
	iconNpmRun  = ""
)

// Title returns the tab title for cmd run in pwd, with home used for the `~`
// directory rule. An empty cmd gives the directory form only.
func Title(cmd, pwd, home string) string {
	var prettyCmd string
	if cmd != "" {
		prettyCmd = prettyCommand(cmd) + separator
	}
	return prettyCmd + Ellipsis(directory(pwd, home), 25)
}

func directory(pwd, home string) string {
	if pwd == home {
		return "~"
	}

	dir := filepath.Base(pwd)
	if dir == "main" || dir == "master" {
		parent := filepath.Base(filepath.Dir(pwd))
		dir = parent + "/" + dir
	}
	return dir
}

// prettyCommand matches cmd against the icon table as a raw prefix match on
// the whole command string, first match wins.
func prettyCommand(cmd string) string {
	switch {
	case strings.HasPrefix(cmd, "nvim"):
		return iconNvim + " "
	case strings.HasPrefix(cmd, "lazyg"):
		return iconGxLazyg + " "
	case strings.HasPrefix(cmd, "gx"):
		return iconGxLazyg + " "
	case strings.HasPrefix(cmd, "npm run test"):
		return iconTest + " "
	case strings.HasPrefix(cmd, "npm run"):
		return iconPrefixedRest(cmd, "npm run ", iconNpmRun)
	case strings.HasPrefix(cmd, "npx vitest"):
		return iconPrefixedRest(cmd, "npx vitest ", iconTest)
	default:
		return Ellipsis(cmd, 10) + ":"
	}
}

// iconPrefixedRest replaces the first occurrence of prefixWithSpace (which
// includes the trailing space) with icon+" ", then ellipsis-cuts the whole
// result to 10 runes so the icon counts inside the cut. When cmd does not
// have that exact prefix (a bare command with no trailing space), it is cut
// to 10 runes unchanged: no icon, no ":".
func iconPrefixedRest(cmd, prefixWithSpace, icon string) string {
	if !strings.HasPrefix(cmd, prefixWithSpace) {
		return Ellipsis(cmd, 10)
	}
	replaced := icon + " " + strings.TrimPrefix(cmd, prefixWithSpace)
	return Ellipsis(replaced, 10)
}

// Ellipsis cuts s to n+1 runes when s is longer than n runes: the first
// round(n/2) runes, then "…", then the last n-round(n/2) runes. Counted in
// runes, not bytes.
func Ellipsis(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}

	lengthStart := int(math.Round(float64(n) / 2))
	lengthEnd := n - lengthStart

	return string(runes[:lengthStart]) + "…" + string(runes[len(runes)-lengthEnd:])
}
