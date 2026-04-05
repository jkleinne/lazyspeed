package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultMaxEntries   = 50
	defaultPingCount    = 1
	defaultFetchTimeout = 30  // seconds
	defaultTestTimeout  = 120 // seconds

	defaultDiagMaxHops    = 30
	defaultDiagTimeout    = 60
	defaultDiagMaxEntries = 20
)

// HistoryConfig holds history-related configuration.
type HistoryConfig struct {
	MaxEntries int    `yaml:"max_entries"`
	Path       string `yaml:"path"`
}

// TestConfig holds test-related configuration.
type TestConfig struct {
	PingCount    int `yaml:"ping_count"`
	FetchTimeout int `yaml:"fetch_timeout"`
	TestTimeout  int `yaml:"test_timeout"`
}

// ExportConfig holds export-related configuration.
type ExportConfig struct {
	Directory string `yaml:"directory"`
}

// DiagnosticsConfig holds network diagnostics configuration.
type DiagnosticsConfig struct {
	MaxHops    int    `yaml:"max_hops"`
	Timeout    int    `yaml:"timeout"`
	MaxEntries int    `yaml:"max_entries"`
	Path       string `yaml:"path"`
}

// ServersConfig holds server preference configuration.
type ServersConfig struct {
	FavoriteIDs []string `yaml:"favorite_ids"`
}

// Config holds all configurable options for lazyspeed.
type Config struct {
	History     HistoryConfig     `yaml:"history"`
	Test        TestConfig        `yaml:"test"`
	Export      ExportConfig      `yaml:"export"`
	Diagnostics DiagnosticsConfig `yaml:"diagnostics"`
	Servers     ServersConfig     `yaml:"servers"`
}

// DefaultConfig returns a Config with all defaults filled in.
func DefaultConfig() *Config {
	return &Config{
		History: HistoryConfig{
			MaxEntries: defaultMaxEntries,
			Path:       defaultHistoryPath(),
		},
		Test: TestConfig{
			PingCount:    defaultPingCount,
			FetchTimeout: defaultFetchTimeout,
			TestTimeout:  defaultTestTimeout,
		},
		Diagnostics: DiagnosticsConfig{
			MaxHops:    defaultDiagMaxHops,
			Timeout:    defaultDiagTimeout,
			MaxEntries: defaultDiagMaxEntries,
			Path:       defaultDiagnosticsPath(),
		},
	}
}

// FavoriteIDSet returns the favorite server IDs as a set for O(1) lookup.
func (c *Config) FavoriteIDSet() map[string]bool {
	set := make(map[string]bool, len(c.Servers.FavoriteIDs))
	for _, id := range c.Servers.FavoriteIDs {
		set[id] = true
	}
	return set
}

// FetchTimeoutDuration returns the configured fetch timeout as a time.Duration.
func (c *Config) FetchTimeoutDuration() time.Duration {
	secs := defaultFetchTimeout
	if c.Test.FetchTimeout > 0 {
		secs = c.Test.FetchTimeout
	}
	return time.Duration(secs) * time.Second
}

// TestTimeoutDuration returns the configured test timeout as a time.Duration.
func (c *Config) TestTimeoutDuration() time.Duration {
	secs := defaultTestTimeout
	if c.Test.TestTimeout > 0 {
		secs = c.Test.TestTimeout
	}
	return time.Duration(secs) * time.Second
}

// PingCount returns the configured ping count.
func (c *Config) PingCount() int {
	if c.Test.PingCount > 0 {
		return c.Test.PingCount
	}
	return defaultPingCount
}

// ExportDir resolves the configured export directory, creating it if it does
// not exist. Falls back to the current working directory if none is configured.
func (c *Config) ExportDir() (string, error) {
	if c.Export.Directory != "" {
		dir := c.Export.Directory
		if dir == "~" || strings.HasPrefix(dir, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to expand home directory: %v", err)
			}
			if dir == "~" {
				dir = home
			} else {
				dir = filepath.Join(home, dir[2:])
			}
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create export directory: %v", err)
		}
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not determine working directory: %v", err)
	}
	return cwd, nil
}

