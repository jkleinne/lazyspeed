package model

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultMaxEntries   = 50
	defaultPingCount    = 10
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

// Config holds all configurable options for lazyspeed.
type Config struct {
	History     HistoryConfig     `yaml:"history"`
	Test        TestConfig        `yaml:"test"`
	Export      ExportConfig      `yaml:"export"`
	Diagnostics DiagnosticsConfig `yaml:"diagnostics"`
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

// LoadConfig reads ~/.config/lazyspeed/config.yaml, returning defaults for any
// missing file or unspecified fields. Returns an error only on YAML parse failures.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	configPath, err := defaultConfigPath()
	if err != nil {
		// Cannot resolve config dir — use defaults silently
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // No config file yet — use defaults
		}
		return cfg, nil // Unreadable config — use defaults silently
	}

	// Unmarshal into a partial struct and overlay onto defaults so unspecified
	// fields retain their default values.
	var partial Config
	if err := yaml.Unmarshal(data, &partial); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

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
		return "", err
	}
	return filepath.Join(homeDir, ".lazyspeed_history.json"), nil
}
