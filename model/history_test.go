package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryStore_Append(t *testing.T) {
	h := NewHistoryStore(HistoryConfig{})

	first := &SpeedTestResult{DownloadSpeed: 100}
	h.Append(first)

	if len(h.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(h.Entries))
	}
	if h.Results != first {
		t.Errorf("Expected Results to point to first entry")
	}

	second := &SpeedTestResult{DownloadSpeed: 200}
	h.Append(second)

	if len(h.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(h.Entries))
	}
	if h.Results != second {
		t.Errorf("Expected Results to point to latest entry")
	}
}

func TestHistoryStore_LoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	cfg := HistoryConfig{Path: path, MaxEntries: 50}

	h := NewHistoryStore(cfg)
	h.Append(&SpeedTestResult{DownloadSpeed: 42, ServerName: "TestServer"})
	h.Append(&SpeedTestResult{DownloadSpeed: 84, ServerName: "TestServer2"})

	if err := h.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	h2 := NewHistoryStore(cfg)
	if err := h2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(h2.Entries) != 2 {
		t.Fatalf("Expected 2 entries after reload, got %d", len(h2.Entries))
	}
	if h2.Entries[0].DownloadSpeed != 42 {
		t.Errorf("Expected first entry DownloadSpeed 42, got %v", h2.Entries[0].DownloadSpeed)
	}
	if h2.Results == nil || h2.Results.DownloadSpeed != 84 {
		t.Errorf("Expected Results to be last entry (84), got %v", h2.Results)
	}
}

func TestHistoryStore_LoadNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "history.json")
	h := NewHistoryStore(HistoryConfig{Path: path})

	if err := h.Load(); err != nil {
		t.Fatalf("Expected nil error for missing file, got %v", err)
	}
	if len(h.Entries) != 0 {
		t.Errorf("Expected empty entries, got %d", len(h.Entries))
	}
	if h.Results != nil {
		t.Errorf("Expected nil Results, got %v", h.Results)
	}
}

func TestHistoryStore_SaveEnforcesMaxEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	cfg := HistoryConfig{Path: path, MaxEntries: 3}

	h := NewHistoryStore(cfg)
	for i := range 10 {
		h.Append(&SpeedTestResult{DownloadSpeed: float64(i)})
	}

	if err := h.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the in-memory slice was truncated
	if len(h.Entries) != 3 {
		t.Fatalf("Expected 3 entries in memory after save, got %d", len(h.Entries))
	}
	if h.Entries[0].DownloadSpeed != 7 {
		t.Errorf("Expected oldest kept entry DownloadSpeed 7, got %v", h.Entries[0].DownloadSpeed)
	}
	if h.Entries[2].DownloadSpeed != 9 {
		t.Errorf("Expected newest entry DownloadSpeed 9, got %v", h.Entries[2].DownloadSpeed)
	}

	// Verify the file contents match
	h2 := NewHistoryStore(cfg)
	if err := h2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(h2.Entries) != 3 {
		t.Fatalf("Expected 3 entries on disk, got %d", len(h2.Entries))
	}
}

func TestHistoryStore_LoadBackupRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	bakPath := path + ".bak"
	cfg := HistoryConfig{Path: path, MaxEntries: 50}

	h := NewHistoryStore(cfg)
	h.Append(&SpeedTestResult{DownloadSpeed: 42, ServerName: "TestServer"})

	// Two saves: first creates the file, second creates the .bak
	if err := h.Save(); err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if err := h.Save(); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	// Corrupt main + valid backup — should recover
	_ = os.WriteFile(path, []byte("invalid json"), 0600)
	h2 := NewHistoryStore(cfg)
	if err := h2.Load(); err != nil {
		t.Fatalf("expected backup recovery to succeed, got: %v", err)
	}
	if len(h2.Entries) != 1 {
		t.Errorf("expected 1 entry recovered from backup, got %d", len(h2.Entries))
	}
	if h2.Results == nil || h2.Results.DownloadSpeed != 42 {
		t.Errorf("expected Results.DownloadSpeed 42, got %v", h2.Results)
	}

	// Corrupt main + no backup — should return error
	_ = os.Remove(bakPath)
	h3 := NewHistoryStore(cfg)
	if err := h3.Load(); err == nil {
		t.Error("expected error loading corrupt file with no backup, got nil")
	}

	// Corrupt main + corrupt backup — should return error
	_ = os.WriteFile(bakPath, []byte("also invalid"), 0600)
	h4 := NewHistoryStore(cfg)
	if err := h4.Load(); err == nil {
		t.Error("expected error when both main and backup are corrupt, got nil")
	}
}

func TestHistoryStore_SaveSkipsCorruptBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	bakPath := path + ".bak"
	cfg := HistoryConfig{Path: path, MaxEntries: 50}

	h := NewHistoryStore(cfg)
	h.Append(&SpeedTestResult{DownloadSpeed: 42})
	if err := h.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Corrupt the main file
	_ = os.WriteFile(path, []byte("corrupt"), 0600)

	// Save new data — should NOT back up the corrupt main file
	h2 := NewHistoryStore(cfg)
	h2.Append(&SpeedTestResult{DownloadSpeed: 99})
	if err := h2.Save(); err != nil {
		t.Fatalf("save over corrupt failed: %v", err)
	}

	if bakData, err := os.ReadFile(bakPath); err == nil {
		if string(bakData) == "corrupt" {
			t.Error("backup contains corrupt data — json.Valid guard did not prevent overwrite")
		}
	}
}

func TestHistoryStore_SaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "history.json")
	h := NewHistoryStore(HistoryConfig{Path: path})
	h.Append(&SpeedTestResult{DownloadSpeed: 1})

	if err := h.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected history file to be created at %s", path)
	}
}
