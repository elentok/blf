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

// ScanMacDirs scans dirs for .app bundles and returns deduplicated App entries.
func ScanMacDirs(dirs []string) []App {
	seen := make(map[string]bool)
	var result []App
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".app")
			if seen[name] {
				continue
			}
			seen[name] = true
			result = append(result, App{
				Name: name,
				Path: filepath.Join(dir, e.Name()),
			})
		}
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
