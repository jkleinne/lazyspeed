package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/notify"
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

	// phaseResultSuffix is the suffix appended to CLI progress phases that
	// carry a speed result. The interactive progress function uses this to
	// detect when a newline should be emitted to preserve the result on screen.
	phaseResultSuffix = "Mbps"

	// Column widths for the headless comparison table.
	colServer  = 20
	colSponsor = 20
	colDist    = 10
	colNum     = 10
)

type runFlags struct {
	json       bool
	csv        bool
	simple     bool
	serverID   string
	noUpload   bool
	noDownload bool
	count      int
	best       int
	serverIDs  string
	favorites  bool
	watch      time.Duration
	webhookURL string
}

var runF runFlags

// webhookCfg holds the merged webhook configuration for the current headless run,
// combining config-file endpoints with any --webhook-url flag value.
// Initialized by runHeadlessTest before any sub-function reads it.
var webhookCfg model.WebhookConfig

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a speed test non-interactively",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if runF.watch == 0 && runF.count < 1 {
			return fmt.Errorf("--count must be at least 1, got %d", runF.count)
		}
		if runF.best < 0 {
			return fmt.Errorf("--best must be a positive number, got %d", runF.best)
		}
		if runF.best > 0 && runF.serverIDs != "" {
			return fmt.Errorf("--best and --servers are mutually exclusive")
		}
		if runF.best == 1 {
			return fmt.Errorf("--best must be at least 2, got %d", runF.best)
		}
		if runF.count > 1 && runF.best > 0 {
			return fmt.Errorf("--count and --best are mutually exclusive")
		}
		if runF.count > 1 && runF.serverIDs != "" {
			return fmt.Errorf("--count and --servers are mutually exclusive")
		}
		if runF.serverIDs != "" {
			ids := splitServerIDs(runF.serverIDs)
			if len(ids) < multiServerMinimum {
				return fmt.Errorf("--servers requires at least 2 server IDs, got %d", len(ids))
			}
		}
		if runF.favorites {
			if runF.serverID != "" {
				return fmt.Errorf("--favorites and --server are mutually exclusive")
			}
			if runF.serverIDs != "" {
				return fmt.Errorf("--favorites and --servers are mutually exclusive")
			}
			if runF.best > 0 {
				return fmt.Errorf("--favorites and --best are mutually exclusive")
			}
			if runF.count > 1 {
				return fmt.Errorf("--favorites and --count are mutually exclusive")
			}
		}
		if runF.watch > 0 {
			if runF.watch < time.Minute {
				return fmt.Errorf("--watch interval must be at least 1m, got %s", runF.watch)
			}
			if runF.best > 0 {
				return fmt.Errorf("--watch and --best are mutually exclusive")
			}
			if runF.serverIDs != "" {
				return fmt.Errorf("--watch and --servers are mutually exclusive")
			}
			if runF.favorites {
				return fmt.Errorf("--watch and --favorites are mutually exclusive")
			}
		}
		if runF.webhookURL != "" {
			parsed, err := url.Parse(runF.webhookURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				return fmt.Errorf("--webhook-url must be a valid http:// or https:// URL")
			}
		}
		return nil
	},
	Run: func(_ *cobra.Command, _ []string) {
		runHeadlessTest()
	},
}

func init() {
	runCmd.Flags().BoolVar(&runF.json, "json", false, "Output as JSON; single run emits a bare object, --count N>1 emits a JSON array")
	runCmd.Flags().BoolVar(&runF.csv, "csv", false, "Output results as CSV to stdout")
	runCmd.Flags().BoolVar(&runF.simple, "simple", false, "Minimal output (one line: DL/UL/Ping)")
	runCmd.Flags().StringVar(&runF.serverID, "server", "", "Skip server selection, use a specific server ID")
	runCmd.Flags().BoolVar(&runF.noUpload, "no-upload", false, "Skip upload phase")
	runCmd.Flags().BoolVar(&runF.noDownload, "no-download", false, "Skip download phase")
	runCmd.Flags().IntVar(&runF.count, "count", 1, "Run multiple tests sequentially")
	runCmd.Flags().IntVar(&runF.best, "best", 0, "Auto-select the N closest servers for comparison (minimum 2)")
	runCmd.Flags().StringVar(&runF.serverIDs, "servers", "", "Test specific servers by ID (comma-separated, minimum 2)")
	runCmd.Flags().BoolVar(&runF.favorites, "favorites", false, "Test all favorited servers (multi-server comparison)")
	runCmd.Flags().DurationVar(&runF.watch, "watch", 0, "Repeat tests on an interval (e.g., 5m, 1h); minimum 1m")
	runCmd.Flags().StringVar(&runF.webhookURL, "webhook-url", "", "POST results to this URL (additive to config webhooks)")

	rootCmd.AddCommand(runCmd)
}

