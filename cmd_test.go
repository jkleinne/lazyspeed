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
	"github.com/spf13/cobra"
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
	var foundVersion, foundRun, foundHistory, foundServers, foundCompletion bool
	for _, cmd := range rootCmd.Commands() {
		switch cmd.Name() {
		case "version":
			foundVersion = true
		case "run":
			foundRun = true
		case "history":
			foundHistory = true
		case "servers":
			foundServers = true
		case "completion":
			foundCompletion = true
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
	if !foundServers {
		t.Error("servers command not registered")
	}
	if !foundCompletion {
		t.Error("completion command not registered")
	}
}

func TestCompletionSubcommands(t *testing.T) {
	expected := map[string]bool{"bash": false, "zsh": false, "fish": false, "powershell": false}
	for _, cmd := range completionCmd.Commands() {
		if _, ok := expected[cmd.Name()]; ok {
			expected[cmd.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("completion subcommand %q not registered", name)
		}
	}
}

func TestCompletionGeneration(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"bash", completionBashCmd},
		{"zsh", completionZshCmd},
		{"fish", completionFishCmd},
		{"powershell", completionPowershellCmd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect stdout to capture output
			r, w, _ := os.Pipe()
			origStdout := os.Stdout
			os.Stdout = w

			err := tt.cmd.RunE(nil, nil)

			_ = w.Close()
			os.Stdout = origStdout

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			if buf.Len() == 0 {
				t.Errorf("Expected non-empty completion output for %s", tt.name)
			}
		})
	}
}

func TestManCommand(t *testing.T) {
	if !manCmd.Hidden {
		t.Error("man command should be hidden")
	}

	f := manCmd.Flags().Lookup("dir")
	if f == nil {
		t.Fatal("man command missing --dir flag")
	}

	t.Run("missing dir flag", func(t *testing.T) {
		origDir := manDir
		defer func() { manDir = origDir }()
		manDir = ""
		err := manCmd.RunE(nil, nil)
		if err == nil || !strings.Contains(err.Error(), "--dir is required") {
			t.Errorf("Expected '--dir is required' error, got %v", err)
		}
	})

	t.Run("generates man pages", func(t *testing.T) {
		origDir := manDir
		defer func() { manDir = origDir }()
		manDir = t.TempDir()
		err := manCmd.RunE(nil, nil)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		entries, err := os.ReadDir(manDir)
		if err != nil {
			t.Fatalf("Could not read output dir: %v", err)
		}
		if len(entries) == 0 {
			t.Error("Expected man page files to be generated")
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".1") {
				t.Errorf("Expected .1 man page file, got %q", e.Name())
			}
		}
	})
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
		{DownloadSpeed: 95.12, Ping: 12.40, Timestamp: testTimestamp},
		{DownloadSpeed: 97.44, Ping: 11.90, Timestamp: testTimestamp},
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

	_ = w.Close()
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
			ServerCountry: "US",
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
	m.History.Entries = makeHistoryEntries(3)
	if err := m.History.Save(); err != nil {
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

	historyFormat = formatJSON
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
	m.History.Entries = makeHistoryEntries(2)
	if err := m.History.Save(); err != nil {
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

	historyFormat = formatCSV
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
	m.History.Entries = makeHistoryEntries(5)
	if err := m.History.Save(); err != nil {
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

	historyFormat = formatJSON
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
	m.History.Entries = makeHistoryEntries(3)
	if err := m.History.Save(); err != nil {
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

	historyFormat = formatJSON
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
	m.History.Entries = makeHistoryEntries(3)
	if err := m.History.Save(); err != nil {
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
	if err := m2.History.Load(); err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}
	if len(m2.History.Entries) != 0 {
		t.Errorf("Expected empty history after clear, got %d entries", len(m2.History.Entries))
	}
}

func TestHistoryDefaultTableFormat(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	m := model.NewDefaultModel()
	m.History.Entries = makeHistoryEntries(2)
	if err := m.History.Save(); err != nil {
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
	expected := "Download: 95.12 Mbps | Upload: 45.23 Mbps | Ping: 12.40 ms"
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
		"Download  95.12 Mbps",
		"Upload    45.23 Mbps",
		"Ping      12.40 ms",
		"Jitter    1.50 ms",
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
	m.History.Entries = []*model.SpeedTestResult{
		{
			DownloadSpeed: 100.0,
			UploadSpeed:   50.0,
			Ping:          10.0,
			ServerName:    "A Very Long Server Name XY",
			ServerCountry: "US",
			Timestamp:     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		},
	}
	if err := m.History.Save(); err != nil {
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

	// Truncation via ui.Truncate at historyServerMaxLen=20: 19 runes + "…"
	if !strings.Contains(out, "A Very Long Server …") {
		t.Errorf("Expected truncated server name 'A Very Long Server …' in output, got %q", out)
	}
	if strings.Contains(out, "A Very Long Server Name XY") {
		t.Errorf("Expected full server name to be truncated, but found it in output")
	}
}

func TestRunIsInteractive(t *testing.T) {
	origJSON := runJSON
	origCSV := runCSV
	origSimple := runSimple
	defer func() { runJSON = origJSON; runCSV = origCSV; runSimple = origSimple }()

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
			runJSON = tt.json
			runCSV = tt.csv
			runSimple = tt.simple
			if got := runIsInteractive(); got != tt.want {
				t.Errorf("runIsInteractive() = %v, want %v", got, tt.want)
			}
		})
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

func TestValidateFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"json is valid", "json", false},
		{"csv is valid", "csv", false},
		{"xml is invalid", "xml", true},
		{"yaml is invalid", "yaml", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid --format") {
				t.Errorf("Expected error to contain 'invalid --format', got %q", err.Error())
			}
		})
	}
}

