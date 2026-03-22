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

// Config holds all configurable options for lazyspeed.
type Config struct {
	History HistoryConfig `yaml:"history"`
	Test    TestConfig    `yaml:"test"`
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

	return cfg, nil
}

// defaultHistoryPath returns the XDG-compliant default history file path:
// ~/.local/share/lazyspeed/history.json
func defaultHistoryPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".lazyspeed_history.json" // Fallback to cwd if $HOME is unavailable
	}
	return filepath.Join(homeDir, ".local", "share", "lazyspeed", "history.json")
}

// defaultConfigPath returns the XDG-compliant config file path.
// Respects $XDG_CONFIG_HOME if set, otherwise uses ~/.config/lazyspeed/config.yaml.
func defaultConfigPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "lazyspeed", "config.yaml"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
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
