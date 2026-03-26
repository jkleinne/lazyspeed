package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/showwin/speedtest-go/speedtest"
)

func TestGetVersionInfo(t *testing.T) {
	// Reset package vars after test
	origVersion, origCommit, origDate := version, commit, date
	defer func() {
		version, commit, date = origVersion, origCommit, origDate
	}()

	// Case 1: All set
	version = "1.0.0"
	commit = "abcdef"
	date = "2023-01-01"
	res := GetVersionInfo()
	if !strings.Contains(res, "1.0.0") || !strings.Contains(res, "2023-01-01") {
		t.Errorf("Expected full version info, got %q", res)
	}

	// Case 2: Only version set
	version = "2.0.0"
	commit = ""
	date = ""
	res = GetVersionInfo()
	if !strings.Contains(res, "2.0.0") || strings.Contains(res, "built:") {
		t.Errorf("Expected simple version info, got %q", res)
	}
}

func TestUpdateServerListMsg(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:   m,
		spinner: ui.DefaultSpinner,
	}

	// Simulate successful server fetch
	s.model.State = model.StateAwaitingServers

	newModel, cmd := s.Update(serverListMsg{err: nil})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateSelectingServer {
		t.Errorf("Expected State to be StateSelectingServer, got %d", newS.model.State)
	}
	if cmd != nil {
		t.Errorf("Expected nil command")
	}

	// Simulate failed server fetch
	s.model.State = model.StateAwaitingServers
	newModel, _ = s.Update(serverListMsg{err: errors.New("fetch failed")})
	newS = newModel.(*speedTest)

	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle after error, got %d", newS.model.State)
	}
	if newS.model.Error == nil || newS.model.Error.Error() != "fetch failed" {
		t.Errorf("Expected error to be set")
	}
}

func TestUpdateKeyMsgQuit(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m}

	// "q" to quit
	newModel, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	newS := newModel.(*speedTest)

	if !newS.quitting {
		t.Errorf("Expected quitting to be true")
	}
	if cmd == nil {
		t.Errorf("Expected quit command")
	}
}

func TestUpdateKeyMsgNavigation(t *testing.T) {
	m := model.NewDefaultModel()
	m.ServerList = speedtest.Servers{
		&speedtest.Server{Name: "Server 1"},
		&speedtest.Server{Name: "Server 2"},
		&speedtest.Server{Name: "Server 3"},
	}
	m.State = model.StateSelectingServer
	s := speedTest{model: m}

	// Initial cursor is 0. Move down.
	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newS := newModel.(*speedTest)
	if newS.cursor != 1 {
		t.Errorf("Expected cursor to move to 1, got %d", newS.cursor)
	}

	// Move up
	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newS = newModel.(*speedTest)
	if newS.cursor != 0 {
		t.Errorf("Expected cursor to move back to 0, got %d", newS.cursor)
	}

	// Move up at top boundary
	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newS = newModel.(*speedTest)
	if newS.cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", newS.cursor)
	}

	// Back to home
	newModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newS = newModel.(*speedTest)
	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle, got %d", newS.model.State)
	}
	if !newS.showHelp {
		t.Errorf("Expected ShowHelp to be true")
	}
}

func TestView(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:   m,
		spinner: ui.DefaultSpinner,
	}

	// Fetching
	s.model.State = model.StateAwaitingServers
	s.model.CurrentPhase = "Fetching..."
	view := s.View()
	if !strings.Contains(view, "Fetching...") {
		t.Errorf("Expected fetching view")
	}

	// Selecting
	s.model.State = model.StateSelectingServer
	s.model.ServerList = speedtest.Servers{
		&speedtest.Server{Name: "Server 1", Latency: 10 * time.Millisecond},
	}
	view = s.View()
	if !strings.Contains(view, "Select a server:") {
		t.Errorf("Expected server selection view")
	}

	// Testing
	s.model.State = model.StateTesting
	s.model.CurrentPhase = "Ping test..."
	view = s.View()
	if !strings.Contains(view, "Ping test...") {
		t.Errorf("Expected testing view")
	}
}

func TestUpdateExportKeyOpensPrompt(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateExporting {
		t.Errorf("Expected State to be StateExporting after pressing e, got %d", newS.model.State)
	}
}

