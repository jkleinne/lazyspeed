package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
)

func TestDiagIsInteractive(t *testing.T) {
	origJSON := diagJSON
	origCSV := diagCSV
	origSimple := diagSimple
	defer func() { diagJSON = origJSON; diagCSV = origCSV; diagSimple = origSimple }()

	tests := []struct {
		name   string
		json   bool
		csv    bool
		simple bool
		want   bool
	}{
		{"all false is interactive", false, false, false, true},
		{"json disables interactive", true, false, false, false},
		{"csv disables interactive", false, true, false, false},
		{"simple disables interactive", false, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagJSON = tt.json
			diagCSV = tt.csv
			diagSimple = tt.simple
			if got := diagIsInteractive(); got != tt.want {
				t.Errorf("diagIsInteractive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiagConfig(t *testing.T) {
	t.Run("custom values", func(t *testing.T) {
		got := diagConfig(model.DiagnosticsConfig{
			MaxHops:    15,
			Timeout:    30,
			MaxEntries: 10,
			Path:       "/custom/path",
		})

		if got.MaxHops != 15 {
			t.Errorf("MaxHops = %d, want 15", got.MaxHops)
		}
		if got.Timeout != 30 {
			t.Errorf("Timeout = %d, want 30", got.Timeout)
		}
		if got.MaxEntries != 10 {
			t.Errorf("MaxEntries = %d, want 10", got.MaxEntries)
		}
		if got.Path != "/custom/path" {
			t.Errorf("Path = %q, want %q", got.Path, "/custom/path")
		}
	})

	t.Run("zero values fall back to defaults", func(t *testing.T) {
		got := diagConfig(model.DiagnosticsConfig{})
		defaults := diag.DefaultDiagConfig()

		if got.MaxHops != defaults.MaxHops {
			t.Errorf("MaxHops = %d, want default %d", got.MaxHops, defaults.MaxHops)
		}
		if got.Timeout != defaults.Timeout {
			t.Errorf("Timeout = %d, want default %d", got.Timeout, defaults.Timeout)
		}
		if got.MaxEntries != defaults.MaxEntries {
			t.Errorf("MaxEntries = %d, want default %d", got.MaxEntries, defaults.MaxEntries)
		}
		if got.Path != defaults.Path {
			t.Errorf("Path = %q, want default %q", got.Path, defaults.Path)
		}
	})
}

func TestDiagCmdFlagDefaults(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{"json default", "json", "false"},
		{"csv default", "csv", "false"},
		{"simple default", "simple", "false"},
		{"history default", "history", "false"},
		{"server default", "server", ""},
		{"last default", "last", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := diagCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag %q not found", tt.flag)
				return
			}
			if f.DefValue != tt.want {
				t.Errorf("flag %q default = %q, want %q", tt.flag, f.DefValue, tt.want)
			}
		})
	}
}

func TestDiagCmdExists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "diag [target]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("diag command not registered on rootCmd")
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"host with port", "192.168.1.1:443", "192.168.1.1"},
		{"IPv6 with port", "[::1]:80", "::1"},
		{"hostname without port", "example.com", "example.com"},
		{"IP without port", "10.0.0.1", "10.0.0.1"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPort(tt.input)
			if got != tt.want {
				t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDiagCSVRow(t *testing.T) {
	ts := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	t.Run("with DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target: "example.com",
			Method: diag.MethodICMP,
			Hops: []diag.Hop{
				{Number: 1, Latency: 5 * time.Millisecond},
			},
			DNS: &diag.DNSResult{
				Host:    "example.com",
				IP:      "93.184.216.34",
				Latency: 2 * time.Millisecond,
				Cached:  false,
			},
			Quality:   diag.QualityScore{Score: 85, Grade: "B", Label: "Good"},
			Timestamp: ts,
		}

		row := diagCSVRow(r)
		if len(row) != len(diagCSVHeader) {
			t.Fatalf("column count = %d, want %d", len(row), len(diagCSVHeader))
		}
		if row[1] != "example.com" {
			t.Errorf("target column = %q, want %q", row[1], "example.com")
		}
		if row[4] != "B" {
			t.Errorf("grade column = %q, want %q", row[4], "B")
		}
	})

	t.Run("nil DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target:    "10.0.0.1",
			Method:    diag.MethodUDP,
			Hops:      []diag.Hop{},
			DNS:       nil,
			Quality:   diag.QualityScore{Score: 70, Grade: "C", Label: "Fair"},
			Timestamp: ts,
		}

		row := diagCSVRow(r)
		if len(row) != len(diagCSVHeader) {
			t.Fatalf("column count = %d, want %d", len(row), len(diagCSVHeader))
		}
		// dns_ms is column index 5, dns_cached is column index 6
		if row[5] != "" {
			t.Errorf("dns_ms column = %q, want empty string", row[5])
		}
		if row[6] != "" {
			t.Errorf("dns_cached column = %q, want empty string", row[6])
		}
	})
}

