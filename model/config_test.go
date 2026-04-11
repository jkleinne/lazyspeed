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

// ptrFloat64 is a test helper that returns a pointer to a float64 value.
func ptrFloat64(v float64) *float64 { return &v }

func TestWebhookConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Webhooks.Endpoints) != 0 {
		t.Errorf("expected empty endpoints, got %d", len(cfg.Webhooks.Endpoints))
	}
	if cfg.Webhooks.Timeout != defaultWebhookTimeout {
		t.Errorf("expected timeout %d, got %d", defaultWebhookTimeout, cfg.Webhooks.Timeout)
	}
	if cfg.Webhooks.MaxRetries != defaultWebhookMaxRetries {
		t.Errorf("expected max_retries %d, got %d", defaultWebhookMaxRetries, cfg.Webhooks.MaxRetries)
	}
	if cfg.Webhooks.Thresholds.MinDownload != nil {
		t.Error("expected nil MinDownload threshold")
	}
	if cfg.Webhooks.Thresholds.MinUpload != nil {
		t.Error("expected nil MinUpload threshold")
	}
	if cfg.Webhooks.Thresholds.MaxPing != nil {
		t.Error("expected nil MaxPing threshold")
	}
	if cfg.Webhooks.Thresholds.MaxJitter != nil {
		t.Error("expected nil MaxJitter threshold")
	}
}

func TestLoadConfigWithWebhooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "lazyspeed")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}

	content := []byte(`webhooks:
  timeout: 15
  max_retries: 3
  endpoints:
    - url: "https://example.com/hook"
    - url: "https://other.com/hook"
      headers:
        Authorization: "Bearer token"
        X-Custom: "value"
  thresholds:
    min_download: 50.0
    max_ping: 100.0
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Webhooks.Timeout != 15 {
		t.Errorf("expected timeout 15, got %d", cfg.Webhooks.Timeout)
	}
	if cfg.Webhooks.MaxRetries != 3 {
		t.Errorf("expected max_retries 3, got %d", cfg.Webhooks.MaxRetries)
	}
	if len(cfg.Webhooks.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(cfg.Webhooks.Endpoints))
	}
	if cfg.Webhooks.Endpoints[0].URL != "https://example.com/hook" {
		t.Errorf("unexpected first endpoint URL: %s", cfg.Webhooks.Endpoints[0].URL)
	}
	if len(cfg.Webhooks.Endpoints[0].Headers) != 0 {
		t.Errorf("expected no headers on first endpoint, got %d", len(cfg.Webhooks.Endpoints[0].Headers))
	}
	if cfg.Webhooks.Endpoints[1].URL != "https://other.com/hook" {
		t.Errorf("unexpected second endpoint URL: %s", cfg.Webhooks.Endpoints[1].URL)
	}
	if cfg.Webhooks.Endpoints[1].Headers["Authorization"] != "Bearer token" {
		t.Errorf("unexpected Authorization header: %s", cfg.Webhooks.Endpoints[1].Headers["Authorization"])
	}
	if cfg.Webhooks.Endpoints[1].Headers["X-Custom"] != "value" {
		t.Errorf("unexpected X-Custom header: %s", cfg.Webhooks.Endpoints[1].Headers["X-Custom"])
	}

	// min_download and max_ping set; min_upload and max_jitter absent (nil).
	if cfg.Webhooks.Thresholds.MinDownload == nil {
		t.Fatal("expected non-nil MinDownload threshold")
	}
	if *cfg.Webhooks.Thresholds.MinDownload != 50.0 {
		t.Errorf("expected MinDownload 50.0, got %f", *cfg.Webhooks.Thresholds.MinDownload)
	}
	if cfg.Webhooks.Thresholds.MinUpload != nil {
		t.Error("expected nil MinUpload threshold (not set in YAML)")
	}
	if cfg.Webhooks.Thresholds.MaxPing == nil {
		t.Fatal("expected non-nil MaxPing threshold")
	}
	if *cfg.Webhooks.Thresholds.MaxPing != 100.0 {
		t.Errorf("expected MaxPing 100.0, got %f", *cfg.Webhooks.Thresholds.MaxPing)
	}
	if cfg.Webhooks.Thresholds.MaxJitter != nil {
		t.Error("expected nil MaxJitter threshold (not set in YAML)")
	}
}

func TestValidateWebhookConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     WebhookConfig
		wantErr bool
	}{
		{
			name:    "empty config is valid",
			cfg:     WebhookConfig{},
			wantErr: false,
		},
		{
			name: "valid single endpoint",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: false,
		},
		{
			name: "empty URL rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: ""}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "non-http scheme rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "ftp://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "missing scheme rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "max_retries zero rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 0,
			},
			wantErr: true,
		},
		{
			name: "max_retries above cap rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: maxWebhookRetries + 1,
			},
			wantErr: true,
		},
		{
			name: "timeout zero rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    0,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "negative min_download threshold rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
				Thresholds: ThresholdConfig{MinDownload: ptrFloat64(-1.0)},
			},
			wantErr: true,
		},
		{
			name: "negative min_upload threshold rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
				Thresholds: ThresholdConfig{MinUpload: ptrFloat64(-0.1)},
			},
			wantErr: true,
		},
		{
			name: "negative max_ping threshold rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
				Thresholds: ThresholdConfig{MaxPing: ptrFloat64(-5.0)},
			},
			wantErr: true,
		},
		{
			name: "negative max_jitter threshold rejected",
			cfg: WebhookConfig{
				Endpoints:  []WebhookEndpoint{{URL: "https://example.com/hook"}},
				Timeout:    10,
				MaxRetries: 1,
				Thresholds: ThresholdConfig{MaxJitter: ptrFloat64(-2.0)},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWebhookConfig(tt.cfg)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMetricsConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Metrics.Endpoints) != 0 {
		t.Errorf("expected empty endpoints, got %d", len(cfg.Metrics.Endpoints))
	}
	if cfg.Metrics.Timeout != defaultMetricsTimeout {
		t.Errorf("expected timeout %d, got %d", defaultMetricsTimeout, cfg.Metrics.Timeout)
	}
	if cfg.Metrics.MaxRetries != defaultMetricsMaxRetries {
		t.Errorf("expected max_retries %d, got %d", defaultMetricsMaxRetries, cfg.Metrics.MaxRetries)
	}
	if cfg.Metrics.HostTag != "" {
		t.Errorf("expected empty host_tag, got %q", cfg.Metrics.HostTag)
	}
	if cfg.Metrics.OmitHostTag {
		t.Error("expected omit_host_tag to be false")
	}
}

func TestValidateMetricsConfig(t *testing.T) {
	v2ok := &InfluxV2{Token: "t", Org: "o", Bucket: "b"}
	v1ok := &InfluxV1{Database: "db"}

	tests := []struct {
		name    string
		cfg     MetricsConfig
		wantErr bool
	}{
		{
			name:    "empty config is valid",
			cfg:     MetricsConfig{},
			wantErr: false,
		},
		{
			name: "valid v2 endpoint",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: v2ok}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: false,
		},
		{
			name: "valid v1 endpoint",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "http://localhost:8086", V1: v1ok}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: false,
		},
		{
			name: "empty URL rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "", V2: v2ok}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "non-http scheme rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "ftp://example.com", V2: v2ok}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "missing scheme rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "example.com", V2: v2ok}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "neither v1 nor v2 rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com"}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "both v1 and v2 rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V1: v1ok, V2: v2ok}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "v2 missing token rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: &InfluxV2{Org: "o", Bucket: "b"}}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "v2 missing org rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: &InfluxV2{Token: "t", Bucket: "b"}}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "v2 missing bucket rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: &InfluxV2{Token: "t", Org: "o"}}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "v1 missing database rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "http://localhost:8086", V1: &InfluxV1{}}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "v1 password without username rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "http://localhost:8086", V1: &InfluxV1{Database: "db", Password: "p"}}},
				Timeout:    10,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "timeout zero rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: v2ok}},
				Timeout:    0,
				MaxRetries: 1,
			},
			wantErr: true,
		},
		{
			name: "max_retries zero rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: v2ok}},
				Timeout:    10,
				MaxRetries: 0,
			},
			wantErr: true,
		},
		{
			name: "max_retries above cap rejected",
			cfg: MetricsConfig{
				Endpoints:  []MetricsEndpoint{{URL: "https://example.com", V2: v2ok}},
				Timeout:    10,
				MaxRetries: maxWebhookRetries + 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetricsConfig(tt.cfg)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSaveConfigWithMetrics(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := DefaultConfig()
	cfg.Metrics = MetricsConfig{
		Timeout:    20,
		MaxRetries: 3,
		HostTag:    "custom-host",
		Endpoints: []MetricsEndpoint{
			{
				URL: "https://influx.example.com:8086",
				V2: &InfluxV2{
					Token:  "abc123",
					Org:    "my-org",
					Bucket: "speedtest",
				},
			},
			{
				URL: "http://localhost:8086",
				V1: &InfluxV1{
					Database: "lazyspeed",
					Username: "admin",
					Password: "secret",
				},
			},
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(loaded.Metrics.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(loaded.Metrics.Endpoints))
	}
	if loaded.Metrics.Endpoints[0].V2 == nil {
		t.Fatal("expected endpoint 0 to have V2 set")
	}
	if loaded.Metrics.Endpoints[0].V2.Token != "abc123" {
		t.Errorf("token mismatch: got %q", loaded.Metrics.Endpoints[0].V2.Token)
	}
	if loaded.Metrics.Endpoints[0].V2.Org != "my-org" {
		t.Errorf("org mismatch: got %q", loaded.Metrics.Endpoints[0].V2.Org)
	}
	if loaded.Metrics.Endpoints[0].V2.Bucket != "speedtest" {
		t.Errorf("bucket mismatch: got %q", loaded.Metrics.Endpoints[0].V2.Bucket)
	}
	if loaded.Metrics.Endpoints[1].V1 == nil {
		t.Fatal("expected endpoint 1 to have V1 set")
	}
	if loaded.Metrics.Endpoints[1].V1.Database != "lazyspeed" {
		t.Errorf("database mismatch: got %q", loaded.Metrics.Endpoints[1].V1.Database)
	}
	if loaded.Metrics.Endpoints[1].V1.Username != "admin" {
		t.Errorf("username mismatch: got %q", loaded.Metrics.Endpoints[1].V1.Username)
	}
	if loaded.Metrics.Endpoints[1].V1.Password != "secret" {
		t.Errorf("password mismatch: got %q", loaded.Metrics.Endpoints[1].V1.Password)
	}
	if loaded.Metrics.Timeout != 20 {
		t.Errorf("timeout mismatch: got %d", loaded.Metrics.Timeout)
	}
	if loaded.Metrics.MaxRetries != 3 {
		t.Errorf("max_retries mismatch: got %d", loaded.Metrics.MaxRetries)
	}
	if loaded.Metrics.HostTag != "custom-host" {
		t.Errorf("host_tag mismatch: got %q", loaded.Metrics.HostTag)
	}
}

func TestSaveConfigWithWebhooks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := DefaultConfig()
	cfg.Webhooks = WebhookConfig{
		Timeout:    20,
		MaxRetries: 2,
		Endpoints: []WebhookEndpoint{
			{
				URL: "https://example.com/hook",
				Headers: map[string]string{
					"Authorization": "Bearer secret",
				},
			},
		},
		Thresholds: ThresholdConfig{
			MinDownload: ptrFloat64(25.0),
			MaxPing:     ptrFloat64(200.0),
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error saving config: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if loaded.Webhooks.Timeout != 20 {
		t.Errorf("expected timeout 20, got %d", loaded.Webhooks.Timeout)
	}
	if loaded.Webhooks.MaxRetries != 2 {
		t.Errorf("expected max_retries 2, got %d", loaded.Webhooks.MaxRetries)
	}
	if len(loaded.Webhooks.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(loaded.Webhooks.Endpoints))
	}
	if loaded.Webhooks.Endpoints[0].URL != "https://example.com/hook" {
		t.Errorf("unexpected endpoint URL: %s", loaded.Webhooks.Endpoints[0].URL)
	}
	if loaded.Webhooks.Endpoints[0].Headers["Authorization"] != "Bearer secret" {
		t.Errorf("unexpected Authorization header: %s", loaded.Webhooks.Endpoints[0].Headers["Authorization"])
	}
	if loaded.Webhooks.Thresholds.MinDownload == nil {
		t.Fatal("expected non-nil MinDownload after round-trip")
	}
	if *loaded.Webhooks.Thresholds.MinDownload != 25.0 {
		t.Errorf("expected MinDownload 25.0, got %f", *loaded.Webhooks.Thresholds.MinDownload)
	}
	if loaded.Webhooks.Thresholds.MinUpload != nil {
		t.Error("expected nil MinUpload after round-trip (not set)")
	}
	if loaded.Webhooks.Thresholds.MaxPing == nil {
		t.Fatal("expected non-nil MaxPing after round-trip")
	}
	if *loaded.Webhooks.Thresholds.MaxPing != 200.0 {
		t.Errorf("expected MaxPing 200.0, got %f", *loaded.Webhooks.Thresholds.MaxPing)
	}
	if loaded.Webhooks.Thresholds.MaxJitter != nil {
		t.Error("expected nil MaxJitter after round-trip (not set)")
	}
}
