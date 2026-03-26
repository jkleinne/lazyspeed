package model

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

// mockBackend implements Backend for testing
type mockBackend struct {
	fetchUserInfoFn func() (*speedtest.User, error)
	fetchServersFn  func() (speedtest.Servers, error)
	pingTestFn      func(server *speedtest.Server, fn func(time.Duration)) error
	downloadTestFn  func(server *speedtest.Server) error
	uploadTestFn    func(server *speedtest.Server) error
}

func (m *mockBackend) FetchUserInfo() (*speedtest.User, error) {
	if m.fetchUserInfoFn != nil {
		return m.fetchUserInfoFn()
	}
	return &speedtest.User{IP: "127.0.0.1", Isp: "Test ISP"}, nil
}

func (m *mockBackend) FetchServers() (speedtest.Servers, error) {
	if m.fetchServersFn != nil {
		return m.fetchServersFn()
	}
	return speedtest.Servers{}, nil
}

func (m *mockBackend) PingTest(server *speedtest.Server, fn func(time.Duration)) error {
	if m.pingTestFn != nil {
		return m.pingTestFn(server, fn)
	}
	return nil
}

func (m *mockBackend) DownloadTest(server *speedtest.Server) error {
	if m.downloadTestFn != nil {
		return m.downloadTestFn(server)
	}
	return nil
}

func (m *mockBackend) UploadTest(server *speedtest.Server) error {
	if m.uploadTestFn != nil {
		return m.uploadTestFn(server)
	}
	return nil
}

func TestSpeedTestResultJSONKeys(t *testing.T) {
	result := SpeedTestResult{
		ServerCountry: "Germany",
		Timestamp:     time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal SpeedTestResult: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal into map: %v", err)
	}

	if _, ok := raw["server_country"]; !ok {
		t.Errorf("Expected key 'server_country' in JSON output, got keys: %v", raw)
	}
	if _, ok := raw["server_loc"]; ok {
		t.Errorf("Unexpected legacy key 'server_loc' in JSON output")
	}
	if raw["server_country"] != "Germany" {
		t.Errorf("Expected server_country 'Germany', got %v", raw["server_country"])
	}
}

func TestUnmarshalJSONLegacyServerLoc(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		wantCountry string
	}{
		{
			name:        "Legacy server_loc only",
			jsonInput:   `{"server_loc":"Germany","download_speed":50}`,
			wantCountry: "Germany",
		},
		{
			name:        "Current server_country only",
			jsonInput:   `{"server_country":"France","download_speed":50}`,
			wantCountry: "France",
		},
		{
			name:        "Both keys — server_country wins",
			jsonInput:   `{"server_country":"France","server_loc":"Germany","download_speed":50}`,
			wantCountry: "France",
		},
		{
			name:        "Neither key",
			jsonInput:   `{"download_speed":50}`,
			wantCountry: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r SpeedTestResult
			if err := json.Unmarshal([]byte(tt.jsonInput), &r); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if r.ServerCountry != tt.wantCountry {
				t.Errorf("Expected ServerCountry %q, got %q", tt.wantCountry, r.ServerCountry)
			}
		})
	}
}

func TestLoadHistoryWithLegacyServerLoc(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write a history file with legacy "server_loc" entries
	historyDir := filepath.Join(tmpDir, ".local", "share", "lazyspeed")
	_ = os.MkdirAll(historyDir, 0700)
	legacyJSON := `[
		{"download_speed":100,"server_loc":"Germany","timestamp":"2026-03-19T00:00:00Z"},
		{"download_speed":200,"server_country":"France","timestamp":"2026-03-19T01:00:00Z"}
	]`
	_ = os.WriteFile(filepath.Join(historyDir, "history.json"), []byte(legacyJSON), 0600)

	m := NewModel(&mockBackend{}, nil)
	if err := m.LoadHistory(); err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}
	if len(m.TestHistory) != 2 {
		t.Fatalf("Expected 2 history entries, got %d", len(m.TestHistory))
	}
	if m.TestHistory[0].ServerCountry != "Germany" {
		t.Errorf("Expected legacy entry ServerCountry 'Germany', got %q", m.TestHistory[0].ServerCountry)
	}
	if m.TestHistory[1].ServerCountry != "France" {
		t.Errorf("Expected current entry ServerCountry 'France', got %q", m.TestHistory[1].ServerCountry)
	}
}

func TestNewModel(t *testing.T) {
	m := NewModel(&mockBackend{}, nil)

	if m.Results != nil {
		t.Errorf("Expected Results to be nil, got %v", m.Results)
	}

	if m.TestHistory == nil {
		t.Errorf("Expected TestHistory to not be nil")
	} else if len(m.TestHistory) != 0 {
		t.Errorf("Expected TestHistory to be empty, got length %d", len(m.TestHistory))
	}

	if m.State != StateIdle {
		t.Errorf("Expected State to be StateIdle, got %d", m.State)
	}

	if m.Progress != 0 {
		t.Errorf("Expected Progress to be 0, got %f", m.Progress)
	}

	if m.CurrentPhase != "" {
		t.Errorf("Expected CurrentPhase to be empty, got %s", m.CurrentPhase)
	}

}

func TestHistoryLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := NewModel(&mockBackend{}, nil)

	// Case 1: missing file (no error)
	err := m.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory on missing file failed: %v", err)
	}
	if len(m.TestHistory) != 0 {
		t.Errorf("Expected empty history, got %d", len(m.TestHistory))
	}

	// Case 2: Save empty history
	err = m.SaveHistory()
	if err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	// Case 3: Save and Load with data
	m.TestHistory = []*SpeedTestResult{
		{DownloadSpeed: 100},
		{DownloadSpeed: 200},
	}
	err = m.SaveHistory()
	if err != nil {
		t.Fatalf("SaveHistory with data failed: %v", err)
	}

	m2 := NewModel(&mockBackend{}, nil)
	err = m2.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory with data failed: %v", err)
	}
	if len(m2.TestHistory) != 2 {
		t.Errorf("Expected 2 history items, got %d", len(m2.TestHistory))
	}
	if m2.Results == nil || m2.Results.DownloadSpeed != 200 {
		t.Errorf("Expected Results to be last item (200), got %v", m2.Results)
	}

	// Case 4: Save > max size
	for i := 0; i < 60; i++ {
		m2.TestHistory = append(m2.TestHistory, &SpeedTestResult{DownloadSpeed: float64(i)})
	}
	err = m2.SaveHistory()
	if err != nil {
		t.Fatalf("SaveHistory > max size failed: %v", err)
	}

	m3 := NewModel(&mockBackend{}, nil)
	err = m3.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory > max size failed: %v", err)
	}
	if len(m3.TestHistory) != 50 {
		t.Errorf("Expected exactly 50 history items, got %d", len(m3.TestHistory))
	}

	// Case 5: Corrupt JSON — write corrupt data to the XDG path
	historyPath := filepath.Join(tmpDir, ".local", "share", "lazyspeed", "history.json")
	_ = os.WriteFile(historyPath, []byte("invalid json"), 0644)
	err = m3.LoadHistory()
	if err == nil {
		t.Errorf("Expected error loading corrupt JSON, got nil")
	}
}

func TestFetchServerList(t *testing.T) {
	// Case 1: Normal fetch (mocked) and sort
	m := NewModel(&mockBackend{
		fetchServersFn: func() (speedtest.Servers, error) {
			return speedtest.Servers{
				&speedtest.Server{Name: "Server C", Latency: 30 * time.Millisecond},
				&speedtest.Server{Name: "Server A", Latency: 10 * time.Millisecond},
				&speedtest.Server{Name: "Server B", Latency: 20 * time.Millisecond},
			}, nil
		},
	}, nil)

	err := m.FetchServerList(context.Background())
	if err != nil {
		t.Fatalf("FetchServerList failed: %v", err)
	}
	if len(m.ServerList) != 3 {
		t.Fatalf("Expected 3 servers, got %d", len(m.ServerList))
	}
	// Verify sorted by latency
	if m.ServerList[0].Name != "Server A" || m.ServerList[1].Name != "Server B" || m.ServerList[2].Name != "Server C" {
		t.Errorf("ServerList not sorted correctly by latency")
	}

	// Case 2: Error from backend
	m = NewModel(&mockBackend{
		fetchServersFn: func() (speedtest.Servers, error) {
			return nil, errors.New("backend error")
		},
	}, nil)
	err = m.FetchServerList(context.Background())
	if err == nil || err.Error() != "failed to fetch servers: backend error" {
		t.Errorf("Expected backend error, got %v", err)
	}

	// Case 3: Cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = m.FetchServerList(ctx)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestPerformSpeedTest(t *testing.T) {
	// To avoid saving history to user dir in test, override HOME
	t.Setenv("HOME", t.TempDir())

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			// Simulate a few successful pings
			fn(10 * time.Millisecond)
			fn(12 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit // 100 Mbps
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit // 50 Mbps
			return nil
		},
	}, nil)

	ctx := context.Background()
	updateChan := make(chan ProgressUpdate, 100) // Buffer so it doesn't block

	server := &speedtest.Server{
		Name:    "Test Server",
		Sponsor: "Test Sponsor",
		Country: "Test Country",
	}

	err := m.PerformSpeedTest(ctx, server, updateChan)
	if err != nil {
		t.Fatalf("PerformSpeedTest failed: %v", err)
	}

	if m.Results == nil {
		t.Fatalf("Expected Results to be populated")
	}
	if m.Results.DownloadSpeed != 100.0 {
		t.Errorf("Expected DL speed 100.0, got %f", m.Results.DownloadSpeed)
	}
	if m.Results.UploadSpeed != 50.0 {
		t.Errorf("Expected UL speed 50.0, got %f", m.Results.UploadSpeed)
	}
	if m.Results.ServerCountry != "Test Country" {
		t.Errorf("Expected ServerCountry 'Test Country', got %q", m.Results.ServerCountry)
	}
	if m.State != StateIdle {
		t.Errorf("Expected State to be StateIdle at end")
	}
}

func TestPerformSpeedTestFailures(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	server := &speedtest.Server{}

	// Case: All pings fail
	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, _ func(time.Duration)) error {
			return errors.New("ping failed")
		},
	}, nil)
	updateChan := make(chan ProgressUpdate, 100)
	_ = m.PerformSpeedTest(ctx, server, updateChan)
	if m.Results != nil && m.Results.Ping != 0.0 {
		t.Errorf("Expected avg ping to be 0.0 when all fail, got %f", m.Results.Ping)
	}

	// Case: Download fails
	m = NewModel(&mockBackend{
		downloadTestFn: func(_ *speedtest.Server) error {
			return errors.New("dl failed")
		},
	}, nil)
	err := m.PerformSpeedTest(ctx, server, updateChan)
	if err == nil || err.Error() != "download test failed: dl failed" {
		t.Errorf("Expected download error, got %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.History.MaxEntries != 50 {
		t.Errorf("Expected default max entries 50, got %d", cfg.History.MaxEntries)
	}
	if cfg.Test.PingCount != 10 {
		t.Errorf("Expected default ping count 10, got %d", cfg.Test.PingCount)
	}
	if cfg.History.Path == "" {
		t.Errorf("Expected non-empty default history path")
	}
	if cfg.Test.FetchTimeout != 30 {
		t.Errorf("Expected default fetch timeout 30, got %d", cfg.Test.FetchTimeout)
	}
	if cfg.Test.TestTimeout != 120 {
		t.Errorf("Expected default test timeout 120, got %d", cfg.Test.TestTimeout)
	}
	if cfg.Export.Directory != "" {
		t.Errorf("Expected empty default export directory, got %q", cfg.Export.Directory)
	}
}

