package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	s.model.FetchingServers = true
	s.model.PendingServerSelection = true

	newModel, cmd := s.Update(serverListMsg{err: nil})
	newS := newModel.(*speedTest)

	if newS.model.FetchingServers {
		t.Errorf("Expected FetchingServers to be false")
	}
	if !newS.model.SelectingServer {
		t.Errorf("Expected SelectingServer to be true")
	}
	if cmd != nil {
		t.Errorf("Expected nil command")
	}

	// Simulate failed server fetch
	s.model.FetchingServers = true
	s.model.PendingServerSelection = true
	s.model.SelectingServer = false
	newModel, _ = s.Update(serverListMsg{err: errors.New("fetch failed")})
	newS = newModel.(*speedTest)

	if newS.model.FetchingServers {
		t.Errorf("Expected FetchingServers to be false")
	}
	if newS.model.SelectingServer {
		t.Errorf("Expected SelectingServer to remain false")
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
	m.SelectingServer = true
	s := speedTest{model: m}

	// Initial cursor is 0. Move down.
	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	newS := newModel.(*speedTest)
	if newS.model.Cursor != 1 {
		t.Errorf("Expected cursor to move to 1, got %d", newS.model.Cursor)
	}

	// Move up
	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newS = newModel.(*speedTest)
	if newS.model.Cursor != 0 {
		t.Errorf("Expected cursor to move back to 0, got %d", newS.model.Cursor)
	}

	// Move up at top boundary
	newModel, _ = newS.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	newS = newModel.(*speedTest)
	if newS.model.Cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", newS.model.Cursor)
	}
}

func TestView(t *testing.T) {
	m := model.NewDefaultModel()
	s := speedTest{
		model:   m,
		spinner: ui.DefaultSpinner,
	}

	// Fetching
	s.model.FetchingServers = true
	s.model.PendingServerSelection = true
	s.model.CurrentPhase = "Fetching..."
	view := s.View()
	if !strings.Contains(view, "Fetching...") {
		t.Errorf("Expected fetching view")
	}

	// Selecting
	s.model.FetchingServers = false
	s.model.PendingServerSelection = false
	s.model.SelectingServer = true
	s.model.ServerList = speedtest.Servers{
		&speedtest.Server{Name: "Server 1", Latency: 10 * time.Millisecond},
	}
	view = s.View()
	if !strings.Contains(view, "Select a server:") {
		t.Errorf("Expected server selection view")
	}

	// Testing
	s.model.SelectingServer = false
	s.model.Testing = true
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

	if !newS.model.Exporting {
		t.Errorf("Expected Exporting to be true after pressing e")
	}
}

func TestUpdateExportKeyNoOpWithoutResult(t *testing.T) {
	m := model.NewDefaultModel()
	// m.Results is nil
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	newS := newModel.(*speedTest)

	if newS.model.Exporting {
		t.Errorf("Expected Exporting to remain false when there is no result")
	}
}

func TestUpdateExportEscCancels(t *testing.T) {
	m := model.NewDefaultModel()
	m.Results = &model.SpeedTestResult{DownloadSpeed: 100, Timestamp: time.Now()}
	m.Exporting = true
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	newModel, _ := s.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newS := newModel.(*speedTest)

	if newS.model.Exporting {
		t.Errorf("Expected Exporting to be false after Esc")
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
	m.Exporting = true
	s := speedTest{model: m, spinner: ui.DefaultSpinner}

	view := s.View()
	if !strings.Contains(view, "[j] JSON") {
		t.Errorf("Expected export prompt in view when Exporting is true")
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
