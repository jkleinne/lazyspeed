package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/showwin/speedtest-go/speedtest"
	"github.com/spf13/cobra"
)

const (
	// comparisonServerNameMaxLen is the maximum display width for server names
	// in the comparison table; names longer than this are truncated.
	comparisonServerNameMaxLen = 20

	// multiServerMinimum is the minimum number of servers required for --best and --servers.
	multiServerMinimum = 2

	// comparisonStarMarker is appended to rows that hold the best value in any metric.
	comparisonStarMarker = " ★"
)

var (
	runJSON       bool
	runCSV        bool
	runSimple     bool
	runServerID   string
	runNoUpload   bool
	runNoDownload bool
	runCount      int
	runBest       int
	runServerIDs  string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a speed test non-interactively",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if runCount < 1 {
			return fmt.Errorf("--count must be at least 1, got %d", runCount)
		}
		if runBest < 0 {
			return fmt.Errorf("--best must be a positive number, got %d", runBest)
		}
		if runBest > 0 && runServerIDs != "" {
			return fmt.Errorf("--best and --servers are mutually exclusive")
		}
		if runBest == 1 {
			return fmt.Errorf("--best must be at least 2, got %d", runBest)
		}
		if runCount > 1 && runBest > 0 {
			return fmt.Errorf("--count and --best are mutually exclusive")
		}
		if runCount > 1 && runServerIDs != "" {
			return fmt.Errorf("--count and --servers are mutually exclusive")
		}
		if runServerIDs != "" {
			ids := splitServerIDs(runServerIDs)
			if len(ids) < multiServerMinimum {
				return fmt.Errorf("--servers requires at least 2 server IDs, got %d", len(ids))
			}
		}
		return nil
	},
	Run: func(_ *cobra.Command, _ []string) {
		runHeadlessTest()
	},
}

func init() {
	runCmd.Flags().BoolVar(&runJSON, "json", false, "Output as JSON; single run emits a bare object, --count N>1 emits a JSON array")
	runCmd.Flags().BoolVar(&runCSV, "csv", false, "Output results as CSV to stdout")
	runCmd.Flags().BoolVar(&runSimple, "simple", false, "Minimal output (one line: DL/UL/Ping)")
	runCmd.Flags().StringVar(&runServerID, "server", "", "Skip server selection, use a specific server ID")
	runCmd.Flags().BoolVar(&runNoUpload, "no-upload", false, "Skip upload phase")
	runCmd.Flags().BoolVar(&runNoDownload, "no-download", false, "Skip download phase")
	runCmd.Flags().IntVar(&runCount, "count", 1, "Run multiple tests sequentially")
	runCmd.Flags().IntVar(&runBest, "best", 0, "Auto-select the N closest servers for comparison (minimum 2)")
	runCmd.Flags().StringVar(&runServerIDs, "servers", "", "Test specific servers by ID (comma-separated, minimum 2)")

	rootCmd.AddCommand(runCmd)
}

func runIsInteractive() bool {
	return !runJSON && !runCSV && !runSimple
}

// splitServerIDs splits a comma-separated server ID string into trimmed, non-empty IDs.
func splitServerIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

// prepareRunServer fetches the server list and resolves the target server index.
// It exits the process on failure (no servers found, server ID not found).
func prepareRunServer(m *model.Model, serverID string, interactive bool) int {
	if interactive {
		fmt.Println("Fetching server list...")
	}
	fetchServersOrExit(m)

	if m.Servers.Len() == 0 {
		exitWithError("no servers found")
	}

	serverIdx := 0
	if serverID != "" {
		idx, found := m.Servers.FindIndex(serverID)
		if !found {
			exitWithError("server %s not found", serverID)
		}
		serverIdx = idx
	}

	if interactive {
		server := m.Servers.Raw()[serverIdx]
		fmt.Printf("Selected server: %s (%s)\n", server.Name, server.Country)
	}
	return serverIdx
}

// writeRunResults emits collected JSON or CSV results after all test runs complete.
// JSON is emitted once after all runs so that --count N>1 produces valid JSON
// (emitting per-iteration would produce concatenated bare objects).
func writeRunResults(jsonResults []*model.SpeedTestResult, csvRows [][]string, outputJSON, outputCSV bool) {
	if !outputJSON && !outputCSV {
		return
	}

	if outputJSON {
		data, err := marshalJSONResults(jsonResults)
		if err != nil {
			exitWithError("serialising results: %v", err)
		}
		fmt.Println(string(data))
	}

	if outputCSV {
		writeCSVRows(model.SpeedTestCSVHeader, csvRows)
	}
}