func TestUpdateExportKeyNoOpWithoutResult(t *testing.T) {
	m := model.NewDefaultModel()
	// m.Results is nil
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to remain StateIdle when there is no result, got %d", newS.model.State)
	}
}

func TestUpdateExportEscCancels(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.State = model.StateExporting
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle after Esc, got %d", newS.model.State)
	}
}

func TestUpdateExportDoneMsgSuccess(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(exportDoneMsg{path: "/tmp/lazyspeed_test.json"})
	newS := newModel.(*speedTest)

	if !strings.Contains(newS.model.ExportMessage, "/tmp/lazyspeed_test.json") {
		t.Errorf("Expected export path in ExportMessage, got %q", newS.model.ExportMessage)
	}
}

func TestUpdateExportDoneMsgError(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(exportDoneMsg{err: errors.New("write failed")})
	newS := newModel.(*speedTest)

	if !strings.Contains(newS.model.ExportMessage, "write failed") {
		t.Errorf("Expected error text in ExportMessage, got %q", newS.model.ExportMessage)
	}
}

func TestViewExportPrompt(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.TestHistory = []*model.SpeedTestResult{m.Results}
	m.State = model.StateExporting
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	view := s.View()
	if !strings.Contains(view, "[j] JSON") {
		t.Errorf("Expected export prompt in view when State is StateExporting")
	}
}

func TestViewExportMessage(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.TestHistory = []*model.SpeedTestResult{m.Results}
	m.ExportMessage = "Saved to /tmp/lazyspeed_result.json"
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	view := s.View()
	if !strings.Contains(view, "Saved to") {
		t.Errorf("Expected export message in view when ExportMessage is set")
	}
}

func TestUpdateHelpToggle(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m, spinner: ui.DefaultSpinner, showHelp: true}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	newS := newModel.(*speedTest)
	if newS.showHelp {
		t.Errorf("Expected ShowHelp to be false after first toggle")
	}

	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	newS = newModel.(*speedTest)
	if !newS.showHelp {
		t.Errorf("Expected ShowHelp to be true after second toggle")
	}
}

func TestUpdateNewTestKey(t *testing.T) {
	tests := []struct {
		name  string
		setup func(s *speedTest)
		check func(t *testing.T, s *speedTest)
	}{
		{
			name: "Opens server selection",
			setup: func(s *speedTest) {
				s.model.State = model.StateIdle
				s.showHelp = true
				s.model.ServerList = speedtest.Servers{
					&speedtest.Server{Name: "Server 1"},
				}
			},
			check: func(t *testing.T, s *speedTest) {
				if s.model.State != model.StateSelectingServer {
					t.Errorf("Expected State to be StateSelectingServer, got %d", s.model.State)
				}
				if s.showHelp {
					t.Errorf("Expected ShowHelp to be false")
				}
			},
		},
		{
			name: "No-op during testing",
			setup: func(s *speedTest) {
				s.model.State = model.StateTesting
			},
			check: func(t *testing.T, s *speedTest) {
				if s.model.State != model.StateTesting {
					t.Errorf("Expected State to remain StateTesting, got %d", s.model.State)
				}
			},
		},
		{
			name: "No-op during server selection",
			setup: func(s *speedTest) {
				s.model.State = model.StateSelectingServer
			},
			check: func(t *testing.T, s *speedTest) {
				if s.model.State != model.StateSelectingServer {
					t.Errorf("Expected State to remain StateSelectingServer, got %d", s.model.State)
				}
			},
		},
		{
			name: "Pending when servers loading",
			setup: func(s *speedTest) {
				s.model.State = model.StateIdle
				s.model.ServerList = nil
			},
			check: func(t *testing.T, s *speedTest) {
				if s.model.State != model.StateAwaitingServers {
					t.Errorf("Expected State to be StateAwaitingServers, got %d", s.model.State)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model.NewDefaultModel()
			s := speedTest{model: m, spinner: ui.DefaultSpinner}
			tt.setup(&s)

			newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
			newS := newModel.(*speedTest)
			tt.check(t, newS)
		})
	}
}

func TestUpdateProgressMsg(t *testing.T) {
	m := model.NewDefaultModel()
	m.State = model.StateTesting
	s := speedTest{
		model:        m,
		spinner:      ui.DefaultSpinner,
		progressChan: make(chan model.ProgressUpdate, 10),
		errChan:      make(chan error, 1),
	}

	newModel, cmd := s.Update(progressMsg{Progress: 0.5, Phase: "Downloading..."})
	newS := newModel.(*speedTest)

	if newS.model.Progress != 0.5 {
		t.Errorf("Expected Progress 0.5, got %f", newS.model.Progress)
	}
	if newS.model.CurrentPhase != "Downloading..." {
		t.Errorf("Expected Phase 'Downloading...', got %s", newS.model.CurrentPhase)
	}
	if cmd == nil {
		t.Errorf("Expected non-nil cmd to continue listening")
	}
}

func TestUpdateTestComplete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.State = model.StateTesting
		s := speedTest{model: m, spinner: ui.DefaultSpinner}

		newModel, _ := s.Update(testComplete{err: nil})
		newS := newModel.(*speedTest)

		if newS.model.State != model.StateIdle {
			t.Errorf("Expected State to be StateIdle, got %d", newS.model.State)
		}
		if newS.cancelTest != nil {
			t.Errorf("Expected cancelTest to be nil")
		}
	})

	t.Run("Error", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.State = model.StateTesting
		m.Results = &model.SpeedTestResult{DownloadSpeed: 100}
		s := speedTest{model: m, spinner: ui.DefaultSpinner}

		newModel, _ := s.Update(testComplete{err: errors.New("failed")})
		newS := newModel.(*speedTest)

		if newS.model.State != model.StateIdle {
			t.Errorf("Expected State to be StateIdle, got %d", newS.model.State)
		}
		if newS.model.Error == nil {
			t.Errorf("Expected Error to be set")
		}
		if newS.model.Results != nil {
			t.Errorf("Expected Results to be nil after error")
		}
	})
}

