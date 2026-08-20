// Package config holds the blf-wide configuration file: its schema, defaults,
// loading, and XDG path resolution. Individual commands (e.g. the launcher)
// read the sections they care about from the decoded Config.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

//go:embed snippets.schema.json
var snippetsSchemaJSON []byte

// SnippetsSchemaJSON is the JSON Schema for snippets.toml, understood by
// taplo (and editors built on it, e.g. Even Better TOML) via a `#:schema`
// magic comment.
func SnippetsSchemaJSON() []byte {
	return snippetsSchemaJSON
}

// SnippetsSchemaFilename is the name of the schema file written alongside
// snippets.toml.
const SnippetsSchemaFilename = "snippets.schema.json"

//go:embed config.schema.json
var configSchemaJSON []byte

// ConfigSchemaJSON is the JSON Schema for config.toml, understood by taplo
// (and editors built on it, e.g. Even Better TOML) via a `#:schema` magic
// comment.
func ConfigSchemaJSON() []byte {
	return configSchemaJSON
}

// ConfigSchemaFilename is the name of the schema file written alongside
// config.toml.
const ConfigSchemaFilename = "config.schema.json"

// TaploTomlFilename is the taplo config file written to the blf config
// directory, associating both config.toml and snippets.toml with their
// schemas by filename (belt-and-suspenders alongside each file's own
// `#:schema` pragma — see https://taplo.tamasfe.dev/configuration/).
const TaploTomlFilename = "taplo.toml"

// TaploTomlContent is the content written to TaploTomlFilename.
func TaploTomlContent() []byte {
	return []byte(`[[schema]]
path = "./` + ConfigSchemaFilename + `"
include = ["config.toml"]

[[schema]]
path = "./` + SnippetsSchemaFilename + `"
include = ["snippets.toml"]
`)
}

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
	// SnippetsWeight is the source weight for snippets (default 1.0).
	SnippetsWeight float64 `toml:"snippets_weight"`
	// AI is the [launcher.ai] section configuring the AI helpers.
	AI AIConfig `toml:"ai"`
}

// AIConfig is the [launcher.ai] section of ~/.config/blf/config.toml. It
// exposes only what a user could reasonably want to tune; the claude flag
// list itself is blf's contract, not a config knob.
type AIConfig struct {
	// Model is the claude model alias to invoke (default "haiku").
	Model string `toml:"model"`
	// Timeout is a duration string, e.g. "120s" (default "120s"). It's a
	// string rather than a duration field because the TOML decoder in use
	// can't decode into a duration type, and an integer would be read as
	// raw nanoseconds. Parsed with time.ParseDuration; an unparseable
	// value falls back to the default via LoadConfig's validation.
	Timeout string `toml:"timeout"`
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

// SnippetConfig is a named text value in snippets.toml, copied to the
// clipboard when selected in the launcher.
type SnippetConfig struct {
	Name  string `toml:"name"`
	Icon  string `toml:"icon"` // optional nerd-font glyph
	Value string `toml:"value"`
}

// Snippets holds the top-level contents of snippets.toml.
type Snippets struct {
	Snippets []SnippetConfig `toml:"snippet"`
}

var defaultCurrencies = []string{"USD", "ILS", "GBP", "EUR"}

// DefaultConfig returns the Config used when no config file is present, and
// as the seed written by `blf config edit` for a first-time config file.
func DefaultConfig() Config {
	return Config{
		Launcher: LauncherConfig{
			ScriptWeight:    1.5,
			AppWeight:       1.0,
			DirectoryWeight: 1.0,
			SnippetsWeight:  1.0,
			Currencies:      defaultCurrencies,
			AI: AIConfig{
				Model:   "haiku",
				Timeout: "120s",
			},
		},
	}
}

// LoadConfig reads ~/.config/blf/config.toml (respecting XDG_CONFIG_HOME).
// If the file is absent it returns defaults with no error. If the file is
// malformed it returns defaults plus a non-nil error so the caller can show a
// non-blocking notice without crashing.
func LoadConfig(readFile func(string) ([]byte, error), homeDir string) (Config, error) {
	path := XDGConfigPath(homeDir)
	data, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), fmt.Errorf("read config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return DefaultConfig(), fmt.Errorf("malformed config %s: %w", path, err)
	}

	if _, err := time.ParseDuration(cfg.Launcher.AI.Timeout); err != nil {
		badTimeout := cfg.Launcher.AI.Timeout
		cfg.Launcher.AI.Timeout = DefaultConfig().Launcher.AI.Timeout
		return cfg, fmt.Errorf("invalid launcher.ai timeout %q in %s: %w", badTimeout, path, err)
	}

	return cfg, nil
}

// XDGConfigPath returns the path to the blf config file.
func XDGConfigPath(homeDir string) string {
	return filepath.Join(xdgConfigDir(homeDir), "config.toml")
}

// XDGSnippetsPath returns the path to the blf snippets file.
func XDGSnippetsPath(homeDir string) string {
	return filepath.Join(xdgConfigDir(homeDir), "snippets.toml")
}

func xdgConfigDir(homeDir string) string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "blf")
	}
	return filepath.Join(homeDir, ".config", "blf")
}

// LoadSnippets reads ~/.config/blf/snippets.toml (respecting
// XDG_CONFIG_HOME). If the file is absent it returns an empty list with no
// error. If the file is malformed it returns an empty list plus a non-nil
// error so the caller can show a non-blocking notice without crashing.
func LoadSnippets(readFile func(string) ([]byte, error), homeDir string) ([]SnippetConfig, error) {
	path := XDGSnippetsPath(homeDir)
	data, err := readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read snippets %s: %w", path, err)
	}

	var snippets Snippets
	if _, err := toml.Decode(string(data), &snippets); err != nil {
		return nil, fmt.Errorf("malformed snippets %s: %w", path, err)
	}
	return snippets.Snippets, nil
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