// resolveMultiServers resolves the server list for multi-server flags.
// For --best N it returns the first N servers from the already-sorted (by latency) list.
// For --servers it looks up each ID and exits on any missing ID.
func resolveMultiServers(m *model.Model, interactive bool) []*speedtest.Server {
	if interactive {
		fmt.Println("Fetching server list...")
	}
	fetchServersOrExit(m)

	if m.Servers.Len() == 0 {
		exitWithError("no servers found")
	}

	if runBest > 0 {
		available := m.Servers.Len()
		count := runBest
		if count > available {
			count = available
		}
		return m.Servers.Raw()[:count]
	}

	// --servers path: parse and resolve each ID.
	ids := splitServerIDs(runServerIDs)
	servers := make([]*speedtest.Server, 0, len(ids))
	for _, id := range ids {
		idx, found := m.Servers.FindIndex(id)
		if !found {
			exitWithError("server %s not found", id)
		}
		servers = append(servers, m.Servers.Raw()[idx])
	}
	return servers
}

// runMultiServerHeadless is the multi-server CLI execution path for --best and --servers.
func runMultiServerHeadless(m *model.Model, interactive bool) {
	servers := resolveMultiServers(m, interactive)

	opts := model.RunOptions{
		SkipDownload: runNoDownload,
		SkipUpload:   runNoUpload,
	}
	if interactive {
		opts.ProgressFn = func(phase string) {
			fmt.Fprintf(os.Stderr, "  %s\n", phase)
		}
	}

	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	totalTimeout := m.Config.TestTimeoutDuration() * time.Duration(len(servers))
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	results, serverErrors := m.RunMultiServerHeadless(ctx, servers, opts)

	for _, se := range serverErrors {
		fmt.Fprintf(os.Stderr, "Warning: server %q failed: %v\n", se.ServerName, se.Err)
	}

	if len(results) == 0 {
		exitWithError("all server tests failed")
	}

	switch {
	case runJSON:
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			exitWithError("serialising results: %v", err)
		}
		fmt.Println(string(data))
	case runCSV:
		rows := make([][]string, len(results))
		for i, res := range results {
			rows[i] = res.CSVRow()
		}
		writeCSVRows(model.SpeedTestCSVHeader, rows)
	case runSimple:
		for _, res := range results {
			fmt.Println(formatSimpleResult(res))
		}
	default:
		fmt.Println(formatComparisonTable(results))
	}
}

// bestMetrics identifies the best (highest DL/UL, lowest Ping/Jitter) result indices.
// When all values are identical the returned index is -1 so no star is emitted.
type bestMetrics struct {
	downloadIdx int
	uploadIdx   int
	pingIdx     int
	jitterIdx   int
}

// findBestMetrics scans results and returns the index of the best value for each metric.
// Returns -1 for a metric when all values are identical (no winner to highlight).
func findBestMetrics(results []*model.SpeedTestResult) bestMetrics {
	bm := bestMetrics{downloadIdx: 0, uploadIdx: 0, pingIdx: 0, jitterIdx: 0}
	allDLEqual := true
	allULEqual := true
	allPingEqual := true
	allJitterEqual := true

	for i, res := range results {
		if i == 0 {
			continue
		}
		if res.DownloadSpeed != results[0].DownloadSpeed {
			allDLEqual = false
		}
		if res.UploadSpeed != results[0].UploadSpeed {
			allULEqual = false
		}
		if res.Ping != results[0].Ping {
			allPingEqual = false
		}
		if res.Jitter != results[0].Jitter {
			allJitterEqual = false
		}

		if res.DownloadSpeed > results[bm.downloadIdx].DownloadSpeed {
			bm.downloadIdx = i
		}
		if res.UploadSpeed > results[bm.uploadIdx].UploadSpeed {
			bm.uploadIdx = i
		}
		if res.Ping < results[bm.pingIdx].Ping {
			bm.pingIdx = i
		}
		if res.Jitter < results[bm.jitterIdx].Jitter {
			bm.jitterIdx = i
		}
	}

	if allDLEqual {
		bm.downloadIdx = -1
	}
	if allULEqual {
		bm.uploadIdx = -1
	}
	if allPingEqual {
		bm.pingIdx = -1
	}
	if allJitterEqual {
		bm.jitterIdx = -1
	}

	return bm
}