func TestUpdateWindowSizeMsg(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	newS := newModel.(*speedTest)

	if newS.model.Width != 120 {
		t.Errorf("Expected Width 120, got %d", newS.model.Width)
	}
	if newS.model.Height != 40 {
		t.Errorf("Expected Height 40, got %d", newS.model.Height)
	}
}

func TestWaitForProgress(t *testing.T) {
	t.Run("Progress received", func(t *testing.T) {
		progressChan := make(chan model.ProgressUpdate, 1)
		errChan := make(chan error, 1)

		progressChan <- model.ProgressUpdate{Progress: 0.5, Phase: "Downloading..."}

		cmd := waitForProgress(progressChan, errChan)
		msg := cmd()

		pm, ok := msg.(progressMsg)
		if !ok {
			t.Fatalf("Expected progressMsg, got %T", msg)
		}
		if pm.Progress != 0.5 {
			t.Errorf("Expected Progress 0.5, got %f", pm.Progress)
		}
		if pm.Phase != "Downloading..." {
			t.Errorf("Expected Phase 'Downloading...', got %s", pm.Phase)
		}
	})

	t.Run("Test complete no error", func(t *testing.T) {
		progressChan := make(chan model.ProgressUpdate)
		errChan := make(chan error, 1)

		close(progressChan)
		errChan <- nil

		cmd := waitForProgress(progressChan, errChan)
		msg := cmd()

		tc, ok := msg.(testComplete)
		if !ok {
			t.Fatalf("Expected testComplete, got %T", msg)
		}
		if tc.err != nil {
			t.Errorf("Expected nil error, got %v", tc.err)
		}
	})

	t.Run("Test complete with error", func(t *testing.T) {
		progressChan := make(chan model.ProgressUpdate)
		errChan := make(chan error, 1)

		close(progressChan)
		errChan <- errors.New("test failed")

		cmd := waitForProgress(progressChan, errChan)
		msg := cmd()

		tc, ok := msg.(testComplete)
		if !ok {
			t.Fatalf("Expected testComplete, got %T", msg)
		}
		if tc.err == nil || tc.err.Error() != "test failed" {
			t.Errorf("Expected 'test failed' error, got %v", tc.err)
		}
	})
}

func TestExportCmd(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Could not get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Could not chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	result := &model.SpeedTestResult{
		DownloadSpeed: 100.0,
		UploadSpeed:   50.0,
		Ping:          10.0,
		Timestamp:     time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
	}

	m := model.NewModel(nil, model.DefaultConfig())
	cmd := exportCmd(result, "json", m)
	msg := cmd()

	dm, ok := msg.(exportDoneMsg)
	if !ok {
		t.Fatalf("Expected exportDoneMsg, got %T", msg)
	}
	if dm.err != nil {
		t.Errorf("Expected nil error, got %v", dm.err)
	}
	if dm.path == "" {
		t.Errorf("Expected non-empty path")
	}
}

