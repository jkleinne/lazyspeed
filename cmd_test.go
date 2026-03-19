package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

func TestVersionCommand(t *testing.T) {
	origVersion := version
	version = "1.2.3"
	defer func() { version = origVersion }()

	// We can't easily capture stdout from os.Stdout in tests if the command writes directly to fmt.Println(GetVersionInfo())
	// But we can check that versionCmd exists and can be executed
	if versionCmd.Use != "version" {
		t.Errorf("expected version command")
	}
}

// We just ensure the commands exist and are wired up properly.
func TestCommandsConfigured(t *testing.T) {
	var foundVersion, foundRun, foundHistory bool
	for _, cmd := range rootCmd.Commands() {
		switch cmd.Name() {
		case "version":
			foundVersion = true
		case "run":
			foundRun = true
		case "history":
			foundHistory = true
		}
	}

	if !foundVersion {
		t.Error("version command not registered")
	}
	if !foundRun {
		t.Error("run command not registered")
	}
	if !foundHistory {
		t.Error("history command not registered")
	}
}

func TestMarshalJSONResultsEmpty(t *testing.T) {
	data, err := marshalJSONResults([]*model.SpeedTestResult{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Must be a valid empty JSON array, not null
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("Expected valid JSON array, got parse error: %v\noutput: %s", err, data)
	}
	if len(arr) != 0 {
		t.Errorf("Expected empty array, got length %d", len(arr))
	}
}

func TestMarshalJSONResultsSingle(t *testing.T) {
	res := &model.SpeedTestResult{
		DownloadSpeed: 95.12,
		UploadSpeed:   45.23,
		Ping:          12.40,
		Jitter:        1.50,
		ServerName:    "Test Server",
		Timestamp:     time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC),
	}

	data, err := marshalJSONResults([]*model.SpeedTestResult{res})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Must be a valid JSON object, not an array
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("Expected bare JSON object, got parse error: %v\noutput: %s", err, data)
	}

	if _, ok := obj["download_speed"]; !ok {
		t.Errorf("Expected download_speed key in JSON object")
	}
}

func TestMarshalJSONResultsMultiple(t *testing.T) {
	results := []*model.SpeedTestResult{
		{DownloadSpeed: 95.12, Ping: 12.40, Timestamp: time.Now()},
		{DownloadSpeed: 97.44, Ping: 11.90, Timestamp: time.Now()},
	}

	data, err := marshalJSONResults(results)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Must be a valid JSON array of length 2
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("Expected JSON array, got parse error: %v\noutput: %s", err, data)
	}

	if len(arr) != 2 {
		t.Errorf("Expected array length 2, got %d", len(arr))
	}

	for i, item := range arr {
		if _, ok := item["download_speed"]; !ok {
			t.Errorf("Expected download_speed key in array item %d", i)
		}
	}
}

// captureStdout redirects os.Stdout during fn and returns what was written.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// makeHistoryEntries builds a slice of SpeedTestResult values for use in tests.
func makeHistoryEntries(n int) []*model.SpeedTestResult {
	results := make([]*model.SpeedTestResult, n)
	for i := range n {
		results[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			UploadSpeed:   float64(50 + i),
			Ping:          float64(10 + i),
			Jitter:        1.0,
			ServerName:    "Server",
			ServerCountry:     "US",
			UserIP:        "1.2.3.4",
			UserISP:       "TestISP",
			Timestamp:     time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC),
		}
	}
	return results
}

func TestHistoryFormatJSON(t *testing.T) {
	// Write temp history file
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = makeHistoryEntries(3)
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	// Reset globals
	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = historyFormatJSON
	historyLast = 0
	historyClear = false

	out := captureStdout(runHistory)

	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("Expected valid JSON array, got parse error: %v\noutput: %s", err, out)
	}
	if len(arr) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(arr))
	}
	if _, ok := arr[0]["download_speed"]; !ok {
		t.Errorf("Expected download_speed key in JSON output")
	}
}

func TestHistoryFormatCSV(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = makeHistoryEntries(2)
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = historyFormatCSV
	historyLast = 0
	historyClear = false

	out := captureStdout(runHistory)

	r := csv.NewReader(strings.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("Expected valid CSV, got parse error: %v\noutput: %s", err, out)
	}

	// Header + 2 data rows
	if len(records) != 3 {
		t.Errorf("Expected 3 CSV records (header + 2 rows), got %d", len(records))
	}
	if records[0][0] != "timestamp" {
		t.Errorf("Expected first header column to be 'timestamp', got %q", records[0][0])
	}
	expectedHeaders := []string{"timestamp", "server", "country", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "ip", "isp"}
	for i, h := range expectedHeaders {
		if records[0][i] != h {
			t.Errorf("Expected header[%d] = %q, got %q", i, h, records[0][i])
		}
	}
}

func TestHistoryLastFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = makeHistoryEntries(5)
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = historyFormatJSON
	historyLast = 2
	historyClear = false

	out := captureStdout(runHistory)

	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("Expected valid JSON array, got parse error: %v\noutput: %s", err, out)
	}
	if len(arr) != 2 {
		t.Errorf("Expected 2 entries with --last 2, got %d", len(arr))
	}
}

func TestHistoryLastFlagExceedsLength(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = makeHistoryEntries(3)
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = historyFormatJSON
	historyLast = 100 // more than available
	historyClear = false

	out := captureStdout(runHistory)

	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("Expected valid JSON array, got parse error: %v\noutput: %s", err, out)
	}
	if len(arr) != 3 {
		t.Errorf("Expected all 3 entries when --last exceeds length, got %d", len(arr))
	}
}

func TestHistoryClearFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = makeHistoryEntries(3)
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyClear = true
	historyFormat = ""
	historyLast = 0

	out := captureStdout(runHistory)

	if !strings.Contains(out, "History cleared.") {
		t.Errorf("Expected 'History cleared.' in output, got %q", out)
	}

	m2 := model.NewDefaultModel()
	if err := m2.LoadHistory(); err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}
	if len(m2.TestHistory) != 0 {
		t.Errorf("Expected empty history after clear, got %d entries", len(m2.TestHistory))
	}
}

func TestHistoryDefaultTableFormat(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = makeHistoryEntries(2)
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = ""
	historyLast = 0
	historyClear = false

	out := captureStdout(runHistory)

	if !strings.Contains(out, "DATE") {
		t.Errorf("Expected 'DATE' header in table output")
	}
	if !strings.Contains(out, "SERVER") {
		t.Errorf("Expected 'SERVER' header in table output")
	}
	if !strings.Contains(out, "DL (Mbps)") {
		t.Errorf("Expected 'DL (Mbps)' header in table output")
	}
}

func TestHistoryEmptyHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = ""
	historyLast = 0
	historyClear = false

	out := captureStdout(runHistory)

	if !strings.Contains(out, "No history found.") {
		t.Errorf("Expected 'No history found.' in output, got %q", out)
	}
}

func TestHistoryCommandValidation(t *testing.T) {
	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = "xml"
	historyLast = 0
	historyClear = false
	err := historyCmd.RunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("Expected 'invalid --format' error, got %v", err)
	}

	historyFormat = ""
	historyLast = -1
	err = historyCmd.RunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--last must be >= 0") {
		t.Errorf("Expected '--last must be >= 0' error, got %v", err)
	}
}

func TestRunSimpleOutputFormat(t *testing.T) {
	res := &model.SpeedTestResult{
		DownloadSpeed: 95.12,
		UploadSpeed:   45.23,
		Ping:          12.40,
	}
	got := formatSimpleResult(res)
	expected := "DL: 95.12 Mbps | UL: 45.23 Mbps | Ping: 12.40 ms"
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestRunDefaultOutputFormat(t *testing.T) {
	res := &model.SpeedTestResult{
		DownloadSpeed: 95.12,
		UploadSpeed:   45.23,
		Ping:          12.40,
		Jitter:        1.50,
	}
	got := formatDefaultResult(res)

	for _, want := range []string{
		"📥 Download: 95.12 Mbps",
		"📤 Upload: 45.23 Mbps",
		"🔄 Ping: 12.40 ms",
		"📊 Jitter: 1.50 ms",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Expected output to contain %q, got %q", want, got)
		}
	}
}

func TestHistoryTableFormatTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.TestHistory = []*model.SpeedTestResult{
		{
			DownloadSpeed: 100.0,
			UploadSpeed:   50.0,
			Ping:          10.0,
			ServerName:    "A Very Long Server Name XY",
			ServerCountry: "US",
			Timestamp:     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	if err := m.SaveHistory(); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}

	origFormat := historyFormat
	origLast := historyLast
	origClear := historyClear
	defer func() {
		historyFormat = origFormat
		historyLast = origLast
		historyClear = origClear
	}()

	historyFormat = ""
	historyLast = 0
	historyClear = false

	out := captureStdout(runHistory)

	// Truncation threshold at cmd_history.go:104 — serverStr[:17] + "..."
	if !strings.Contains(out, "A Very Long Serve...") {
		t.Errorf("Expected truncated server name 'A Very Long Serve...' in output, got %q", out)
	}
	if strings.Contains(out, "A Very Long Server Name XY") {
		t.Errorf("Expected full server name to be truncated, but found it in output")
	}
}

func TestRunCommandValidation(t *testing.T) {
	origCount := runCount
	defer func() { runCount = origCount }()

	runCount = 0
	err := runCmd.PreRunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--count must be at least 1") {
		t.Errorf("Expected '--count must be at least 1' error, got %v", err)
	}

	runCount = 1
	err = runCmd.PreRunE(nil, nil)
	if err != nil {
		t.Errorf("Expected nil error for valid count, got %v", err)
	}
}