func TestPrintJSONServerEntries(t *testing.T) {
	entries := []serverEntry{
		{ID: "1", Name: "Server A", Sponsor: "SpA", Country: "US", Latency: 12.5, Distance: 100.3},
		{ID: "2", Name: "Server B", Sponsor: "SpB", Country: "DE", Latency: 25.0, Distance: 500.0},
	}

	out := captureStdout(func() { printJSON(entries) })

	var got []serverEntry
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("Expected valid JSON, got parse error: %v\noutput: %s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(got))
	}
	if got[0].ID != "1" || got[0].Name != "Server A" {
		t.Errorf("Unexpected first entry: %+v", got[0])
	}
	if got[1].Country != "DE" || got[1].Distance != 500.0 {
		t.Errorf("Unexpected second entry: %+v", got[1])
	}
}

func TestWriteCSVRowsServerData(t *testing.T) {
	header := []string{"id", "name", "sponsor", "country", "latency_ms", "distance_km"}
	rows := [][]string{
		{"1", "Server A", "SpA", "US", "12.50", "100.3"},
		{"2", "Server B", "SpB", "DE", "25.00", "500.0"},
	}

	out := captureStdout(func() { writeCSVRows(header, rows) })

	r := csv.NewReader(strings.NewReader(out))
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("Expected valid CSV, got parse error: %v\noutput: %s", err, out)
	}
	if len(records) != 3 {
		t.Fatalf("Expected 3 CSV records (header + 2 rows), got %d", len(records))
	}
	for i, h := range header {
		if records[0][i] != h {
			t.Errorf("Expected header[%d] = %q, got %q", i, h, records[0][i])
		}
	}
	if records[1][0] != "1" || records[1][1] != "Server A" {
		t.Errorf("Unexpected first data row: %v", records[1])
	}
	if records[2][3] != "DE" {
		t.Errorf("Expected country 'DE' in second row, got %q", records[2][3])
	}
}

func TestServersCommandValidation(t *testing.T) {
	origFormat := serversFormat
	defer func() { serversFormat = origFormat }()

	serversFormat = "xml"
	err := serversCmd.RunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("Expected 'invalid --format' error, got %v", err)
	}

	serversFormat = "yaml"
	err = serversCmd.RunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("Expected 'invalid --format' error for yaml, got %v", err)
	}
}

