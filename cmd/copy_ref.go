package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// runCopyRef copies one or more file references to the system clipboard, so that
// pasting into a GUI app drops/attaches the actual files (not their path text).
func runCopyRef(args []string, d deps) error {
	return copyRefForOS(args, d, runtime.GOOS)
}

// copyRefForOS is the OS-parameterized core of runCopyRef. Taking goos as an
// argument (rather than reading runtime.GOOS directly) lets every OS branch be
// exercised from a single machine in tests.
func copyRefForOS(args []string, d deps, goos string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: blf copy-ref <file>...")
	}

	paths, err := resolveCopyRefPaths(args, d.userHomeDir)
	if err != nil {
		return err
	}

	name, cmdArgs, err := buildCopyRefCommand(goos, paths)
	if err != nil {
		return fmt.Errorf("copy-ref: %w", err)
	}

	if goos == "linux" {
		if _, err := d.lookPath(name); err != nil {
			return fmt.Errorf("copy-ref: %s not found (install wl-clipboard)", name)
		}
	}

	// Use the stderr-passthrough runner: wl-copy forks a daemon child that
	// inherits stderr and lives until the next clipboard write, which would
	// hang a stderr-capturing runner. See runCommandWithoutStderr in cmd.go.
	if _, err := d.runCommandWithoutStderr(name, cmdArgs...); err != nil {
		return fmt.Errorf("copy-ref: %w", err)
	}

	what := pluralize(len(paths), "file reference", "file references")
	fmt.Fprintf(d.stdout, "copied %s to clipboard\n", what)
	return nil
}

// pluralize formats count with the matching noun, using singular only when
// count is exactly one.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// resolveCopyRefPaths expands, absolutizes, and validates every argument up
// front. It is all-or-nothing: if any path is missing the whole command fails
// and nothing is written to the clipboard.
func resolveCopyRefPaths(args []string, homeDir func() (string, error)) ([]string, error) {
	paths := make([]string, 0, len(args))
	for _, arg := range args {
		expanded, err := expandTilde(arg, homeDir)
		if err != nil {
			return nil, fmt.Errorf("copy-ref: %w", err)
		}

		abs, err := filepath.Abs(expanded)
		if err != nil {
			return nil, fmt.Errorf("copy-ref: %w", err)
		}

		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("copy-ref: file not found: %s", abs)
			}
			return nil, fmt.Errorf("copy-ref: %w", err)
		}

		paths = append(paths, abs)
	}
	return paths, nil
}

// expandTilde expands a leading "~" (the home dir) or "~/..." prefix. It
// deliberately leaves "~user" (other users' homes) and a "~" appearing
// mid-path untouched.
func expandTilde(path string, homeDir func() (string, error)) (string, error) {
	if path == "~" {
		return homeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// buildCopyRefCommand returns the OS-specific command (name + args) that puts
// the given absolute paths on the clipboard as file references.
func buildCopyRefCommand(goos string, paths []string) (string, []string, error) {
	switch goos {
	case "darwin":
		script, err := buildMacScript(paths)
		if err != nil {
			return "", nil, err
		}
		return "osascript", []string{"-l", "JavaScript", "-e", script}, nil
	case "linux":
		return "wl-copy", []string{"--type", "text/uri-list", buildURIList(paths)}, nil
	default:
		return "", nil, fmt.Errorf("unsupported on this OS (%s)", goos)
	}
}

// buildMacScript builds a JXA (JavaScript for Automation) script that writes the
// given paths to the clipboard as NSURL file references via NSPasteboard's
// modern writeObjects API.
//
// writeObjects installs lazy data providers, one pasteboard item per file. From
// a short-lived CLI process those providers race with process teardown, so
// copying N files intermittently lands only some of them. To defeat the race we
// force every item's file-url data to materialize in-process (by reading it)
// before returning, which resolves the providers synchronously.
//
// (AppleScript's "set the clipboard to {POSIX file ...}" is not an option: it
// puts an opaque list on the clipboard that GUI apps cannot paste.)
//
// Paths are embedded as a JSON array, which is also valid JavaScript and gives
// correct string escaping.
func buildMacScript(paths []string) (string, error) {
	encoded, err := json.Marshal(paths)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ObjC.import('AppKit');"+
		"var paths=%s;"+
		"var urls=$.NSMutableArray.alloc.init;"+
		"paths.forEach(function(p){urls.addObject($.NSURL.fileURLWithPath(p));});"+
		"var pb=$.NSPasteboard.generalPasteboard;"+
		"pb.clearContents;"+
		"pb.writeObjects(urls);"+
		"var items=pb.pasteboardItems;"+
		"for(var i=0;i<items.count;i++){items.objectAtIndex(i).dataForType('public.file-url');}", encoded), nil
}

// buildURIList builds a newline-joined text/uri-list payload of file:// URIs.
// net/url handles percent-encoding (spaces and other special characters).
func buildURIList(paths []string) string {
	uris := make([]string, len(paths))
	for i, p := range paths {
		u := url.URL{Scheme: "file", Path: p}
		uris[i] = u.String()
	}
	return strings.Join(uris, "\n")
}