func TestExportDir(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		wantCWD   bool
	}{
		{"empty config uses CWD", "", true},
		{"configured directory", "", false}, // uses tmpDir, set below
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			if !tt.wantCWD {
				cfg.Export.Directory = t.TempDir()
			}
			m := NewModel(nil, cfg)

			dir, err := m.ExportDir()
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
	m := NewModel(nil, cfg)

	dir, err := m.ExportDir()
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
	m := NewModel(nil, cfg)

	dir, err := m.ExportDir()
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
	m := NewModel(nil, cfg)

	dir, err := m.ExportDir()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if dir != fakeHome {
		t.Errorf("Expected %q, got %q", fakeHome, dir)
	}
}

func TestLoadConfigExportDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	configDir := filepath.Join(tmpDir, ".config", "lazyspeed")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Could not create config dir: %v", err)
	}
	configData := []byte("export:\n  directory: /tmp/my-exports\n")
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), configData, 0644); err != nil {
		t.Fatalf("Could not write config file: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cfg.Export.Directory != "/tmp/my-exports" {
		t.Errorf("Expected export directory '/tmp/my-exports', got %q", cfg.Export.Directory)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error on missing config, got %v", err)
	}
	if cfg.History.MaxEntries != 50 {
		t.Errorf("Expected default max entries, got %d", cfg.History.MaxEntries)
	}
}

func TestLoadConfigPartial(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Write a partial config
	configDir := filepath.Join(tmpDir, "lazyspeed")
	_ = os.MkdirAll(configDir, 0700)
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("test:\n  ping_count: 5\n"), 0600)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Test.PingCount != 5 {
		t.Errorf("Expected ping_count 5, got %d", cfg.Test.PingCount)
	}
	// Unspecified fields should retain defaults
	if cfg.History.MaxEntries != 50 {
		t.Errorf("Expected default max_entries 50, got %d", cfg.History.MaxEntries)
	}
}

func TestLoadConfigTimeouts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "lazyspeed")
	_ = os.MkdirAll(configDir, 0700)
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"),
		[]byte("test:\n  fetch_timeout: 15\n  test_timeout: 60\n"), 0600)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Test.FetchTimeout != 15 {
		t.Errorf("Expected fetch_timeout 15, got %d", cfg.Test.FetchTimeout)
	}
	if cfg.Test.TestTimeout != 60 {
		t.Errorf("Expected test_timeout 60, got %d", cfg.Test.TestTimeout)
	}
	if cfg.Test.PingCount != 10 {
		t.Errorf("Expected default ping_count 10, got %d", cfg.Test.PingCount)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "lazyspeed")
	_ = os.MkdirAll(configDir, 0700)
	_ = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("history:\n  max_entries: [unclosed\n"), 0600)

	_, err := LoadConfig()
	if err == nil {
		t.Errorf("Expected error on invalid YAML config, got nil")
	}
}

func TestConfigDrivenHistoryPath(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom_history.json")

	cfg := DefaultConfig()
	cfg.History.Path = customPath

	m := NewModel(&mockBackend{}, cfg)
	m.TestHistory = []*SpeedTestResult{{DownloadSpeed: 99}}

	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Errorf("Expected history file at custom path %s", customPath)
	}
}

func TestConfigDrivenPingCount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	pingCallCount := 0
	cfg := DefaultConfig()
	cfg.Test.PingCount = 3

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			pingCallCount++
			fn(10 * time.Millisecond)
			return nil
		},
	}, cfg)

	updateChan := make(chan ProgressUpdate, 100)
	server := &speedtest.Server{}
	_ = m.PerformSpeedTest(context.Background(), server, updateChan)

	if pingCallCount != 3 {
		t.Errorf("Expected 3 ping calls (from config), got %d", pingCallCount)
	}
}

func TestTimeoutDurations(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		m := NewModel(&mockBackend{}, nil)
		if m.FetchTimeoutDuration() != 30*time.Second {
			t.Errorf("Expected 30s fetch timeout, got %v", m.FetchTimeoutDuration())
		}
		if m.TestTimeoutDuration() != 120*time.Second {
			t.Errorf("Expected 120s test timeout, got %v", m.TestTimeoutDuration())
		}
	})

	t.Run("Custom config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Test.FetchTimeout = 10
		cfg.Test.TestTimeout = 60
		m := NewModel(&mockBackend{}, cfg)
		if m.FetchTimeoutDuration() != 10*time.Second {
			t.Errorf("Expected 10s fetch timeout, got %v", m.FetchTimeoutDuration())
		}
		if m.TestTimeoutDuration() != 60*time.Second {
			t.Errorf("Expected 60s test timeout, got %v", m.TestTimeoutDuration())
		}
	})
}

func TestExportResultJSON(t *testing.T) {
	dir := t.TempDir()
	result := &SpeedTestResult{
		DownloadSpeed: 99.5,
		UploadSpeed:   55.2,
		Ping:          12.0,
		Jitter:        1.5,
		ServerName:    "Test Server",
		ServerCountry: "US",
		UserIP:        "1.2.3.4",
		UserISP:       "TestISP",
		Timestamp:     time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
	}

	path, err := ExportResult(result, "json", dir)
	if err != nil {
		t.Fatalf("ExportResult JSON failed: %v", err)
	}
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("Expected .json suffix, got %q", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Could not read exported file: %v", err)
	}
	var got SpeedTestResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Could not parse exported JSON: %v", err)
	}
	if got.DownloadSpeed != result.DownloadSpeed {
		t.Errorf("Expected DownloadSpeed %.2f, got %.2f", result.DownloadSpeed, got.DownloadSpeed)
	}
	if got.ServerName != result.ServerName {
		t.Errorf("Expected ServerName %q, got %q", result.ServerName, got.ServerName)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Could not stat exported file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0644 {
		t.Errorf("Expected file permissions 0644, got %04o", perm)
	}
}