func TestExportCmdUsesConfigDirectory(t *testing.T) {
	exportDir := t.TempDir()
	cfg := model.DefaultConfig()
	cfg.Export.Directory = exportDir
	m := model.NewModel(nil, cfg)

	result := &model.SpeedTestResult{
		DownloadSpeed: 100.0,
		UploadSpeed:   50.0,
		Ping:          10.0,
		Timestamp:     time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
	}

	cmd := exportCmd(result, "json", m)
	msg := cmd()

	dm, ok := msg.(exportDoneMsg)
	if !ok {
		t.Fatalf("Expected exportDoneMsg, got %T", msg)
	}
	if dm.err != nil {
		t.Fatalf("Expected nil error, got %v", dm.err)
	}
	if !strings.HasPrefix(dm.path, exportDir) {
		t.Errorf("Expected path to start with %q, got %q", exportDir, dm.path)
	}
}

func TestMigrateHistoryIfNeeded(t *testing.T) {
	t.Run("No legacy file", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		migrateHistoryIfNeeded()
	})

	t.Run("Legacy exists new does not", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		legacyPath := filepath.Join(tmpDir, ".lazyspeed_history.json")
		content := []byte(`[{"download_speed": 100}]`)
		if err := os.WriteFile(legacyPath, content, 0600); err != nil {
			t.Fatalf("Could not write legacy file: %v", err)
		}

		migrateHistoryIfNeeded()

		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Errorf("Expected legacy file to be removed")
		}

		newPath := filepath.Join(tmpDir, ".local", "share", "lazyspeed", "history.json")
		data, err := os.ReadFile(newPath)
		if err != nil {
			t.Fatalf("Expected new history file to exist: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("Expected same content, got %s", string(data))
		}
	})

	t.Run("Legacy exists new already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		legacyPath := filepath.Join(tmpDir, ".lazyspeed_history.json")
		if err := os.WriteFile(legacyPath, []byte(`[{"download_speed": 100}]`), 0600); err != nil {
			t.Fatalf("Could not write legacy file: %v", err)
		}

		newDir := filepath.Join(tmpDir, ".local", "share", "lazyspeed")
		if err := os.MkdirAll(newDir, 0700); err != nil {
			t.Fatalf("Could not create new dir: %v", err)
		}
		newPath := filepath.Join(newDir, "history.json")
		if err := os.WriteFile(newPath, []byte(`[{"download_speed": 200}]`), 0600); err != nil {
			t.Fatalf("Could not write new file: %v", err)
		}

		migrateHistoryIfNeeded()

		if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
			t.Errorf("Expected legacy file to remain when new file already exists")
		}
	})
}

func TestCancelTestIfRunning(t *testing.T) {
	s := speedTest{model: model.NewDefaultModel()}
	s.cancelTestIfRunning() // nil cancelTest — should not panic

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelTest = cancel
	s.cancelTestIfRunning()

	if s.cancelTest != nil {
		t.Errorf("Expected cancelTest to be nil after call")
	}
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context to be cancelled")
	}
}

func TestUpdateExportJKey(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.State = model.StateExporting
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle after pressing j, got %d", newS.model.State)
	}
	if cmd == nil {
		t.Errorf("Expected non-nil cmd for JSON export")
	}
}

func TestUpdateExportCKey(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.State = model.StateExporting
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle after pressing c, got %d", newS.model.State)
	}
	if cmd == nil {
		t.Errorf("Expected non-nil cmd for CSV export")
	}
}

func TestViewWarning(t *testing.T) {
	m := model.NewDefaultModel()
	m.Warning = "some warning"
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.TestHistory = []*model.SpeedTestResult{m.Results}
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	view := s.View()
	if !strings.Contains(view, "Warning") {
		t.Errorf("Expected view to contain 'Warning'")
	}
}

func TestViewError(t *testing.T) {
	m := model.NewDefaultModel()
	m.Error = errors.New("broke")
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	view := s.View()
	if !strings.Contains(view, "Error") {
		t.Errorf("Expected view to contain 'Error'")
	}
}

