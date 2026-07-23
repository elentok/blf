package apps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// App is a launchable application entry.
type App struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Subtitle is dimmed trailing context shown in the launcher: the immediate
	// parent folder name when the app is nested below a scan root (e.g.
	// "Utilities"), empty for apps directly under a root.
	Subtitle string `json:"subtitle,omitempty"`
}

// Index is the persisted application index.
type Index struct {
	Apps      []App     `json:"apps"`
	IndexedAt time.Time `json:"indexed_at"`
}

// Load reads an index from path. Returns an empty index if the file is absent or corrupt.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Index{}, nil
		}
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return &Index{}, nil
	}
	return &idx, nil
}

// Save writes idx to path, creating parent directories as needed.
func Save(path string, idx *Index) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Reindex scans platform app directories and returns a fresh Index.
func Reindex(homeDir string) (*Index, error) {
	var appList []App
	switch runtime.GOOS {
	case "darwin":
		appList = ScanMacDirs([]string{
			"/Applications",
			"/System/Applications",
			"/System/Library/CoreServices/Applications",
			filepath.Join(homeDir, "Applications"),
		})
	default:
		dirs := []string{
			"/usr/share/applications",
			"/usr/local/share/applications",
			filepath.Join(homeDir, ".local", "share", "applications"),
		}
		if extra := os.Getenv("XDG_DATA_DIRS"); extra != "" {
			for d := range strings.SplitSeq(extra, ":") {
				dirs = append(dirs, filepath.Join(d, "applications"))
			}
		}
		appList = ScanLinuxDirs(dirs)
	}
	return &Index{Apps: appList, IndexedAt: time.Now()}, nil
}

// LaunchArgs returns the platform-specific command arguments to launch app.
func LaunchArgs(app App) []string {
	switch runtime.GOOS {
	case "darwin":
		return LaunchArgsMac(app)
	default:
		return LaunchArgsLinux(app)
	}
}

// LaunchArgsMac returns the macOS open command for app.
func LaunchArgsMac(app App) []string {
	return []string{"open", "-a", app.Path}
}

// LaunchArgsLinux returns the Linux gio launch command for app.
func LaunchArgsLinux(app App) []string {
	return []string{"gio", "launch", app.Path}
}

// ScanMacDirs recursively scans dirs for .app bundles and returns App entries.
//
// Each root is walked recursively; descent stops at any directory whose name
// ends in ".app" (emitted as an app, never recursed into) and at hidden entries
// (names starting with "."). Apps nested below a root carry their immediate
// parent folder name as Subtitle. Entries are deduplicated by full path only —
// two apps with the same name in different folders both appear.
func ScanMacDirs(dirs []string) []App {
	seen := make(map[string]bool)
	var result []App
	for _, root := range dirs {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				// Unreadable root or entry: skip its subtree, keep walking siblings.
				return nil
			}
			name := d.Name()
			if path != root && strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".app") {
				return nil
			}
			if !seen[path] {
				seen[path] = true
				app := App{
					Name: strings.TrimSuffix(name, ".app"),
					Path: path,
				}
				if parent := filepath.Dir(path); parent != root {
					app.Subtitle = filepath.Base(parent)
				}
				result = append(result, app)
			}
			if d.IsDir() {
				return filepath.SkipDir // don't descend into the bundle
			}
			return nil
		})
	}
	return result
}

// ScanLinuxDirs scans dirs for .desktop files and returns deduplicated App entries.
func ScanLinuxDirs(dirs []string) []App {
	seen := make(map[string]bool)
	var result []App
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			name := ParseDesktopName(path)
			if name == "" {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			result = append(result, App{Name: name, Path: path})
		}
	}
	return result
}

// ParseDesktopName reads the Name= field from a .desktop file.
func ParseDesktopName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if name, ok := strings.CutPrefix(line, "Name="); ok {
			return name
		}
	}
	return ""
}