// LoadConfig reads ~/.config/lazyspeed/config.yaml, returning defaults for any
// missing file or unspecified fields. Returns an error only on YAML parse failures.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	configPath, err := defaultConfigPath()
	if err != nil {
		return cfg, fmt.Errorf("failed to resolve config path: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // No config file yet — use defaults
		}
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Unmarshal into a partial struct and overlay onto defaults so unspecified
	// fields retain their default values.
	var partial Config
	if err := yaml.Unmarshal(data, &partial); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Overlay non-zero partial values onto defaults. Each field must be checked
	// individually — adding a new config field requires adding a corresponding
	// overlay line here. This is a maintenance hazard: if a new field is added
	// to the Config struct but not to this overlay, it will silently use its
	// zero value instead of the default.
	if partial.History.MaxEntries > 0 {
		cfg.History.MaxEntries = partial.History.MaxEntries
	}
	if partial.History.Path != "" {
		cfg.History.Path = partial.History.Path
	}
	if partial.Test.PingCount > 0 {
		cfg.Test.PingCount = partial.Test.PingCount
	}
	if partial.Test.FetchTimeout > 0 {
		cfg.Test.FetchTimeout = partial.Test.FetchTimeout
	}
	if partial.Test.TestTimeout > 0 {
		cfg.Test.TestTimeout = partial.Test.TestTimeout
	}
	if partial.Export.Directory != "" {
		cfg.Export.Directory = partial.Export.Directory
	}
	if partial.Diagnostics.MaxHops > 0 {
		cfg.Diagnostics.MaxHops = partial.Diagnostics.MaxHops
	}
	if partial.Diagnostics.Timeout > 0 {
		cfg.Diagnostics.Timeout = partial.Diagnostics.Timeout
	}
	if partial.Diagnostics.MaxEntries > 0 {
		cfg.Diagnostics.MaxEntries = partial.Diagnostics.MaxEntries
	}
	if partial.Diagnostics.Path != "" {
		cfg.Diagnostics.Path = partial.Diagnostics.Path
	}
	if len(partial.Servers.FavoriteIDs) > 0 {
		cfg.Servers.FavoriteIDs = deduplicateStrings(partial.Servers.FavoriteIDs)
	}

	return cfg, nil
}

// xdgDataPath returns the XDG-compliant path for a data file.
// Respects $XDG_DATA_HOME if set, otherwise uses ~/.local/share/lazyspeed/<filename>.
func xdgDataPath(filename string) string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazyspeed", filename)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".lazyspeed_" + filename
	}
	return filepath.Join(homeDir, ".local", "share", "lazyspeed", filename)
}

// defaultHistoryPath returns the XDG-compliant default history file path.
// Respects $XDG_DATA_HOME if set, otherwise uses ~/.local/share/lazyspeed/history.json.
func defaultHistoryPath() string {
	return xdgDataPath("history.json")
}

// defaultDiagnosticsPath returns the XDG-compliant default diagnostics file path.
// Respects $XDG_DATA_HOME if set, otherwise uses ~/.local/share/lazyspeed/diagnostics.json.
func defaultDiagnosticsPath() string {
	return xdgDataPath("diagnostics.json")
}

// defaultConfigPath returns the XDG-compliant config file path.
// Respects $XDG_CONFIG_HOME if set, otherwise uses ~/.config/lazyspeed/config.yaml.
func defaultConfigPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazyspeed", "config.yaml"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %v", err)
	}
	return filepath.Join(homeDir, ".config", "lazyspeed", "config.yaml"), nil
}

// LegacyHistoryPath returns the old history file path used before XDG migration:
// ~/.lazyspeed_history.json
func LegacyHistoryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve legacy history path: %v", err)
	}
	return filepath.Join(homeDir, ".lazyspeed_history.json"), nil
}

const configFilePerm = 0644

// SaveConfig writes the config to the XDG config file using atomic writes.
// Creates the config directory if it does not exist.
func SaveConfig(cfg *Config) error {
	configPath, err := defaultConfigPath()
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %v", err)
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, configFilePerm); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit config file: %v", err)
	}

	return nil
}

// deduplicateStrings returns a new slice with duplicates removed, preserving order.
func deduplicateStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