func TestUpdateKeyMsgNavigationDownBoundary(t *testing.T) {
	m := model.NewDefaultModel()
	m.ServerList = speedtest.Servers{
		&speedtest.Server{Name: "Server 1"},
		&speedtest.Server{Name: "Server 2"},
		&speedtest.Server{Name: "Server 3"},
	}
	m.State = model.StateSelectingServer
	s := speedTest{model: m, spinner: ui.DefaultSpinner, cursor: 2}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newS := newModel.(*speedTest)
	if newS.cursor != 2 {
		t.Errorf("Expected cursor to stay at 2 (last position), got %d", newS.cursor)
	}
}

func TestUpdateEnterOnEmptyServerList(t *testing.T) {
	m := model.NewDefaultModel()
	m.State = model.StateSelectingServer
	m.ServerList = speedtest.Servers{}
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newS := newModel.(*speedTest)

	if newS.model.Error == nil {
		t.Fatalf("Expected error for invalid server selection, got nil")
	}
	if !strings.Contains(newS.model.Error.Error(), "invalid server selection") {
		t.Errorf("Expected 'invalid server selection' error, got %q", newS.model.Error.Error())
	}
	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle, got %d", newS.model.State)
	}
	if newS.showHelp {
		t.Errorf("Expected ShowHelp to be false")
	}
}

func TestViewResultsDisplay(t *testing.T) {
	t.Run("With results", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.Results = &model.SpeedTestResult{
			DownloadSpeed: 95.50,
			UploadSpeed:   45.00,
			Ping:          10.0,
			ServerName:    "Test Server",
			Timestamp:     time.Now(),
		}
		m.TestHistory = []*model.SpeedTestResult{m.Results}
		s := speedTest{model: m, spinner: ui.DefaultSpinner}

		view := s.View()
		if !strings.Contains(view, "Latest Test Results:") {
			t.Errorf("Expected 'Latest Test Results:' in view")
		}
		if !strings.Contains(view, "95.50") {
			t.Errorf("Expected '95.50' in view")
		}
	})

	t.Run("With help visible", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.Results = &model.SpeedTestResult{
			DownloadSpeed: 95.50,
			UploadSpeed:   45.00,
			Ping:          10.0,
			ServerName:    "Test Server",
			Timestamp:     time.Now(),
		}
		m.TestHistory = []*model.SpeedTestResult{m.Results}
		s := speedTest{model: m, spinner: ui.DefaultSpinner, showHelp: true}

		view := s.View()
		if !strings.Contains(view, "Latest Test Results:") {
			t.Errorf("Expected 'Latest Test Results:' in view")
		}
		if !strings.Contains(view, "Controls:") {
			t.Errorf("Expected 'Controls:' in view when ShowHelp is true")
		}
	})
}

func TestInitMethod(t *testing.T) {
	t.Run("Empty history", func(t *testing.T) {
		m := model.NewDefaultModel()
		s := speedTest{model: m, spinner: ui.DefaultSpinner}

		cmd := s.Init()

		if s.model.State != model.StateAwaitingServers {
			t.Errorf("Expected State to be StateAwaitingServers, got %d", s.model.State)
		}
		if s.model.CurrentPhase != "Fetching server list..." {
			t.Errorf("Expected CurrentPhase 'Fetching server list...', got %s", s.model.CurrentPhase)
		}
		if cmd == nil {
			t.Errorf("Expected non-nil cmd")
		}
	})

	t.Run("Has history", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.TestHistory = []*model.SpeedTestResult{{DownloadSpeed: 100}}
		s := speedTest{model: m, spinner: ui.DefaultSpinner}

		cmd := s.Init()

		if s.model.State != model.StateIdle {
			t.Errorf("Expected State to be StateIdle when history exists, got %d", s.model.State)
		}
		if cmd == nil {
			t.Errorf("Expected non-nil cmd")
		}
	})
}

func TestNewTestKeyResetsCursorAndOffset(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100.0}
	m.ServerList = make(speedtest.Servers, 10)
	for i := range m.ServerList {
		m.ServerList[i] = &speedtest.Server{Name: "S"}
	}
	s := speedTest{model: m, spinner: ui.DefaultSpinner, cursor: 5, serverListOffset: 3}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateSelectingServer {
		t.Errorf("Expected State to be StateSelectingServer, got %d", newS.model.State)
	}
	if newS.cursor != 0 {
		t.Errorf("Expected Cursor reset to 0, got %d", newS.cursor)
	}
	if newS.serverListOffset != 0 {
		t.Errorf("Expected ServerListOffset reset to 0, got %d", newS.serverListOffset)
	}
}