func TestDiagSimpleLine(t *testing.T) {
	t.Run("with DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target: "example.com",
			Method: diag.MethodICMP,
			Hops: []diag.Hop{
				{Number: 1, Latency: 5 * time.Millisecond},
				{Number: 2, Latency: 10 * time.Millisecond},
			},
			DNS: &diag.DNSResult{
				Host:    "example.com",
				IP:      "93.184.216.34",
				Latency: 2 * time.Millisecond,
			},
			Quality: diag.QualityScore{Score: 85, Grade: "B"},
		}

		got := diagSimpleLine(r)
		if !strings.Contains(got, "Score: 85/B") {
			t.Errorf("expected score/grade in output, got %q", got)
		}
		if !strings.Contains(got, "Hops: 2") {
			t.Errorf("expected hop count in output, got %q", got)
		}
	})

	t.Run("nil DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target:  "10.0.0.1",
			Method:  diag.MethodUDP,
			Hops:    []diag.Hop{},
			DNS:     nil,
			Quality: diag.QualityScore{Score: 70, Grade: "C"},
		}

		got := diagSimpleLine(r)
		if !strings.Contains(got, "DNS: -") {
			t.Errorf("expected 'DNS: -' for nil DNS, got %q", got)
		}
	})
}

func TestDiagDefaultOutput(t *testing.T) {
	r := &diag.DiagResult{
		Target: "example.com",
		Method: diag.MethodICMP,
		Hops: []diag.Hop{
			{Number: 1, IP: "192.168.1.1", Host: "gateway.local", Latency: 2 * time.Millisecond},
			{Number: 2, Timeout: true},
			{Number: 3, IP: "93.184.216.34", Host: "example.com", Latency: 15 * time.Millisecond},
		},
		DNS: &diag.DNSResult{
			Host:    "example.com",
			IP:      "93.184.216.34",
			Latency: 3 * time.Millisecond,
		},
		Quality:   diag.QualityScore{Score: 80, Grade: "B", Label: "Good"},
		Timestamp: time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
	}

	got := diagDefaultOutput(r)

	checks := []struct {
		label    string
		contains string
	}{
		{"target", "example.com"},
		{"method", "icmp"},
		{"score and grade", "80 / B"},
		{"quality label", "Good"},
		{"host with IP", "gateway.local (192.168.1.1)"},
		{"timeout marker", "*"},
		{"hop count", "Hops (3)"},
	}

	for _, c := range checks {
		if !strings.Contains(got, c.contains) {
			t.Errorf("expected output to contain %s (%q), got:\n%s", c.label, c.contains, got)
		}
	}
}

// saveDiagFlags saves and defers restoration of all diag-related package globals.
func saveDiagFlags(t *testing.T) {
	t.Helper()
	origJSON := diagJSON
	origCSV := diagCSV
	origSimple := diagSimple
	origHistory := diagHistory
	origLast := diagLast
	t.Cleanup(func() {
		diagJSON = origJSON
		diagCSV = origCSV
		diagSimple = origSimple
		diagHistory = origHistory
		diagLast = origLast
	})
}

// makeDiagHistory builds a slice of DiagResult values for use in tests.
func makeDiagHistory(n int) []*diag.DiagResult {
	results := make([]*diag.DiagResult, n)
	for i := range n {
		results[i] = &diag.DiagResult{
			Target: "8.8.8.8",
			Method: diag.MethodICMP,
			Hops: []diag.Hop{
				{Number: 1, IP: "10.0.0.1", Latency: 5 * time.Millisecond},
			},
			Quality:   diag.QualityScore{Score: 90, Grade: "A", Label: "Great"},
			Timestamp: time.Date(2026, 1, i+1, 12, 0, 0, 0, time.UTC),
		}
	}
	return results
}

// populateDiagHistory writes diag history entries to the XDG-compliant path under tmpDir.
func populateDiagHistory(t *testing.T, tmpDir string, entries []*diag.DiagResult) {
	t.Helper()
	histDir := filepath.Join(tmpDir, ".local", "share", "lazyspeed")
	if err := os.MkdirAll(histDir, 0700); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := diag.SaveHistory(filepath.Join(histDir, "diagnostics.json"), entries, 20); err != nil {
		t.Fatalf("SaveHistory failed: %v", err)
	}
}

func TestRunDiagHistoryEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saveDiagFlags(t)

	diagJSON = false
	diagCSV = false
	diagSimple = false
	diagHistory = false
	diagLast = 0

	out := captureStdout(runDiagHistory)

	if !strings.Contains(out, "No diagnostics history found.") {
		t.Errorf("expected 'No diagnostics history found.' in output, got %q", out)
	}
}

func TestRunDiagHistoryJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	saveDiagFlags(t)

	populateDiagHistory(t, tmpDir, makeDiagHistory(1))

	diagJSON = true
	diagCSV = false
	diagSimple = false
	diagHistory = false
	diagLast = 0

	out := captureStdout(runDiagHistory)

	var arr []*diag.DiagResult
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("expected valid JSON array, got parse error: %v\noutput: %s", err, out)
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 entry, got %d", len(arr))
	}
}

func TestRunDiagHistoryLastSlice(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	saveDiagFlags(t)

	populateDiagHistory(t, tmpDir, makeDiagHistory(3))

	diagJSON = true
	diagCSV = false
	diagSimple = false
	diagHistory = false
	diagLast = 2

	out := captureStdout(runDiagHistory)

	var arr []*diag.DiagResult
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("expected valid JSON array, got parse error: %v\noutput: %s", err, out)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 entries with --last 2, got %d", len(arr))
	}
}
