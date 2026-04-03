package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestTimeoutDurations(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.FetchTimeoutDuration() != 30*time.Second {
			t.Errorf("Expected 30s fetch timeout, got %v", cfg.FetchTimeoutDuration())
		}
		if cfg.TestTimeoutDuration() != 120*time.Second {
			t.Errorf("Expected 120s test timeout, got %v", cfg.TestTimeoutDuration())
		}
	})

	t.Run("Custom config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Test.FetchTimeout = 10
		cfg.Test.TestTimeout = 60
		if cfg.FetchTimeoutDuration() != 10*time.Second {
			t.Errorf("Expected 10s fetch timeout, got %v", cfg.FetchTimeoutDuration())
		}
		if cfg.TestTimeoutDuration() != 60*time.Second {
			t.Errorf("Expected 60s test timeout, got %v", cfg.TestTimeoutDuration())
		}
	})
}

func TestPingCount(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.PingCount() != 1 {
			t.Errorf("Expected default ping count 1, got %d", cfg.PingCount())
		}
	})

	t.Run("Custom config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Test.PingCount = 5
		if cfg.PingCount() != 5 {
			t.Errorf("Expected ping count 5, got %d", cfg.PingCount())
		}
	})
}

func TestExportDir(t *testing.T) {
	tests := []struct {
		name    string
		wantCWD bool
	}{
		{"empty config uses CWD", true},
		{"configured directory", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			if !tt.wantCWD {
				cfg.Export.Directory = t.TempDir()
			}

			dir, err := cfg.ExportDir()
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			if tt.wantCWD {
				cwd, _ := os.Getwd()
				if dir != cwd {
					t.Errorf("Expected CWD %q, got %q", cwd, dir)
				}
			} else {
				if dir != cfg.Export.Directory {
					t.Errorf("Expected %q, got %q", cfg.Export.Directory, dir)
				}
			}
		})
	}
}

func TestExportDirCreatesDirectory(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "exports", "sub")
	cfg := DefaultConfig()
	cfg.Export.Directory = nested

	dir, err := cfg.ExportDir()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if dir != nested {
		t.Errorf("Expected %q, got %q", nested, dir)
	}
	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("Expected directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected path to be a directory")
	}
}

func TestExportDirTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	cfg := DefaultConfig()
	cfg.Export.Directory = "~/lazyspeed-test-export-" + t.Name()

	dir, err := cfg.ExportDir()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	expected := filepath.Join(home, "lazyspeed-test-export-"+t.Name())
	if dir != expected {
		t.Errorf("Expected %q, got %q", expected, dir)
	}
	// Clean up the created directory
	_ = os.Remove(dir)
}

func TestExportDirBareTilde(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	cfg := DefaultConfig()
	cfg.Export.Directory = "~"

	dir, err := cfg.ExportDir()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if dir != fakeHome {
		t.Errorf("Expected %q, got %q", fakeHome, dir)
	}
}

func TestLoadConfigWithFavorites(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "lazyspeed")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("servers:\n  favorite_ids:\n    - \"111\"\n    - \"222\"\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Servers.FavoriteIDs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(cfg.Servers.FavoriteIDs))
	}
	if cfg.Servers.FavoriteIDs[0] != "111" || cfg.Servers.FavoriteIDs[1] != "222" {
		t.Errorf("unexpected favorites: %v", cfg.Servers.FavoriteIDs)
	}
}

func TestLoadConfigDeduplicatesFavorites(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "lazyspeed")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("servers:\n  favorite_ids:\n    - \"111\"\n    - \"222\"\n    - \"111\"\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Servers.FavoriteIDs) != 2 {
		t.Fatalf("expected 2 deduplicated favorites, got %d", len(cfg.Servers.FavoriteIDs))
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := DefaultConfig()
	cfg.Servers.FavoriteIDs = []string{"111", "222"}
	cfg.Test.PingCount = 3

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Servers.FavoriteIDs) != 2 {
		t.Fatalf("expected 2 favorites after round-trip, got %d", len(loaded.Servers.FavoriteIDs))
	}
	if loaded.Test.PingCount != 3 {
		t.Errorf("expected ping_count 3 preserved, got %d", loaded.Test.PingCount)
	}
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "config")
	t.Setenv("XDG_CONFIG_HOME", nested)

	cfg := DefaultConfig()
	cfg.Servers.FavoriteIDs = []string{"123"}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configFile := filepath.Join(nested, "lazyspeed", "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}

func TestSaveConfigAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := DefaultConfig()
	cfg.Servers.FavoriteIDs = []string{"111"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	cfg.Servers.FavoriteIDs = []string{"111", "222"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(dir, "lazyspeed")
	entries, _ := os.ReadDir(cfgDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover .tmp file: %s", e.Name())
		}
	}
}
