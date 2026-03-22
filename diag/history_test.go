package diag

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
			Target:    "example.com",
			Method:    MethodUDP,
			Hops:      []Hop{{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 5 * time.Millisecond}},
			Quality:   QualityScore{Score: 85, Grade: "B", Label: "Good for most activities"},
			Timestamp: time.Now(),
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
	if loaded[0].Target != "example.com" {
		t.Errorf("target = %q, want %q", loaded[0].Target, "example.com")
	}
}

func TestSaveHistoryRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.json")

	var results []*DiagResult
	for i := 0; i < 25; i++ {
		results = append(results, &DiagResult{
			Target:    "target",
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
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

func TestSaveHistoryPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.json")
	results := []*DiagResult{{Target: "test", Timestamp: time.Now()}}

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
