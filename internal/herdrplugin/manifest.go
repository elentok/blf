// Package herdrplugin generates blf's herdr plugin manifest: a tiny
// herdr-plugin.toml that hooks a handful of herdr events, each running
// `<blf> herdr sync-tab-title`. See docs/specs/blf-herdr.md, "Plugin".
package herdrplugin

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/elentok/blf/internal/config"
)

const (
	// ID is the plugin id herdr registers it under.
	ID = "blf"
	// Name is the plugin's display name.
	Name = "blf tab titles"
	// ManifestVersion is the manifest's own version, a TOML string raised by
	// one each time the manifest's shape changes. Not the blf version; herdr
	// refuses a bare number here.
	ManifestVersion = "1"
	// MinHerdrVersion is the oldest herdr version this manifest's reply
	// shapes were verified against.
	MinHerdrVersion = "0.9.3"

	// manifestFilename is the plugin folder's one file.
	manifestFilename = "herdr-plugin.toml"
)

// events lists the herdr events the plugin hooks, in the order they're
// written to the manifest. tab.focused is deliberately not hooked: it
// always comes with pane.focused.
var events = []string{
	"pane.focused",
	"tab.created",
	"pane.agent_detected",
	"pane.agent_status_changed",
	"pane.closed",
}

// Dir returns the plugin's folder: $XDG_DATA_HOME/blf/herdr-plugin (default
// ~/.local/share/blf/herdr-plugin). herdr stores this path as absolute, so
// it must not move.
func Dir(homeDir string) string {
	return filepath.Join(config.XDGDataDir(homeDir), "herdr-plugin")
}

// ManifestPath returns the path of the plugin's one file inside Dir.
func ManifestPath(homeDir string) string {
	return filepath.Join(Dir(homeDir), manifestFilename)
}

// ResolveBlfPath picks the <blf> path to write into the manifest.
//
// pathEntry is where "blf" resolves on PATH ("" when it isn't found there),
// resolvedPathEntry is that same entry with symlinks resolved, and
// runningBinary is the currently running binary's path exactly as the OS
// reports it (never further resolved). When the resolved PATH entry matches
// the running binary, the unresolved PATH entry is written, so a Homebrew
// symlink farm keeps working across `brew upgrade`. Any mismatch, including
// blf missing from PATH, falls back to the running binary. (R F16)
func ResolveBlfPath(pathEntry, resolvedPathEntry, runningBinary string) string {
	if pathEntry != "" && resolvedPathEntry == runningBinary {
		return pathEntry
	}
	return runningBinary
}

// GenerateManifest returns the exact herdr-plugin.toml content for blfPath,
// generated from an embedded template rather than read from disk.
func GenerateManifest(blfPath string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "id = %s\n", tomlString(ID))
	fmt.Fprintf(&b, "name = %s\n", tomlString(Name))
	fmt.Fprintf(&b, "version = %s\n", tomlString(ManifestVersion))
	fmt.Fprintf(&b, "platforms = [%s, %s]\n", tomlString("macos"), tomlString("linux"))
	fmt.Fprintf(&b, "min_herdr_version = %s\n", tomlString(MinHerdrVersion))

	command := fmt.Sprintf("[%s, %s, %s]", tomlString(blfPath), tomlString("herdr"), tomlString("sync-tab-title"))

	for _, event := range events {
		b.WriteString("\n[[events]]\n")
		fmt.Fprintf(&b, "on = %s\n", tomlString(event))
		fmt.Fprintf(&b, "command = %s\n", command)
	}

	b.WriteString("\n[[startup]]\n")
	fmt.Fprintf(&b, "command = %s\n", command)

	return b.String()
}

// tomlString renders s as a TOML basic string. Backslash and double-quote
// are the only characters a filesystem path could plausibly contain that
// need escaping.
func tomlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
