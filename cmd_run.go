package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/spf13/cobra"
)

var (
	runJSON       bool
	runCSV        bool
	runSimple     bool
	runServerID   string
	runNoUpload   bool
	runNoDownload bool
	runCount      int
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a speed test non-interactively",
	PreRunE: func(_ *cobra.Command, _ []string) error {
		if runCount < 1 {
			return fmt.Errorf("--count must be at least 1, got %d", runCount)
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

	rootCmd.AddCommand(runCmd)
}

func runIsInteractive() bool {
	return !runJSON && !runCSV && !runSimple
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

func runHeadlessTest() {
	m := model.NewDefaultModel()
	interactive := runIsInteractive()
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