// formatComparisonTable renders a fixed-width comparison table for multiple server results.
// Best-value rows are marked with a star: higher is better for DL/UL, lower for Ping/Jitter.
func formatComparisonTable(results []*model.SpeedTestResult) string {
	const (
		colServer  = 20
		colCountry = 10
		colNum     = 10
	)

	header := fmt.Sprintf("%-*s  %-*s  %*s  %*s  %*s  %*s",
		colServer, "SERVER",
		colCountry, "COUNTRY",
		colNum, "DL (Mbps)",
		colNum, "UL (Mbps)",
		colNum, "PING (ms)",
		colNum, "JITTER (ms)",
	)
	separator := strings.Repeat("─", len(header))

	bm := findBestMetrics(results)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteByte('\n')
	sb.WriteString(separator)
	sb.WriteByte('\n')

	for i, res := range results {
		hasStar := i == bm.downloadIdx || i == bm.uploadIdx || i == bm.pingIdx || i == bm.jitterIdx

		star := ""
		if hasStar {
			star = comparisonStarMarker
		}

		row := fmt.Sprintf("%-*s  %-*s  %*.2f  %*.2f  %*.2f  %*.2f%s",
			colServer, ui.Truncate(res.ServerName, comparisonServerNameMaxLen),
			colCountry, res.ServerCountry,
			colNum, res.DownloadSpeed,
			colNum, res.UploadSpeed,
			colNum, res.Ping,
			colNum, res.Jitter,
			star,
		)
		sb.WriteString(row)
		sb.WriteByte('\n')
	}

	return sb.String()
}

func runHeadlessTest() {
	m := model.NewDefaultModel()
	interactive := runIsInteractive()

	if runBest > 0 || runServerIDs != "" {
		runMultiServerHeadless(m, interactive)
		return
	}

	serverIdx := prepareRunServer(m, runServerID, interactive)
	server := m.Servers.Raw()[serverIdx]

	opts := model.RunOptions{
		SkipDownload: runNoDownload,
		SkipUpload:   runNoUpload,
	}
	if interactive {
		opts.ProgressFn = func(phase string) {
			fmt.Fprintf(os.Stderr, "  %s\n", phase)
		}
	}

	// Load before the loop so results from each iteration accumulate correctly.
	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	var jsonResults []*model.SpeedTestResult
	var csvRows [][]string

	for i := range runCount {
		if runCount > 1 && !runJSON && !runCSV {
			fmt.Printf("\n--- Test %d of %d ---\n", i+1, runCount)
		}

		testCtx, testCancel := context.WithTimeout(context.Background(), m.Config.TestTimeoutDuration())
		res, err := m.RunHeadless(testCtx, server, opts)
		testCancel()
		if err != nil {
			exitWithError("running test: %v", err)
		}

		if m.Warning != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", m.Warning)
			m.Warning = ""
		}

		m.History.Append(res)
		if err := m.History.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save history: %v\n", err)
		}

		switch {
		case runJSON:
			jsonResults = append(jsonResults, res)
		case runCSV:
			csvRows = append(csvRows, res.CSVRow())
		case runSimple:
			fmt.Println(formatSimpleResult(res))
		default:
			fmt.Println(formatDefaultResult(res))
		}
	}

	writeRunResults(jsonResults, csvRows, runJSON, runCSV)
}

// formatSimpleResult formats a speed test result as a one-line string.
func formatSimpleResult(res *model.SpeedTestResult) string {
	return fmt.Sprintf("Download: %.2f Mbps | Upload: %.2f Mbps | Ping: %.2f ms", res.DownloadSpeed, res.UploadSpeed, res.Ping)
}

// formatDefaultResult formats a speed test result for terminal output.
func formatDefaultResult(res *model.SpeedTestResult) string {
	return fmt.Sprintf("\nDownload  %.2f Mbps\nUpload    %.2f Mbps\nPing      %.2f ms\nJitter    %.2f ms",
		res.DownloadSpeed, res.UploadSpeed, res.Ping, res.Jitter)
}

// marshalJSONResults serialises speed test results for --json output.
// A single result is marshalled as a bare JSON object to preserve
// backward-compatibility with existing scripts.
// Multiple results are marshalled as a JSON array so the output is
// always valid JSON regardless of --count.
func marshalJSONResults(results []*model.SpeedTestResult) ([]byte, error) {
	switch len(results) {
	case 0:
		return []byte("[]"), nil
	case 1:
		return json.MarshalIndent(results[0], "", "  ")
	default:
		return json.MarshalIndent(results, "", "  ")
	}
}