func TestRunCommandValidation_BestAndServersMutuallyExclusive(t *testing.T) {
	origBest := runBest
	origServers := runServerIDs
	origCount := runCount
	defer func() { runBest = origBest; runServerIDs = origServers; runCount = origCount }()

	runCount = 1
	runBest = 3
	runServerIDs = "1,2"
	err := runCmd.PreRunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--best and --servers are mutually exclusive") {
		t.Errorf("Expected '--best and --servers are mutually exclusive' error, got %v", err)
	}
}

func TestRunCommandValidation_BestMinimumTwo(t *testing.T) {
	origBest := runBest
	origServers := runServerIDs
	origCount := runCount
	defer func() { runBest = origBest; runServerIDs = origServers; runCount = origCount }()

	runCount = 1
	runServerIDs = ""
	runBest = 1
	err := runCmd.PreRunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--best must be at least 2") {
		t.Errorf("Expected '--best must be at least 2' error, got %v", err)
	}
}

func TestRunCommandValidation_CountAndBestMutuallyExclusive(t *testing.T) {
	origBest := runBest
	origServers := runServerIDs
	origCount := runCount
	defer func() { runBest = origBest; runServerIDs = origServers; runCount = origCount }()

	runCount = 3
	runBest = 2
	runServerIDs = ""
	err := runCmd.PreRunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--count and --best are mutually exclusive") {
		t.Errorf("Expected '--count and --best are mutually exclusive' error, got %v", err)
	}
}

func TestRunCommandValidation_CountAndServersMutuallyExclusive(t *testing.T) {
	origBest := runBest
	origServers := runServerIDs
	origCount := runCount
	defer func() { runBest = origBest; runServerIDs = origServers; runCount = origCount }()

	runCount = 3
	runBest = 0
	runServerIDs = "1,2"
	err := runCmd.PreRunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--count and --servers are mutually exclusive") {
		t.Errorf("Expected '--count and --servers are mutually exclusive' error, got %v", err)
	}
}

func TestRunCommandValidation_ServersMinimumTwo(t *testing.T) {
	origBest := runBest
	origServers := runServerIDs
	origCount := runCount
	defer func() { runBest = origBest; runServerIDs = origServers; runCount = origCount }()

	runCount = 1
	runBest = 0
	runServerIDs = "123"
	err := runCmd.PreRunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--servers requires at least 2 server IDs") {
		t.Errorf("Expected '--servers requires at least 2 server IDs' error, got %v", err)
	}
}

func TestFormatComparisonTable(t *testing.T) {
	results := []*model.SpeedTestResult{
		{
			ServerName:    "Tokyo Fiber",
			ServerSponsor: "NTT",
			ServerCountry: "Japan",
			Distance:      150.50,
			DownloadSpeed: 95.12,
			UploadSpeed:   45.23,
			Ping:          12.40,
			Jitter:        1.20,
			UserIP:        "1.2.3.4",
			UserISP:       "Bell",
		},
		{
			ServerName:    "London Edge",
			ServerSponsor: "BT",
			ServerCountry: "UK",
			Distance:      5400.00,
			DownloadSpeed: 80.00,
			UploadSpeed:   60.00,
			Ping:          20.00,
			Jitter:        2.50,
			UserIP:        "1.2.3.4",
			UserISP:       "Bell",
		},
		{
			ServerName:    "New York Hub",
			ServerSponsor: "Comcast",
			ServerCountry: "USA",
			Distance:      600.25,
			DownloadSpeed: 110.00,
			UploadSpeed:   40.00,
			Ping:          8.00,
			Jitter:        0.80,
			UserIP:        "1.2.3.4",
			UserISP:       "Bell",
		},
	}

	out := formatComparisonTable(results)

	// IP header must be present
	if !strings.Contains(out, "IP: 1.2.3.4 (Bell)") {
		t.Errorf("Expected IP header in output, got:\n%s", out)
	}

	// Column headers must be present
	for _, header := range []string{"SERVER", "SPONSOR", "DIST (km)", "DL (Mbps)", "UL (Mbps)", "PING (ms)", "JITTER (ms)"} {
		if !strings.Contains(out, header) {
			t.Errorf("Expected header %q in output, got:\n%s", header, out)
		}
	}

	// Server names and sponsors must be present
	for _, name := range []string{"Tokyo Fiber", "London Edge", "New York Hub"} {
		if !strings.Contains(out, name) {
			t.Errorf("Expected server name %q in output, got:\n%s", name, out)
		}
	}
	for _, sponsor := range []string{"NTT", "BT", "Comcast"} {
		if !strings.Contains(out, sponsor) {
			t.Errorf("Expected sponsor %q in output, got:\n%s", sponsor, out)
		}
	}

	// Star marker must appear (best values get marked)
	if !strings.Contains(out, "★") {
		t.Errorf("Expected star marker '★' in output, got:\n%s", out)
	}
}

