package diag

import (
	"fmt"
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

	results := []*Result{
		{
			Target:    testExampleHost,
			Method:    MethodUDP,
			Hops:      []Hop{{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 5 * time.Millisecond}},
			Quality:   QualityScore{Score: 85, Grade: "B", Label: "Good for most activities"},
			Timestamp: testTimestamp,
		},
	}

	if err := saveHistory(path, results, 20); err != nil {
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

	var results []*Result
	for i := range 25 {
		results = append(results, &Result{
			Target:    "target",
			Timestamp: testTimestamp.Add(time.Duration(i) * time.Minute),
		})
	}

	if err := saveHistory(path, results, 20); err != nil {
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

	var results []*Result
	for i := range 25 {
		results = append(results, &Result{
			Target:    "target",
			Timestamp: testTimestamp.Add(time.Duration(i) * time.Minute),
		})
	}

	if err := saveHistory(path, results, 0); err != nil {
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

	if err := saveHistory(path, nil, 20); err != nil {
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
	result := &Result{Target: testExampleHost, Method: MethodICMP}
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
		r := &Result{Target: fmt.Sprintf("host-%d", i), Method: MethodUDP}
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