func TestExportResultCSV(t *testing.T) {
	dir := t.TempDir()
	result := &SpeedTestResult{
		DownloadSpeed: 88.0,
		UploadSpeed:   44.0,
		Ping:          8.0,
		Jitter:        0.5,
		ServerName:    "CSV Server",
		ServerCountry: "EU",
		UserIP:        "2.3.4.5",
		UserISP:       "EuroISP",
		Timestamp:     time.Date(2026, 3, 15, 11, 0, 0, 0, time.UTC),
	}

	path, err := ExportResult(result, "csv", dir)
	if err != nil {
		t.Fatalf("ExportResult CSV failed: %v", err)
	}
	if !strings.HasSuffix(path, ".csv") {
		t.Errorf("Expected .csv suffix, got %q", path)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Could not open exported file: %v", err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("Could not parse exported CSV: %v", err)
	}
	// Header + 1 data row
	if len(records) != 2 {
		t.Fatalf("Expected 2 CSV records, got %d", len(records))
	}
	if records[0][0] != "timestamp" {
		t.Errorf("Expected first header to be 'timestamp', got %q", records[0][0])
	}
	if records[1][1] != "CSV Server" {
		t.Errorf("Expected server name in CSV data row, got %q", records[1][1])
	}
	if records[1][2] != "EU" {
		t.Errorf("Expected country 'EU' in CSV data row, got %q", records[1][2])
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Could not stat exported CSV file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0644 {
		t.Errorf("Expected file permissions 0644, got %04o", perm)
	}
}

func TestExportResultUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	result := &SpeedTestResult{Timestamp: time.Now()}

	_, err := ExportResult(result, "xml", dir)
	if err == nil {
		t.Errorf("Expected error for unknown format, got nil")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("Expected error to mention the bad format, got %q", err.Error())
	}
}

func TestExportResultFilenameContainsTimestamp(t *testing.T) {
	dir := t.TempDir()
	ts := time.Date(2026, 3, 15, 12, 30, 45, 0, time.UTC)
	result := &SpeedTestResult{Timestamp: ts}

	path, err := ExportResult(result, "json", dir)
	if err != nil {
		t.Fatalf("ExportResult failed: %v", err)
	}
	base := filepath.Base(path)
	if !strings.Contains(base, "20260315_123045_000000000") {
		t.Errorf("Expected filename to contain timestamp '20260315_123045_000000000', got %q", base)
	}
}