func TestServerListMsgResetsCursorAndOffset(t *testing.T) {
	m := model.NewDefaultModel()
	m.State = model.StateAwaitingServers
	s := speedTest{model: m, spinner: ui.DefaultSpinner, serverListOffset: 3}

	newModel, _ := s.Update(serverListMsg{err: nil})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateSelectingServer {
		t.Errorf("Expected State to be StateSelectingServer, got %d", newS.model.State)
	}
	if newS.cursor != 0 {
		t.Errorf("Expected Cursor reset to 0, got %d", newS.cursor)
	}
	if newS.serverListOffset != 0 {
		t.Errorf("Expected ServerListOffset reset to 0, got %d", newS.serverListOffset)
	}
}

func TestAdjustServerListOffset(t *testing.T) {
	tests := []struct {
		name           string
		cursor         int
		offset         int
		height         int
		serverCount    int
		expectedOffset int
	}{
		{
			name:           "Cursor visible, no adjustment",
			cursor:         3,
			offset:         0,
			height:         20,
			serverCount:    30,
			expectedOffset: 0,
		},
		{
			name:           "Cursor past bottom scrolls down",
			cursor:         15,
			offset:         0,
			height:         20,
			serverCount:    30,
			expectedOffset: 4,
		},
		{
			name:           "Cursor above top scrolls up",
			cursor:         2,
			offset:         5,
			height:         20,
			serverCount:    30,
			expectedOffset: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model.NewDefaultModel()
			m.Height = tt.height
			m.ServerList = make(speedtest.Servers, tt.serverCount)
			for i := range m.ServerList {
				m.ServerList[i] = &speedtest.Server{Name: "S"}
			}

			s := speedTest{model: m, spinner: ui.DefaultSpinner, cursor: tt.cursor, serverListOffset: tt.offset}
			s.adjustServerListOffset()

			if s.serverListOffset != tt.expectedOffset {
				t.Errorf("Expected offset %d, got %d", tt.expectedOffset, s.serverListOffset)
			}
		})
	}
}

func TestServerSelectionViewportNavigation(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 15
	m.State = model.StateSelectingServer
	m.ServerList = make(speedtest.Servers, 30)
	for i := range m.ServerList {
		m.ServerList[i] = &speedtest.Server{Name: "S"}
	}
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	// Move cursor down past visible area
	for i := 0; i < 10; i++ {
		newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		s = *newModel.(*speedTest)
	}

	if s.cursor != 10 {
		t.Errorf("Expected cursor at 10, got %d", s.cursor)
	}
	if s.serverListOffset == 0 {
		t.Errorf("Expected ServerListOffset to have scrolled from 0")
	}

	// Move cursor back to top
	for i := 0; i < 10; i++ {
		newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		s = *newModel.(*speedTest)
	}

	if s.cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", s.cursor)
	}
	if s.serverListOffset != 0 {
		t.Errorf("Expected ServerListOffset back at 0, got %d", s.serverListOffset)
	}
}

func TestHistoryScrollKeys(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 30
	m.TestHistory = make([]*model.SpeedTestResult, 20)
	for i := range m.TestHistory {
		m.TestHistory[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			Timestamp:     time.Now(),
		}
	}
	m.Results = m.TestHistory[len(m.TestHistory)-1]
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	// Scroll down
	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	s = *newModel.(*speedTest)
	if s.historyOffset != 1 {
		t.Errorf("Expected HistoryOffset 1 after j, got %d", s.historyOffset)
	}

	// Scroll back up
	newModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	s = *newModel.(*speedTest)
	if s.historyOffset != 0 {
		t.Errorf("Expected HistoryOffset 0 after k, got %d", s.historyOffset)
	}

	// Don't scroll past 0
	newModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	s = *newModel.(*speedTest)
	if s.historyOffset != 0 {
		t.Errorf("Expected HistoryOffset to stay at 0, got %d", s.historyOffset)
	}

	// Scroll down many times — should stop at max
	for i := 0; i < 50; i++ {
		newModel, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		s = *newModel.(*speedTest)
	}

	totalRows := len(m.TestHistory) - 1
	maxVisible := ui.HistoryVisibleRows(m.Height, totalRows)
	expectedMax := totalRows - maxVisible
	if s.historyOffset != expectedMax {
		t.Errorf("Expected HistoryOffset capped at %d, got %d", expectedMax, s.historyOffset)
	}
}

