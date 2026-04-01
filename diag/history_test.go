package diag

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testTimestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func TestLoadHistoryMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	results, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(results))
	}
}

func TestSaveAndLoadHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "diagnostics.json")

	results := []*DiagResult{
		{
			Target:    testExampleHost,
			Method:    MethodUDP,
			Hops:      []Hop{{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 5 * time.Millisecond}},
			Quality:   QualityScore{Score: 85, Grade: "B", Label: "Good for most activities"},
			Timestamp: testTimestamp,
		},
	}

	if err := SaveHistory(path, results, 20); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded))
	}
	if loaded[0].Target != testExampleHost {
		t.Errorf("target = %q, want %q", loaded[0].Target, testExampleHost)
	}
}

func TestSaveHistoryRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.json")

	var results []*DiagResult
	for i := 0; i < 25; i++ {
		results = append(results, &DiagResult{
			Target:    "target",
			Timestamp: testTimestamp.Add(time.Duration(i) * time.Minute),
		})
	}

	if err := SaveHistory(path, results, 20); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 20 {
		t.Errorf("expected 20 entries after trim, got %d", len(loaded))
	}
}

func TestSaveHistoryMaxEntriesZero_NoTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.json")

	var results []*DiagResult
	for i := range 25 {
		results = append(results, &DiagResult{
			Target:    "target",
			Timestamp: testTimestamp.Add(time.Duration(i) * time.Minute),
		})
	}

	if err := SaveHistory(path, results, 0); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 25 {
		t.Errorf("expected 25 entries (no truncation with maxEntries=0), got %d", len(loaded))
	}
}

func TestSaveHistoryNilSlice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.json")

	if err := SaveHistory(path, nil, 20); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries for nil slice, got %d", len(loaded))
	}
}

func TestAppendHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "diag_history.json")

	// Append to non-existent file creates it
	result := &DiagResult{Target: testExampleHost, Method: MethodICMP}
	if err := AppendHistory(path, result, 5); err != nil {
		t.Fatalf("AppendHistory on new file: %v", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}
	if history[0].Target != testExampleHost {
		t.Errorf("target = %q, want %q", history[0].Target, testExampleHost)
	}

	// Append respects maxEntries
	for i := range 6 {
		r := &DiagResult{Target: fmt.Sprintf("host-%d", i), Method: MethodUDP}
		if err := AppendHistory(path, r, 5); err != nil {
			t.Fatalf("AppendHistory iteration %d: %v", i, err)
		}
	}

	history, err = LoadHistory(path)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("expected 5 entries (maxEntries), got %d", len(history))
	}
	if history[4].Target != "host-5" {
		t.Errorf("last entry target = %q, want %q", history[4].Target, "host-5")
	}
}

func TestLoadHistoryBackupRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "diagnostics.json")
	bakPath := path + ".bak"

	validResults := []*DiagResult{
		{Target: testExampleHost, Method: MethodUDP, Timestamp: testTimestamp},
	}

	// First save creates the file; second save backs it up to .bak
	if err := SaveHistory(path, validResults, 20); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	if err := SaveHistory(path, validResults, 20); err != nil {
		t.Fatalf("second save (to create .bak) failed: %v", err)
	}
	_ = os.WriteFile(path, []byte("invalid json"), 0600)

	// Corrupt main + valid backup — should recover
	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("expected backup recovery to succeed, got: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("expected 1 entry recovered from backup, got %d", len(loaded))
	}
	if loaded[0].Target != testExampleHost {
		t.Errorf("target = %q, want %q", loaded[0].Target, testExampleHost)
	}

	// Corrupt main + no backup — should return error
	_ = os.Remove(bakPath)
	_, err = LoadHistory(path)
	if err == nil {
		t.Error("expected error loading corrupt file with no backup, got nil")
	}

	// Corrupt main + corrupt backup — should return error mentioning both
	_ = os.WriteFile(bakPath, []byte("also invalid"), 0600)
	_, err = LoadHistory(path)
	if err == nil {
		t.Error("expected error when both main and backup are corrupt, got nil")
	}
}

func TestSaveHistorySkipsCorruptBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "diagnostics.json")
	bakPath := path + ".bak"

	validResults := []*DiagResult{
		{Target: testExampleHost, Method: MethodUDP, Timestamp: testTimestamp},
	}

	// Save valid data, then corrupt the main file
	if err := SaveHistory(path, validResults, 20); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	_ = os.WriteFile(path, []byte("corrupt"), 0600)

	// Save again — should NOT back up the corrupt main file
	newResults := []*DiagResult{
		{Target: "new.com", Method: MethodICMP, Timestamp: testTimestamp},
	}
	if err := SaveHistory(path, newResults, 20); err != nil {
		t.Fatalf("save over corrupt failed: %v", err)
	}

	// The .bak should either not exist or not contain the corrupt data
	if bakData, err := os.ReadFile(bakPath); err == nil {
		if string(bakData) == "corrupt" {
			t.Error("backup contains corrupt data — json.Valid guard did not prevent overwrite")
		}
	}
}

func TestSaveHistoryPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.json")
	results := []*DiagResult{{Target: "test", Timestamp: testTimestamp}}

	if err := SaveHistory(path, results, 20); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}