func TestRunHeadless(t *testing.T) {
	tests := []struct {
		name         string
		opts         RunOptions
		pingCount    int
		setupBackend func(t *testing.T) *mockBackend
		wantErr      string
		checkResult  func(t *testing.T, res *SpeedTestResult)
	}{
		{
			name:      "Happy path",
			opts:      RunOptions{},
			pingCount: 2,
			setupBackend: func(_ *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(s *speedtest.Server) error {
						s.DLSpeed = 100 * bytesPerMbit
						return nil
					},
					uploadTestFn: func(s *speedtest.Server) error {
						s.ULSpeed = 50 * bytesPerMbit
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, res *SpeedTestResult) {
				if res.DownloadSpeed != 100.0 {
					t.Errorf("Expected DL 100.0, got %f", res.DownloadSpeed)
				}
				if res.UploadSpeed != 50.0 {
					t.Errorf("Expected UL 50.0, got %f", res.UploadSpeed)
				}
				if res.Ping == 0 {
					t.Errorf("Expected non-zero ping")
				}
				if res.UserIP != "127.0.0.1" {
					t.Errorf("Expected UserIP 127.0.0.1, got %s", res.UserIP)
				}
				if res.UserISP != "Test ISP" {
					t.Errorf("Expected UserISP Test ISP, got %s", res.UserISP)
				}
				if res.ServerCountry != "US" {
					t.Errorf("Expected ServerCountry 'US', got %q", res.ServerCountry)
				}
			},
		},
		{
			name:      "Skip download",
			opts:      RunOptions{SkipDownload: true},
			pingCount: 2,
			setupBackend: func(t *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(_ *speedtest.Server) error {
						t.Fatal("downloadTestFn should not be called when SkipDownload is true")
						return nil
					},
					uploadTestFn: func(s *speedtest.Server) error {
						s.ULSpeed = 50 * bytesPerMbit
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, res *SpeedTestResult) {
				if res.DownloadSpeed != 0 {
					t.Errorf("Expected DL 0, got %f", res.DownloadSpeed)
				}
				if res.UploadSpeed != 50.0 {
					t.Errorf("Expected UL 50.0, got %f", res.UploadSpeed)
				}
			},
		},
		{
			name:      "Skip upload",
			opts:      RunOptions{SkipUpload: true},
			pingCount: 2,
			setupBackend: func(t *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(s *speedtest.Server) error {
						s.DLSpeed = 100 * bytesPerMbit
						return nil
					},
					uploadTestFn: func(_ *speedtest.Server) error {
						t.Fatal("uploadTestFn should not be called when SkipUpload is true")
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, res *SpeedTestResult) {
				if res.DownloadSpeed != 100.0 {
					t.Errorf("Expected DL 100.0, got %f", res.DownloadSpeed)
				}
				if res.UploadSpeed != 0 {
					t.Errorf("Expected UL 0, got %f", res.UploadSpeed)
				}
			},
		},
		{
			name:      "Skip both",
			opts:      RunOptions{SkipDownload: true, SkipUpload: true},
			pingCount: 2,
			setupBackend: func(t *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(_ *speedtest.Server) error {
						t.Fatal("downloadTestFn should not be called")
						return nil
					},
					uploadTestFn: func(_ *speedtest.Server) error {
						t.Fatal("uploadTestFn should not be called")
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, res *SpeedTestResult) {
				if res.DownloadSpeed != 0 {
					t.Errorf("Expected DL 0, got %f", res.DownloadSpeed)
				}
				if res.UploadSpeed != 0 {
					t.Errorf("Expected UL 0, got %f", res.UploadSpeed)
				}
			},
		},
		{
			name:      "Download failure",
			opts:      RunOptions{},
			pingCount: 2,
			setupBackend: func(_ *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(_ *speedtest.Server) error {
						return errors.New("connection reset")
					},
				}
			},
			wantErr: "download test failed",
		},
		{
			name:      "Upload failure",
			opts:      RunOptions{},
			pingCount: 2,
			setupBackend: func(_ *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(s *speedtest.Server) error {
						s.DLSpeed = 100 * bytesPerMbit
						return nil
					},
					uploadTestFn: func(_ *speedtest.Server) error {
						return errors.New("upload timeout")
					},
				}
			},
			wantErr: "upload test failed",
		},
		{
			name:      "User info failure",
			opts:      RunOptions{},
			pingCount: 2,
			setupBackend: func(_ *testing.T) *mockBackend {
				return &mockBackend{
					fetchUserInfoFn: func() (*speedtest.User, error) {
						return nil, errors.New("network error")
					},
					pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
						fn(10 * time.Millisecond)
						return nil
					},
					downloadTestFn: func(s *speedtest.Server) error {
						s.DLSpeed = 100 * bytesPerMbit
						return nil
					},
					uploadTestFn: func(s *speedtest.Server) error {
						s.ULSpeed = 50 * bytesPerMbit
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, res *SpeedTestResult) {
				if res.UserIP != "" {
					t.Errorf("Expected empty UserIP, got %s", res.UserIP)
				}
				if res.UserISP != "" {
					t.Errorf("Expected empty UserISP, got %s", res.UserISP)
				}
			},
		},
		{
			name:      "All pings fail",
			opts:      RunOptions{},
			pingCount: 2,
			setupBackend: func(_ *testing.T) *mockBackend {
				return &mockBackend{
					pingTestFn: func(_ *speedtest.Server, _ func(time.Duration)) error {
						return errors.New("ping timeout")
					},
					downloadTestFn: func(s *speedtest.Server) error {
						s.DLSpeed = 100 * bytesPerMbit
						return nil
					},
					uploadTestFn: func(s *speedtest.Server) error {
						s.ULSpeed = 50 * bytesPerMbit
						return nil
					},
				}
			},
			checkResult: func(t *testing.T, res *SpeedTestResult) {
				if res.Ping != 0 {
					t.Errorf("Expected Ping 0, got %f", res.Ping)
				}
				if res.Jitter != 0 {
					t.Errorf("Expected Jitter 0, got %f", res.Jitter)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Test.PingCount = tt.pingCount

			backend := tt.setupBackend(t)
			m := NewModel(backend, cfg)
			server := &speedtest.Server{Name: "Test", Country: "US"}

			res, err := m.RunHeadless(context.Background(), server, tt.opts)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if res == nil {
				t.Fatalf("Expected non-nil result")
			}
			if tt.checkResult != nil {
				tt.checkResult(t, res)
			}
		})
	}

	// Config ping count — tested separately because it needs a call counter
	t.Run("Config ping count", func(t *testing.T) {
		pingCallCount := 0
		cfg := DefaultConfig()
		cfg.Test.PingCount = 3
		m := NewModel(&mockBackend{
			pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
				pingCallCount++
				fn(10 * time.Millisecond)
				return nil
			},
			downloadTestFn: func(s *speedtest.Server) error {
				s.DLSpeed = 100 * bytesPerMbit
				return nil
			},
			uploadTestFn: func(s *speedtest.Server) error {
				s.ULSpeed = 50 * bytesPerMbit
				return nil
			},
		}, cfg)

		_, err := m.RunHeadless(context.Background(), &speedtest.Server{}, RunOptions{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pingCallCount != 3 {
			t.Errorf("Expected 3 ping calls, got %d", pingCallCount)
		}
	})
}

func TestRunHeadlessContextCancellation(t *testing.T) {
	t.Run("Pre-cancelled context", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Test.PingCount = 2
		m := NewModel(&mockBackend{}, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := m.RunHeadless(ctx, &speedtest.Server{}, RunOptions{})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("Mid-test cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cfg := DefaultConfig()
		cfg.Test.PingCount = 2
		m := NewModel(&mockBackend{
			pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
				fn(10 * time.Millisecond)
				return nil
			},
			downloadTestFn: func(_ *speedtest.Server) error {
				cancel()
				return nil
			},
		}, cfg)

		_, err := m.RunHeadless(ctx, &speedtest.Server{}, RunOptions{})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

func TestRunHeadlessProgressCallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Test.PingCount = 2

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit
			return nil
		},
	}, cfg)

	var phases []string
	opts := RunOptions{
		ProgressFn: func(phase string) {
			phases = append(phases, phase)
		},
	}

	res, err := m.RunHeadless(context.Background(), &speedtest.Server{Name: "Test"}, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("Expected non-nil result")
	}
	if len(phases) == 0 {
		t.Errorf("Expected progress callbacks to be called")
	}

	found := map[string]bool{"ping": false, "download": false, "upload": false}
	for _, p := range phases {
		lower := strings.ToLower(p)
		if strings.Contains(lower, "ping") {
			found["ping"] = true
		}
		if strings.Contains(lower, "download") {
			found["download"] = true
		}
		if strings.Contains(lower, "upload") {
			found["upload"] = true
		}
	}
	for phase, seen := range found {
		if !seen {
			t.Errorf("Expected %s phase in progress callbacks", phase)
		}
	}
}

func TestRunHeadlessProgressCallbackNil(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Test.PingCount = 1
	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit
			return nil
		},
	}, cfg)

	res, err := m.RunHeadless(context.Background(), &speedtest.Server{}, RunOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("Expected non-nil result")
	}
}

func TestRunHeadlessProgressSkipPhases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Test.PingCount = 1
	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
	}, cfg)

	var phases []string
	opts := RunOptions{
		SkipDownload: true,
		SkipUpload:   true,
		ProgressFn: func(phase string) {
			phases = append(phases, phase)
		},
	}

	_, err := m.RunHeadless(context.Background(), &speedtest.Server{}, opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	for _, p := range phases {
		lower := strings.ToLower(p)
		if strings.Contains(lower, "download") {
			t.Errorf("Expected no download phase when SkipDownload, got %q", p)
		}
		if strings.Contains(lower, "upload") {
			t.Errorf("Expected no upload phase when SkipUpload, got %q", p)
		}
	}
}

func TestPerformSpeedTestUploadFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(_ *speedtest.Server) error {
			return errors.New("upload timeout")
		},
	}, nil)

	err := m.PerformSpeedTest(context.Background(), &speedtest.Server{}, make(chan ProgressUpdate, 100))
	if err == nil || !strings.Contains(err.Error(), "upload test failed") {
		t.Errorf("Expected error containing 'upload test failed', got %v", err)
	}
}

func TestPerformSpeedTestContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	cfg := DefaultConfig()
	cfg.Test.PingCount = 3

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			callCount++
			if callCount == 2 {
				cancel()
			}
			fn(10 * time.Millisecond)
			return nil
		},
	}, cfg)

	err := m.PerformSpeedTest(ctx, &speedtest.Server{}, make(chan ProgressUpdate, 100))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
	if m.State != StateIdle {
		t.Errorf("Expected State to be StateIdle after cancellation")
	}
}

func TestSendUpdate(t *testing.T) {
	t.Run("Nil channel", func(_ *testing.T) {
		sendUpdate(0.5, "test", nil)
	})

	t.Run("Buffered channel", func(t *testing.T) {
		ch := make(chan ProgressUpdate, 1)
		sendUpdate(0.5, "test phase", ch)

		update := <-ch
		if update.Progress != 0.5 {
			t.Errorf("Expected Progress 0.5, got %f", update.Progress)
		}
		if update.Phase != "test phase" {
			t.Errorf("Expected Phase 'test phase', got %s", update.Phase)
		}
	})
}

func TestPerformSpeedTestJitterCalculation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	latencies := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 15 * time.Millisecond}
	callIdx := 0

	cfg := DefaultConfig()
	cfg.Test.PingCount = 3

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(latencies[callIdx])
			callIdx++
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit
			return nil
		},
	}, cfg)

	err := m.PerformSpeedTest(context.Background(), &speedtest.Server{}, make(chan ProgressUpdate, 100))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if m.Results.Ping != 15.0 {
		t.Errorf("Expected avg ping 15.0, got %f", m.Results.Ping)
	}
	if m.Results.Jitter != 7.5 {
		t.Errorf("Expected jitter 7.5, got %f", m.Results.Jitter)
	}
}

func TestPerformSpeedTestUserInfoFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel(&mockBackend{
		fetchUserInfoFn: func() (*speedtest.User, error) {
			return nil, errors.New("network error")
		},
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit
			return nil
		},
	}, nil)

	err := m.PerformSpeedTest(context.Background(), &speedtest.Server{}, make(chan ProgressUpdate, 100))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if m.Warning == "" {
		t.Errorf("Expected non-empty warning")
	}
	if m.Results.UserIP != "" {
		t.Errorf("Expected empty UserIP, got %s", m.Results.UserIP)
	}
}

func TestPerformSpeedTestProgressChannel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultConfig()
	cfg.Test.PingCount = 1

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit
			return nil
		},
	}, cfg)

	updateChan := make(chan ProgressUpdate, 100)
	err := m.PerformSpeedTest(context.Background(), &speedtest.Server{}, updateChan)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var updates []ProgressUpdate
drain:
	for {
		select {
		case u := <-updateChan:
			updates = append(updates, u)
		default:
			break drain
		}
	}

	if len(updates) == 0 {
		t.Fatalf("Expected at least one progress update")
	}
	lastUpdate := updates[len(updates)-1]
	if lastUpdate.Phase != "Test completed" {
		t.Errorf("Expected final phase 'Test completed', got %s", lastUpdate.Phase)
	}
}

func TestFetchServerListEmptyResult(t *testing.T) {
	m := NewModel(&mockBackend{
		fetchServersFn: func() (speedtest.Servers, error) {
			return speedtest.Servers{}, nil
		},
	}, nil)

	err := m.FetchServerList(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(m.ServerList) != 0 {
		t.Errorf("Expected empty ServerList, got %d servers", len(m.ServerList))
	}
}

func TestPerformSpeedTestSinglePing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultConfig()
	cfg.Test.PingCount = 1

	m := NewModel(&mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
		downloadTestFn: func(s *speedtest.Server) error {
			s.DLSpeed = 100 * bytesPerMbit
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesPerMbit
			return nil
		},
	}, cfg)

	err := m.PerformSpeedTest(context.Background(), &speedtest.Server{}, make(chan ProgressUpdate, 100))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if m.Results.Ping != 10.0 {
		t.Errorf("Expected Ping 10.0, got %f", m.Results.Ping)
	}
	if m.Results.Jitter != 0.0 {
		t.Errorf("Expected Jitter 0.0 with single ping (MAD requires 2+ samples), got %f", m.Results.Jitter)
	}
}

