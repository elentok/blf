package cmd

import (
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

	if _, err := d.runCommand(name, cmdArgs...); err != nil {
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
		return "osascript", []string{"-e", buildAppleScript(paths)}, nil
	case "linux":
		return "wl-copy", []string{"--type", "text/uri-list", buildURIList(paths)}, nil
	default:
		return "", nil, fmt.Errorf("unsupported on this OS (%s)", goos)
	}
}

// buildAppleScript builds an AppleScript that sets the clipboard to the given
// paths as POSIX file references. Each path is embedded in a double-quoted
// AppleScript literal; because execution goes through exec (no shell), only
// AppleScript-level escaping is required.
func buildAppleScript(paths []string) string {
	refs := make([]string, len(paths))
	for i, p := range paths {
		refs[i] = fmt.Sprintf("POSIX file \"%s\"", escapeAppleScript(p))
	}
	if len(refs) == 1 {
		return "set the clipboard to " + refs[0]
	}
	return "set the clipboard to {" + strings.Join(refs, ", ") + "}"
}

// escapeAppleScript backslash-escapes the characters that are special inside an
// AppleScript string literal. Backslash must be escaped first so the escaping
// of quotes is not doubled.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
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
