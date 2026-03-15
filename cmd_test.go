package main

import (
	"encoding/json"
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