func TestFormatComparisonTable_IdenticalValues(t *testing.T) {
	results := []*model.SpeedTestResult{
		{
			ServerName:    "Server A",
			ServerCountry: "US",
			DownloadSpeed: 100.00,
			UploadSpeed:   50.00,
			Ping:          10.00,
			Jitter:        1.00,
		},
		{
			ServerName:    "Server B",
			ServerCountry: "US",
			DownloadSpeed: 100.00,
			UploadSpeed:   50.00,
			Ping:          10.00,
			Jitter:        1.00,
		},
	}

	// Must not panic with identical values
	out := formatComparisonTable(results)

	if !strings.Contains(out, "Server A") || !strings.Contains(out, "Server B") {
		t.Errorf("Expected both server names in output, got:\n%s", out)
	}
}

func TestRunFavoritesMutualExclusivity(t *testing.T) {
	origFavorites := runFavorites
	origServerID := runServerID
	origServerIDs := runServerIDs
	origBest := runBest
	origCount := runCount
	defer func() {
		runFavorites = origFavorites
		runServerID = origServerID
		runServerIDs = origServerIDs
		runBest = origBest
		runCount = origCount
	}()

	tests := []struct {
		name        string
		serverID    string
		serverIDs   string
		best        int
		count       int
		wantErrFrag string
	}{
		{
			name:        "favorites and server are mutually exclusive",
			serverID:    "123",
			count:       1,
			wantErrFrag: "--favorites and --server are mutually exclusive",
		},
		{
			name:        "favorites and servers are mutually exclusive",
			serverIDs:   "1,2",
			count:       1,
			wantErrFrag: "--favorites and --servers are mutually exclusive",
		},
		{
			name:        "favorites and best are mutually exclusive",
			best:        3,
			count:       1,
			wantErrFrag: "--favorites and --best are mutually exclusive",
		},
		{
			name:        "favorites and count are mutually exclusive",
			count:       3,
			wantErrFrag: "--favorites and --count are mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCount = tt.count
			runFavorites = true
			runServerID = tt.serverID
			runServerIDs = tt.serverIDs
			runBest = tt.best

			err := runCmd.PreRunE(nil, nil)
			if err == nil {
				t.Fatal("expected mutual exclusivity error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrFrag) {
				t.Errorf("expected error containing %q, got %q", tt.wantErrFrag, err.Error())
			}
		})
	}
}

func TestServersPinUnpinMutualExclusivity(t *testing.T) {
	origPin := serversPin
	origUnpin := serversUnpin
	defer func() {
		serversPin = origPin
		serversUnpin = origUnpin
	}()

	serversPin = "123"
	serversUnpin = "456"

	err := serversCmd.RunE(nil, nil)
	if err == nil {
		t.Fatal("expected mutual exclusivity error, got nil")
	}
	if !strings.Contains(err.Error(), "--pin and --unpin are mutually exclusive") {
		t.Errorf("expected '--pin and --unpin are mutually exclusive' error, got %q", err.Error())
	}
}
