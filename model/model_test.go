package model

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestNewModel(t *testing.T) {
	m := NewModel(&mockBackend{})

	if m.Results != nil {
		t.Errorf("Expected Results to be nil, got %v", m.Results)
	}

	if m.TestHistory == nil {
		t.Errorf("Expected TestHistory to not be nil")
	} else if len(m.TestHistory) != 0 {
		t.Errorf("Expected TestHistory to be empty, got length %d", len(m.TestHistory))
	}

	if m.Testing != false {
		t.Errorf("Expected Testing to be false, got %t", m.Testing)
	}

	if m.Progress != 0 {
		t.Errorf("Expected Progress to be 0, got %f", m.Progress)
	}

	if m.CurrentPhase != "" {
		t.Errorf("Expected CurrentPhase to be empty, got %s", m.CurrentPhase)
	}

	if m.ShowHelp != true {
		t.Errorf("Expected ShowHelp to be true, got %t", m.ShowHelp)
	}

	if m.SelectingServer != false {
		t.Errorf("Expected SelectingServer to be false, got %t", m.SelectingServer)
	}

	if m.Cursor != 0 {
		t.Errorf("Expected Cursor to be 0, got %d", m.Cursor)
	}
}

func TestHistoryLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := NewModel(&mockBackend{})

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

	m2 := NewModel(&mockBackend{})
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

	m3 := NewModel(&mockBackend{})
	err = m3.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory > max size failed: %v", err)
	}
	if len(m3.TestHistory) != 50 {
		t.Errorf("Expected exactly 50 history items, got %d", len(m3.TestHistory))
	}

	// Case 5: Corrupt JSON
	historyPath := filepath.Join(tmpDir, ".lazyspeed_history.json")
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
	})

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
	})
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
			s.DLSpeed = 100 * bytesToMB // 100 MBps
			return nil
		},
		uploadTestFn: func(s *speedtest.Server) error {
			s.ULSpeed = 50 * bytesToMB // 50 MBps
			return nil
		},
	})

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
	if m.Testing != false {
		t.Errorf("Expected Testing to be false at end")
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
	})
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
	})
	err := m.PerformSpeedTest(ctx, server, updateChan)
	if err == nil || err.Error() != "download test failed: dl failed" {
		t.Errorf("Expected download error, got %v", err)
	}
}