func runIsInteractive() bool {
	return !runF.json && !runF.csv && !runF.simple
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
		writeCSVRows(model.SpeedTestCSVHeader(), csvRows)
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

	if runF.best > 0 {
		available := m.Servers.Len()
		count := runF.best
		if count > available {
			count = available
		}
		return m.Servers.Raw()[:count]
	}

	if runF.favorites {
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
	ids := splitServerIDs(runF.serverIDs)
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
		SkipDownload: runF.noDownload,
		SkipUpload:   runF.noUpload,
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

	csvRows := make([][]string, len(results))
	for i, res := range results {
		csvRows[i] = res.CSVRow()
	}
	format := resolveFormat(runF.json, runF.csv)
	formatOutput(format, results, model.SpeedTestCSVHeader(), csvRows, func() {
		if runF.simple {
			for _, res := range results {
				fmt.Println(formatSimpleResult(res))
			}
		} else {
			fmt.Println(formatComparisonTable(results))
		}
	})

	for _, res := range results {
		dispatchWebhooks(context.Background(), webhookCfg, res)
	}
}

// formatComparisonRow formats a single row for the headless comparison table.
func formatComparisonRow(res *model.SpeedTestResult, isBest bool) string {
	star := ""
	if isBest {
		star = comparisonStarMarker
	}
	return fmt.Sprintf("%-*s  %-*s  %*.2f  %*.2f  %*.2f  %*.2f  %*.2f%s",
		colServer, ui.Truncate(res.ServerName, comparisonServerNameMaxLen),
		colSponsor, ui.Truncate(res.ServerSponsor, comparisonServerNameMaxLen),
		colDist, res.Distance,
		colNum, res.DownloadSpeed,
		colNum, res.UploadSpeed,
		colNum, res.Ping,
		colNum, res.Jitter,
		star,
	)
}

// formatComparisonTable renders a fixed-width comparison table for multiple server results.
// Best-value rows are marked with a star: higher is better for DL/UL, lower for Ping/Jitter.
func formatComparisonTable(results []*model.SpeedTestResult) string {
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
		isBest := i == bm.DownloadIdx || i == bm.UploadIdx || i == bm.PingIdx || i == bm.JitterIdx
		sb.WriteString(formatComparisonRow(res, isBest))
		sb.WriteByte('\n')
	}

	return sb.String()
}

func runHeadlessTest() {
	m := model.NewDefaultModel()
	interactive := runIsInteractive()

	webhookCfg = m.Config.Webhooks
	if runF.webhookURL != "" {
		webhookCfg.Endpoints = append(
			slices.Clone(webhookCfg.Endpoints),
			model.WebhookEndpoint{URL: runF.webhookURL},
		)
	}

	if runF.best > 0 || runF.serverIDs != "" || runF.favorites {
		runMultiServerHeadless(m, interactive)
		return
	}

	serverIdx := prepareRunServer(m, runF.serverID, interactive)
	server := m.Servers.Raw()[serverIdx]

	opts := model.RunOptions{
		SkipDownload: runF.noDownload,
		SkipUpload:   runF.noUpload,
	}
	if interactive {
		opts.ProgressFn = interactiveProgressFn()
	}

	// Load before the loop so results from each iteration accumulate correctly.
	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	if runF.watch > 0 {
		runWatchLoop(m, server, opts, interactive)
		return
	}

	runCountLoop(m, server, opts, interactive)
}

