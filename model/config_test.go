package model

import (
	"testing"
)

func TestLoadConfigPathResolutionError(t *testing.T) {
	// When HOME is unset, defaultConfigPath() fails.
	// LoadConfig should return defaults with a non-nil error.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	cfg, err := LoadConfig()
	if cfg == nil {
		t.Fatal("expected default config, got nil")
	}
	if err == nil {
		t.Fatal("expected error for unresolvable config path")
	}
	if cfg.History.MaxEntries != DefaultConfig().History.MaxEntries {
		t.Error("expected default config values when path resolution fails")
	}
}
