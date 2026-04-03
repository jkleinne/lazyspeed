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