// makeReadOnlyDir creates a read-only directory and returns its path.
// Skips the test if the directory is writable despite 0555 (e.g., running as root).
func makeReadOnlyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o755); err != nil {
		t.Fatalf("Could not create directory: %v", err)
	}
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("Could not set read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	testPath := filepath.Join(readOnlyDir, "test_write")
	if err := os.WriteFile(testPath, []byte("test"), 0644); err == nil {
		_ = os.Remove(testPath)
		t.Skip("Directory is writable despite 0555 permissions (running as root?)")
	}
	return readOnlyDir
}

func TestSaveHistoryUnwritableDirectory(t *testing.T) {
	readOnlyDir := makeReadOnlyDir(t)

	cfg := DefaultConfig()
	cfg.History.Path = filepath.Join(readOnlyDir, "history.json")

	m := NewModel(&mockBackend{}, cfg)
	m.TestHistory = []*SpeedTestResult{{DownloadSpeed: 100}}

	err := m.SaveHistory()
	if err == nil {
		t.Fatalf("Expected error writing to read-only directory, got nil")
	}
}

func TestExportResultUnwritableDirectory(t *testing.T) {
	readOnlyDir := makeReadOnlyDir(t)

	result := &SpeedTestResult{
		DownloadSpeed: 100,
		Timestamp:     time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
	}

	t.Run("JSON", func(t *testing.T) {
		_, err := ExportResult(result, "json", readOnlyDir)
		if err == nil {
			t.Errorf("Expected error writing JSON to read-only directory, got nil")
		}
	})

	t.Run("CSV", func(t *testing.T) {
		_, err := ExportResult(result, "csv", readOnlyDir)
		if err == nil {
			t.Errorf("Expected error writing CSV to read-only directory, got nil")
		}
	})
}

func TestDefaultPathsRespectXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")

	histPath := defaultHistoryPath()
	if histPath != "/tmp/xdg-data/lazyspeed/history.json" {
		t.Errorf("defaultHistoryPath() = %s, want /tmp/xdg-data/lazyspeed/history.json", histPath)
	}

	diagPath := defaultDiagnosticsPath()
	if diagPath != "/tmp/xdg-data/lazyspeed/diagnostics.json" {
		t.Errorf("defaultDiagnosticsPath() = %s, want /tmp/xdg-data/lazyspeed/diagnostics.json", diagPath)
	}
}

func TestLegacyHistoryPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/fakehome")

	path, err := LegacyHistoryPath()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if path != "/tmp/fakehome/.lazyspeed_history.json" {
		t.Errorf("Expected /tmp/fakehome/.lazyspeed_history.json, got %s", path)
	}
}

func TestDiagnosticsConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Diagnostics.MaxHops != 30 {
		t.Errorf("MaxHops = %d, want 30", cfg.Diagnostics.MaxHops)
	}
	if cfg.Diagnostics.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", cfg.Diagnostics.Timeout)
	}
	if cfg.Diagnostics.MaxEntries != 20 {
		t.Errorf("MaxEntries = %d, want 20", cfg.Diagnostics.MaxEntries)
	}
}

func TestDiagnosticsConfigOverlay(t *testing.T) {
	dir := t.TempDir()
	content := []byte("diagnostics:\n  max_hops: 15\n  timeout: 45\n")
	if err := os.MkdirAll(filepath.Join(dir, "lazyspeed"), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configDst := filepath.Join(dir, "lazyspeed", "config.yaml")
	if err := os.WriteFile(configDst, content, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Diagnostics.MaxHops != 15 {
		t.Errorf("MaxHops = %d, want 15", cfg.Diagnostics.MaxHops)
	}
	if cfg.Diagnostics.Timeout != 45 {
		t.Errorf("Timeout = %d, want 45", cfg.Diagnostics.Timeout)
	}
	if cfg.Diagnostics.MaxEntries != 20 {
		t.Errorf("MaxEntries = %d, want 20 (default)", cfg.Diagnostics.MaxEntries)
	}
}

func TestResultCSVRow(t *testing.T) {
	ts := time.Date(2026, 3, 15, 12, 30, 45, 0, time.UTC)
	res := &SpeedTestResult{
		Timestamp:     ts,
		ServerName:    "Test Server",
		ServerCountry: "US",
		DownloadSpeed: 95.50,
		UploadSpeed:   45.25,
		Ping:          12.34,
		Jitter:        1.56,
		UserIP:        "1.2.3.4",
		UserISP:       "Test ISP",
	}

	row := res.CSVRow()
	if len(row) != 9 {
		t.Fatalf("CSVRow() returned %d fields, want 9", len(row))
	}

	want := []string{
		"2026-03-15T12:30:45Z",
		"Test Server",
		"US",
		"95.50",
		"45.25",
		"12.34",
		"1.56",
		"1.2.3.4",
		"Test ISP",
	}
	for i, w := range want {
		if row[i] != w {
			t.Errorf("row[%d] = %q, want %q", i, row[i], w)
		}
	}
}

func TestResultCSVRowZeroValues(t *testing.T) {
	res := &SpeedTestResult{}
	row := res.CSVRow()
	if len(row) != 9 {
		t.Fatalf("CSVRow() returned %d fields, want 9", len(row))
	}
	// Numeric fields should format as "0.00"
	for _, idx := range []int{3, 4, 5, 6} {
		if row[idx] != "0.00" {
			t.Errorf("row[%d] = %q, want %q", idx, row[idx], "0.00")
		}
	}
	// String fields should be empty
	for _, idx := range []int{1, 2, 7, 8} {
		if row[idx] != "" {
			t.Errorf("row[%d] = %q, want empty", idx, row[idx])
		}
	}
}

func TestMeasurePing(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		12 * time.Millisecond,
		8 * time.Millisecond,
	}
	callIdx := 0
	backend := &mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			if callIdx < len(latencies) {
				fn(latencies[callIdx])
				callIdx++
			}
			return nil
		},
	}

	ctx := context.Background()
	server := &speedtest.Server{}
	result, err := measurePing(ctx, backend, server, 3, nil)
	if err != nil {
		t.Fatalf("measurePing failed: %v", err)
	}
	if len(result.pings) != 3 {
		t.Fatalf("got %d pings, want 3", len(result.pings))
	}
	// avg = (10+12+8)/3 = 10
	if result.avgPing != 10 {
		t.Errorf("AvgPing = %f, want 10", result.avgPing)
	}
	// jitter = mean of |12-10|, |8-12| = (2+4)/2 = 3
	if result.jitter != 3 {
		t.Errorf("Jitter = %f, want 3", result.jitter)
	}
}

