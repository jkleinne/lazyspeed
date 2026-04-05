package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	runFavorites  bool
	runWatch      time.Duration
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a speed test non-interactively",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if runWatch == 0 && runCount < 1 {
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
		if runFavorites {
			if runServerID != "" {
				return fmt.Errorf("--favorites and --server are mutually exclusive")
			}
			if runServerIDs != "" {
				return fmt.Errorf("--favorites and --servers are mutually exclusive")
			}
			if runBest > 0 {
				return fmt.Errorf("--favorites and --best are mutually exclusive")
			}
			if runCount > 1 {
				return fmt.Errorf("--favorites and --count are mutually exclusive")
			}
		}
		if runWatch > 0 {
			if runWatch < time.Minute {
				return fmt.Errorf("--watch interval must be at least 1m, got %s", runWatch)
			}
			if runBest > 0 {
				return fmt.Errorf("--watch and --best are mutually exclusive")
			}
			if runServerIDs != "" {
				return fmt.Errorf("--watch and --servers are mutually exclusive")
			}
			if runFavorites {
				return fmt.Errorf("--watch and --favorites are mutually exclusive")
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
	runCmd.Flags().BoolVar(&runFavorites, "favorites", false, "Test all favorited servers (multi-server comparison)")
	runCmd.Flags().DurationVar(&runWatch, "watch", 0, "Repeat tests on an interval (e.g., 5m, 1h); minimum 1m")

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

	if runFavorites {
		favIDs := m.Config.Servers.FavoriteIDs
		if len(favIDs) == 0 {
			exitWithError("no favorites configured; use 'lazyspeed servers --pin <id>' to add favorites")
		}
		servers := make([]*speedtest.Server, 0, len(favIDs))
		for _, id := range favIDs {
			idx, found := m.Servers.FindIndex(id)
			if found {
				servers = append(servers, m.Servers.Raw()[idx])
			}
		}
		if len(servers) == 0 {
			exitWithError("none of the favorited servers were found in the server list")
		}
		return servers
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
		opts.ProgressFn = interactiveProgressFn()
	}

	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	totalTimeout := m.Config.TestTimeoutDuration() * time.Duration(len(servers))
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	results, serverErrors := m.RunMultiServerHeadless(ctx, servers, opts)
	if interactive {
		fmt.Fprint(os.Stderr, "\n")
	}

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

// formatComparisonTable renders a fixed-width comparison table for multiple server results.
// Best-value rows are marked with a star: higher is better for DL/UL, lower for Ping/Jitter.
func formatComparisonTable(results []*model.SpeedTestResult) string {
	const (
		colServer  = 20
		colSponsor = 20
		colDist    = 10
		colNum     = 10
	)

	var sb strings.Builder

	if len(results) > 0 {
		first := results[0]
		fmt.Fprintf(&sb, "IP: %s (%s)\n\n", first.UserIP, first.UserISP)
	}

	header := fmt.Sprintf("%-*s  %-*s  %*s  %*s  %*s  %*s  %*s",
		colServer, "SERVER",
		colSponsor, "SPONSOR",
		colDist, "DIST (km)",
		colNum, "DL (Mbps)",
		colNum, "UL (Mbps)",
		colNum, "PING (ms)",
		colNum, "JITTER (ms)",
	)
	separator := strings.Repeat("─", len(header))

	bm := model.FindBestMetrics(results)

	sb.WriteString(header)
	sb.WriteByte('\n')
	sb.WriteString(separator)
	sb.WriteByte('\n')

	for i, res := range results {
		hasStar := i == bm.DownloadIdx || i == bm.UploadIdx || i == bm.PingIdx || i == bm.JitterIdx

		star := ""
		if hasStar {
			star = comparisonStarMarker
		}

		row := fmt.Sprintf("%-*s  %-*s  %*.2f  %*.2f  %*.2f  %*.2f  %*.2f%s",
			colServer, ui.Truncate(res.ServerName, comparisonServerNameMaxLen),
			colSponsor, ui.Truncate(res.ServerSponsor, comparisonServerNameMaxLen),
			colDist, res.Distance,
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

	if runBest > 0 || runServerIDs != "" || runFavorites {
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
		opts.ProgressFn = interactiveProgressFn()
	}

	// Load before the loop so results from each iteration accumulate correctly.
	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	if runWatch > 0 {
		runWatchLoop(m, server, opts, interactive)
		return
	}

	runCountLoop(m, server, opts, interactive)
}

// runCountLoop runs N sequential tests (the existing --count behavior).
func runCountLoop(m *model.Model, server *speedtest.Server, opts model.RunOptions, interactive bool) {
	var jsonResults []*model.SpeedTestResult
	var csvRows [][]string

	for i := range runCount {
		if runCount > 1 && !runJSON && !runCSV {
			fmt.Printf("\n--- Test %d of %d ---\n", i+1, runCount)
		}

		testCtx, testCancel := context.WithTimeout(context.Background(), m.Config.TestTimeoutDuration())
		res, err := m.RunHeadless(testCtx, server, opts)
		testCancel()
		if interactive {
			fmt.Fprint(os.Stderr, "\n")
		}
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

// runWatchLoop runs tests on a fixed interval until interrupted or --count is reached.
func runWatchLoop(m *model.Model, server *speedtest.Server, opts model.RunOptions, interactive bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	ticker := time.NewTicker(runWatch)
	defer ticker.Stop()

	// Print CSV header once at the start.
	if runCSV {
		writeCSVRows(model.SpeedTestCSVHeader, nil)
	}

	iteration := 0
	maxIterations := runCount // 0 means indefinite

	for {
		iteration++
		if maxIterations > 0 && iteration > maxIterations {
			return
		}

		if iteration > 1 && !runJSON && !runCSV {
			fmt.Print(formatWatchSeparator(iteration, time.Now()))
		}

		testCtx, testCancel := context.WithTimeout(context.Background(), m.Config.TestTimeoutDuration())

		// Allow a second signal to force-cancel a running test.
		done := make(chan struct{})
		go func() {
			select {
			case <-sigChan:
				testCancel()
			case <-done:
			}
		}()

		res, err := m.RunHeadless(testCtx, server, opts)
		close(done)
		testCancel()

		if interactive {
			fmt.Fprint(os.Stderr, "\n")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: running test: %v\n", err)
		} else {
			if m.Warning != "" {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", m.Warning)
				m.Warning = ""
			}

			m.History.Append(res)
			if err := m.History.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save history: %v\n", err)
			}

			emitWatchResult(res)
		}

		// Wait for next tick or signal.
		select {
		case <-sigChan:
			return
		case <-ticker.C:
		}
	}
}

// emitWatchResult writes a single result in the active output format.
// Unlike the --count path, structured output is emitted incrementally.
func emitWatchResult(res *model.SpeedTestResult) {
	switch {
	case runJSON:
		data, err := json.Marshal(res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: serialising result: %v\n", err)
			return
		}
		fmt.Println(string(data))
	case runCSV:
		writeCSVRows(nil, [][]string{res.CSVRow()})
	case runSimple:
		fmt.Println(formatSimpleResult(res))
	default:
		fmt.Println(formatDefaultResult(res))
	}
}

// formatWatchSeparator formats the separator printed between watch iterations.
func formatWatchSeparator(iteration int, ts time.Time) string {
	return fmt.Sprintf("\n--- Watch #%d at %s ---\n", iteration, ts.Format("15:04:05"))
}

// formatSimpleResult formats a speed test result as a one-line string.
func formatSimpleResult(res *model.SpeedTestResult) string {
	return fmt.Sprintf("Download: %.2f Mbps | Upload: %.2f Mbps | Ping: %.2f ms", res.DownloadSpeed, res.UploadSpeed, res.Ping)
}

// formatDefaultResult formats a speed test result for terminal output.
func formatDefaultResult(res *model.SpeedTestResult) string {
	return fmt.Sprintf("\nIP        %s (%s)\nServer    %s (%s) — %.2f km\n\nDownload  %.2f Mbps\nUpload    %.2f Mbps\nPing      %.2f ms\nJitter    %.2f ms",
		res.UserIP, res.UserISP,
		res.ServerName, res.ServerSponsor, res.Distance,
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