// recordResult prints any pending model warning and persists the result to history.
func recordResult(m *model.Model, res *model.SpeedTestResult) {
	if m.Warning != "" {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", m.Warning)
		m.Warning = ""
	}
	m.History.Append(res)
	if saveErr := m.History.Save(); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save history: %v\n", saveErr)
	}
}

// dispatchWebhooks sends the result to configured webhook endpoints.
// Errors are printed as warnings to stderr and do not affect the test result.
func dispatchWebhooks(ctx context.Context, cfg model.WebhookConfig, result *model.SpeedTestResult) {
	if len(cfg.Endpoints) == 0 {
		return
	}
	client := &http.Client{}
	errs := notify.Dispatch(ctx, client, cfg, result, version)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: webhook delivery to %s failed: %v\n", e.URL, e.Err)
	}
}

// runSingleIteration runs one headless test with a timeout, records the result,
// and returns it. Used by runCountLoop where each iteration is independent.
func runSingleIteration(m *model.Model, server *speedtest.Server, opts model.RunOptions, interactive bool) (*model.SpeedTestResult, error) {
	testCtx, testCancel := context.WithTimeout(context.Background(), m.Config.TestTimeoutDuration())
	res, err := m.RunHeadless(testCtx, server, opts)
	testCancel()
	if interactive {
		fmt.Fprint(os.Stderr, "\n")
	}
	if err != nil {
		return nil, err
	}
	recordResult(m, res)
	return res, nil
}

// runWatchIteration runs one headless test with a timeout. A second signal on
// sigChan cancels the in-flight test immediately, allowing graceful shutdown.
// Unlike runSingleIteration, this does not record the result or print
// interactive output — callers must handle both via recordResult.
func runWatchIteration(m *model.Model, server *speedtest.Server, opts model.RunOptions, sigChan <-chan os.Signal) (*model.SpeedTestResult, error) {
	testCtx, testCancel := context.WithTimeout(context.Background(), m.Config.TestTimeoutDuration())
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
	return res, err
}

// runCountLoop runs N sequential tests (the existing --count behavior).
func runCountLoop(m *model.Model, server *speedtest.Server, opts model.RunOptions, interactive bool) {
	var jsonResults []*model.SpeedTestResult
	var csvRows [][]string

	for i := range runF.count {
		if runF.count > 1 && !runF.json && !runF.csv {
			fmt.Printf("\n--- Test %d of %d ---\n", i+1, runF.count)
		}

		res, err := runSingleIteration(m, server, opts, interactive)
		if err != nil {
			exitWithError("running test: %v", err)
		}

		dispatchWebhooks(context.Background(), webhookCfg, res)

		switch {
		case runF.json:
			jsonResults = append(jsonResults, res)
		case runF.csv:
			csvRows = append(csvRows, res.CSVRow())
		case runF.simple:
			fmt.Println(formatSimpleResult(res))
		default:
			fmt.Println(formatDefaultResult(res))
		}
	}

	writeRunResults(jsonResults, csvRows, runF.json, runF.csv)
}

// runWatchLoop runs tests on a fixed interval until interrupted or --count is reached.
func runWatchLoop(m *model.Model, server *speedtest.Server, opts model.RunOptions, interactive bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	ticker := time.NewTicker(runF.watch)
	defer ticker.Stop()

	if runF.csv {
		writeCSVRows(model.SpeedTestCSVHeader(), nil)
	}

	iteration := 0
	maxIterations := runF.count // 0 means indefinite

	for {
		iteration++
		if maxIterations > 0 && iteration > maxIterations {
			return
		}

		if iteration > 1 && !runF.json && !runF.csv {
			fmt.Print(formatWatchSeparator(iteration, time.Now()))
		}

		res, err := runWatchIteration(m, server, opts, sigChan)
		if interactive {
			fmt.Fprint(os.Stderr, "\n")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: running test: %v\n", err)
		} else {
			recordResult(m, res)
			dispatchWebhooks(context.Background(), webhookCfg, res)
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
	case runF.json:
		data, err := json.Marshal(res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: serialising result: %v\n", err)
			return
		}
		fmt.Println(string(data))
	case runF.csv:
		writeCSVRows(nil, [][]string{res.CSVRow()})
	case runF.simple:
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