func TestMeasurePingSinglePingZeroJitter(t *testing.T) {
	backend := &mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(15 * time.Millisecond)
			return nil
		},
	}
	ctx := context.Background()
	server := &speedtest.Server{}
	result, err := measurePing(ctx, backend, server, 1, nil)
	if err != nil {
		t.Fatalf("measurePing failed: %v", err)
	}
	if len(result.pings) != 1 {
		t.Fatalf("got %d pings, want 1", len(result.pings))
	}
	if result.avgPing != 15 {
		t.Errorf("AvgPing = %f, want 15", result.avgPing)
	}
	if result.jitter != 0 {
		t.Errorf("Jitter = %f, want 0 (single ping)", result.jitter)
	}
}

func TestMeasurePingContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	backend := &mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			fn(10 * time.Millisecond)
			return nil
		},
	}
	server := &speedtest.Server{}
	result, err := measurePing(ctx, backend, server, 5, nil)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on cancellation, got %+v", result)
	}
}

func TestMeasurePingNoPings(t *testing.T) {
	backend := &mockBackend{
		pingTestFn: func(_ *speedtest.Server, _ func(time.Duration)) error {
			return errors.New("network error")
		},
	}
	ctx := context.Background()
	server := &speedtest.Server{}
	result, err := measurePing(ctx, backend, server, 3, nil)
	if err != nil {
		t.Fatalf("measurePing should not error: %v", err)
	}
	if len(result.pings) != 0 {
		t.Errorf("got %d pings, want 0", len(result.pings))
	}
	if result.avgPing != 0 {
		t.Errorf("avgPing = %f, want 0", result.avgPing)
	}
	if result.jitter != 0 {
		t.Errorf("jitter = %f, want 0", result.jitter)
	}
}

func TestMeasurePingObserverCalled(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		14 * time.Millisecond,
	}
	callIdx := 0
	backend := &mockBackend{
		pingTestFn: func(_ *speedtest.Server, fn func(time.Duration)) error {
			if callIdx < len(latencies) {
				fn(latencies[callIdx])
				callIdx++
			}
			return nil
		},
	}

	type observation struct {
		iteration int
		total     int
		ping      float64
		jitter    float64
	}
	var observations []observation
	observer := func(i, total int, ping, jitter float64) {
		observations = append(observations, observation{i, total, ping, jitter})
	}

	ctx := context.Background()
	_, err := measurePing(ctx, backend, &speedtest.Server{}, 2, observer)
	if err != nil {
		t.Fatalf("measurePing failed: %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("observer called %d times, want 2", len(observations))
	}
	// First ping: iteration=1, jitter=0 (only one sample)
	if observations[0].iteration != 1 || observations[0].jitter != 0 {
		t.Errorf("obs[0] = %+v, want iteration=1 jitter=0", observations[0])
	}
	// Second ping: iteration=2, jitter=|14-10|=4
	if observations[1].iteration != 2 || observations[1].jitter != 4 {
		t.Errorf("obs[1] = %+v, want iteration=2 jitter=4", observations[1])
	}
}

func TestMonitorTransferProgress(t *testing.T) {
	updateChan := make(chan ProgressUpdate, 50)
	ctx := context.Background()

	done, doneAck := monitorTransferProgress(ctx, transferPhase{
		start:   0.5,
		span:    0.25,
		maxProg: 0.7,
		label:   "download",
		rateFn:  func() float64 { return 100 * bytesPerMbit }, // 100 Mbps in bytes/sec
	}, updateChan)

	// Let the ticker fire at least once (progressInterval = 200ms)
	time.Sleep(300 * time.Millisecond)
	close(done)
	<-doneAck

	// Drain updates
	close(updateChan)
	var updates []ProgressUpdate
	for u := range updateChan {
		updates = append(updates, u)
	}

	if len(updates) == 0 {
		t.Fatal("monitorTransferProgress sent no updates")
	}
	for _, u := range updates {
		if u.Progress < 0.5 || u.Progress > 0.7 {
			t.Errorf("progress %f out of [0.5, 0.7] bounds", u.Progress)
		}
		if !strings.Contains(u.Phase, "download") {
			t.Errorf("phase %q does not mention download", u.Phase)
		}
		if !strings.Contains(u.Phase, "100.00 Mbps") {
			t.Errorf("phase %q does not show expected rate", u.Phase)
		}
	}
}

func TestMonitorTransferProgressContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updateChan := make(chan ProgressUpdate, 50)

	done, doneAck := monitorTransferProgress(ctx, transferPhase{
		start:   0.0,
		span:    1.0,
		maxProg: 1.0,
		label:   "test",
		rateFn:  func() float64 { return 0 },
	}, updateChan)

	cancel()
	// doneAck should close promptly after context cancellation
	select {
	case <-doneAck:
		// goroutine exited cleanly
	case <-time.After(time.Second):
		t.Fatal("monitorTransferProgress did not stop after context cancellation")
	}
	_ = done // done channel is still open, but goroutine exited via ctx
}
