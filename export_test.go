package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

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

func TestExportResultJSON(t *testing.T) {
	dir := t.TempDir()
	result := &model.SpeedTestResult{
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
	var got model.SpeedTestResult
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
	result := &model.SpeedTestResult{
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
	result := &model.SpeedTestResult{Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}

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
	result := &model.SpeedTestResult{Timestamp: ts}

	path, err := ExportResult(result, "json", dir)
	if err != nil {
		t.Fatalf("ExportResult failed: %v", err)
	}
	base := filepath.Base(path)
	if !strings.Contains(base, "20260315_123045_000000000") {
		t.Errorf("Expected filename to contain timestamp '20260315_123045_000000000', got %q", base)
	}
}

func TestExportResultUnwritableDirectory(t *testing.T) {
	readOnlyDir := makeReadOnlyDir(t)

	result := &model.SpeedTestResult{
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