func TestHistoryOffsetResetOnTestComplete(t *testing.T) {
	m := model.NewDefaultModel()
	m.State = model.StateTesting
	s := speedTest{model: m, spinner: ui.DefaultSpinner, historyOffset: 5}

	newModel, _ := s.Update(testComplete{err: nil})
	newS := newModel.(*speedTest)

	if newS.historyOffset != 0 {
		t.Errorf("Expected HistoryOffset reset to 0 after testComplete, got %d", newS.historyOffset)
	}
	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to be StateIdle after testComplete, got %d", newS.model.State)
	}
}

func TestViewStateValues(t *testing.T) {
	var s ViewState
	if s != ViewMain {
		t.Errorf("Expected zero value to be ViewMain, got %d", s)
	}

	states := []ViewState{ViewMain, ViewDiagRunning, ViewDiagCompact, ViewDiagExpanded}
	seen := make(map[ViewState]bool)
	for _, st := range states {
		if seen[st] {
			t.Errorf("Duplicate ViewState value: %d", st)
		}
		seen[st] = true
	}
}

func TestUpdateDiagCompleteSuccess(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m, spinner: ui.DefaultSpinner, viewState: ViewDiagRunning}

	result := &diag.DiagResult{
		Target: "8.8.8.8",
		Hops:   []diag.Hop{{Number: 1, IP: "10.0.0.1"}},
	}

	newModel, _ := s.Update(diagCompleteMsg{result: result})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewDiagCompact {
		t.Errorf("Expected viewState ViewDiagCompact, got %d", newS.viewState)
	}
	if newS.diagResult == nil {
		t.Errorf("Expected diagResult to be set")
	}
	if newS.diagResult.Target != "8.8.8.8" {
		t.Errorf("Expected target 8.8.8.8, got %s", newS.diagResult.Target)
	}
}

func TestUpdateDiagCompleteError(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{model: m, spinner: ui.DefaultSpinner, viewState: ViewDiagRunning}

	newModel, _ := s.Update(diagCompleteMsg{err: errors.New("traceroute failed")})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewMain {
		t.Errorf("Expected viewState ViewMain after error, got %d", newS.viewState)
	}
	if newS.model.Error == nil || newS.model.Error.Error() != "traceroute failed" {
		t.Errorf("Expected error to be set")
	}
}

func TestDiagCompactEscReturnsToMain(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:      m,
		spinner:    ui.DefaultSpinner,
		viewState:  ViewDiagCompact,
		diagResult: &diag.DiagResult{Target: "8.8.8.8"},
	}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewMain {
		t.Errorf("Expected viewState ViewMain after Esc, got %d", newS.viewState)
	}
	if newS.diagResult != nil {
		t.Errorf("Expected diagResult to be cleared")
	}
	if !newS.showHelp {
		t.Errorf("Expected ShowHelp to be true")
	}
}

func TestDiagCompactEnterExpandsTrace(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:      m,
		spinner:    ui.DefaultSpinner,
		viewState:  ViewDiagCompact,
		diagResult: &diag.DiagResult{Target: "8.8.8.8"},
		diagOffset: 5,
	}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewDiagExpanded {
		t.Errorf("Expected viewState ViewDiagExpanded, got %d", newS.viewState)
	}
	if newS.diagOffset != 0 {
		t.Errorf("Expected diagOffset reset to 0, got %d", newS.diagOffset)
	}
}

func TestDiagCompactNewTestWithServers(t *testing.T) {
	m := model.NewDefaultModel()
	m.ServerList = speedtest.Servers{&speedtest.Server{Name: "S1"}}
	s := speedTest{
		model:      m,
		spinner:    ui.DefaultSpinner,
		viewState:  ViewDiagCompact,
		diagResult: &diag.DiagResult{Target: "8.8.8.8"},
	}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewMain {
		t.Errorf("Expected viewState ViewMain, got %d", newS.viewState)
	}
	if newS.model.State != model.StateSelectingServer {
		t.Errorf("Expected State StateSelectingServer, got %d", newS.model.State)
	}
	if newS.diagResult != nil {
		t.Errorf("Expected diagResult to be cleared")
	}
}

