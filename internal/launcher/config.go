package launcher

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds the full blf configuration. Only the [launcher] section is
// used by the launcher; other sections are reserved for future commands.
type Config struct {
	Launcher LauncherConfig `toml:"launcher"`
}

// LauncherConfig is the [launcher] section of ~/.config/blf/config.toml.
type LauncherConfig struct {
	// ScriptWeight is the source weight for user-defined scripts (default 1.5).
	ScriptWeight float64 `toml:"script_weight"`
	// AppWeight is the source weight for installed applications (default 1.0).
	AppWeight float64 `toml:"app_weight"`
	// Currencies is the ordered list of ISO currency codes shown in conversion results.
	Currencies []string `toml:"currencies"`
	// UnitGroups allows users to define custom unit conversion groups.
	UnitGroups []UnitGroupConfig `toml:"unit_group"`
	// Scripts lists user scripts that add to or override the built-in defaults.
	Scripts []ScriptConfig `toml:"script"`
	// DirectoryWeight is the source weight for directories (default 1.0).
	DirectoryWeight float64 `toml:"directory_weight"`
	// Directories lists user directories that add to or override the built-in defaults.
	Directories []DirectoryConfig `toml:"directory"`
}

// DirectoryConfig is a named filesystem location in the config file.
type DirectoryConfig struct {
	Name string `toml:"name"`
	Path string `toml:"path"` // may start with "~"
}

// ScriptConfig is a named runnable action in the config file.
type ScriptConfig struct {
	Name     string `toml:"name"`
	Icon     string `toml:"icon"`     // optional nerd-font glyph
	Type     string `toml:"type"`     // "bash" or "osascript"
	Platform string `toml:"platform"` // "mac", "linux", or "" for both
	Body     string `toml:"body"`
	Output   string `toml:"output"` // "ignore", "show", "clipboard"
}

// UnitGroupConfig is a user-defined unit group in the config file.
type UnitGroupConfig struct {
	Name  string       `toml:"name"`
	Units []UnitConfig `toml:"unit"`
}

// UnitConfig is one unit within a user-defined UnitGroupConfig.
type UnitConfig struct {
	Name    string   `toml:"name"`
	Symbols []string `toml:"symbols"`
	Factor  float64  `toml:"factor"`
	Offset  float64  `toml:"offset"`
}

var defaultCurrencies = []string{"USD", "ILS", "GBP", "EUR"}

func defaultConfig() Config {
	return Config{
		Launcher: LauncherConfig{
			ScriptWeight:    1.5,
			AppWeight:       1.0,
			DirectoryWeight: 1.0,
			Currencies:      defaultCurrencies,
		},
	}
}

// LoadConfig reads ~/.config/blf/config.toml (respecting XDG_CONFIG_HOME).
// If the file is absent it returns defaults with no error. If the file is
// malformed it returns defaults plus a non-nil error so the caller can show a
// non-blocking notice without crashing.
func LoadConfig(readFile func(string) ([]byte, error), homeDir string) (Config, error) {
	path := xdgConfigPath(homeDir)
	data, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return defaultConfig(), fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := defaultConfig()
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return defaultConfig(), fmt.Errorf("malformed config %s: %w", path, err)
	}
	return cfg, nil
}

// XDGConfigPath returns the path to the blf config file.
func XDGConfigPath(homeDir string) string {
	return xdgConfigPath(homeDir)
}

// XDGCacheDir returns the blf cache directory (~/.cache/blf or $XDG_CACHE_HOME/blf).
func XDGCacheDir(homeDir string) string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "blf")
	}
	return filepath.Join(homeDir, ".cache", "blf")
}

// XDGStateDir returns the blf state directory (~/.local/state/blf or $XDG_STATE_HOME/blf).
func XDGStateDir(homeDir string) string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "blf")
	}
	return filepath.Join(homeDir, ".local", "state", "blf")
}

func xdgConfigPath(homeDir string) string {
	configDir := filepath.Join(homeDir, ".config", "blf")
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		configDir = filepath.Join(d, "blf")
	}
	return filepath.Join(configDir, "config.toml")
}