func TestDiagCompactNewTestWithoutServers(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:      m,
		spinner:    ui.DefaultSpinner,
		viewState:  ViewDiagCompact,
		diagResult: &diag.DiagResult{Target: "8.8.8.8"},
	}

	newModel, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewMain {
		t.Errorf("Expected viewState ViewMain, got %d", newS.viewState)
	}
	if newS.model.State != model.StateAwaitingServers {
		t.Errorf("Expected State StateAwaitingServers, got %d", newS.model.State)
	}
	if cmd == nil {
		t.Errorf("Expected non-nil cmd for spinner tick")
	}
}

func TestDiagExpandedEscCollapsesToCompact(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:      m,
		spinner:    ui.DefaultSpinner,
		viewState:  ViewDiagExpanded,
		diagResult: &diag.DiagResult{Target: "8.8.8.8"},
	}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newS := newModel.(*speedTest)

	if newS.viewState != ViewDiagCompact {
		t.Errorf("Expected viewState ViewDiagCompact after Esc, got %d", newS.viewState)
	}
}

func TestDiagExpandedScrollNavigation(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:     m,
		spinner:   ui.DefaultSpinner,
		viewState: ViewDiagExpanded,
		diagResult: &diag.DiagResult{
			Target: "8.8.8.8",
			Hops: []diag.Hop{
				{Number: 1, IP: "10.0.0.1"},
				{Number: 2, IP: "10.0.0.2"},
				{Number: 3, IP: "10.0.0.3"},
			},
		},
		diagOffset: 0,
	}

	// Scroll down
	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newS := newModel.(*speedTest)
	if newS.diagOffset != 1 {
		t.Errorf("Expected diagOffset 1 after j, got %d", newS.diagOffset)
	}

	// Scroll up
	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newS = newModel.(*speedTest)
	if newS.diagOffset != 0 {
		t.Errorf("Expected diagOffset 0 after k, got %d", newS.diagOffset)
	}

	// Don't scroll past 0
	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newS = newModel.(*speedTest)
	if newS.diagOffset != 0 {
		t.Errorf("Expected diagOffset to stay at 0, got %d", newS.diagOffset)
	}
}

func TestViewDiagStates(t *testing.T) {
	t.Run("DiagRunning", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.CurrentPhase = runningDiagnosticsPhase
		s := speedTest{model: m, spinner: ui.DefaultSpinner, viewState: ViewDiagRunning}

		view := s.View()
		if !strings.Contains(view, "Running diagnostics...") {
			t.Errorf("Expected diagnostics phase in view")
		}
	})

	t.Run("DiagCompact", func(t *testing.T) {
		m := model.NewDefaultModel()
		s := speedTest{
			model:     m,
			spinner:   ui.DefaultSpinner,
			viewState: ViewDiagCompact,
			diagResult: &diag.DiagResult{
				Target:  "example.com",
				Hops:    []diag.Hop{{Number: 1, IP: "10.0.0.1"}},
				Quality: diag.QualityScore{Score: 85, Grade: "B"},
			},
		}

		view := s.View()
		if !strings.Contains(view, "example.com") {
			t.Errorf("Expected target in compact view")
		}
	})

	t.Run("DiagExpanded", func(t *testing.T) {
		m := model.NewDefaultModel()
		m.Height = 40
		s := speedTest{
			model:     m,
			spinner:   ui.DefaultSpinner,
			viewState: ViewDiagExpanded,
			diagResult: &diag.DiagResult{
				Target:  "example.com",
				Hops:    []diag.Hop{{Number: 1, IP: "10.0.0.1"}},
				Quality: diag.QualityScore{Score: 85, Grade: "B"},
			},
		}

		view := s.View()
		if !strings.Contains(view, "10.0.0.1") {
			t.Errorf("Expected hop address in expanded view")
		}
	})
}

func TestServerListMsgErrorDuringIdleKeepsState(t *testing.T) {
	m := model.NewDefaultModel()
	m.TestHistory = []*model.SpeedTestResult{{DownloadSpeed: 100}}
	// State is StateIdle (background fetch)
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(serverListMsg{err: errors.New("network error")})
	newS := newModel.(*speedTest)

	if newS.model.State != model.StateIdle {
		t.Errorf("Expected State to remain StateIdle during background fetch error, got %d", newS.model.State)
	}
	if newS.model.Error == nil {
		t.Errorf("Expected error to be set")
	}
}
