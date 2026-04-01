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

func runHeadlessTest() {
	m := model.NewDefaultModel()

	if runIsInteractive() {
		fmt.Println("Fetching server list...")
	}
	fetchServersOrExit(m)

	if m.Servers.Len() == 0 {
		fmt.Fprintf(os.Stderr, "Error: no servers found\n")
		os.Exit(1)
	}

	serverIdx := 0
	if runServerID != "" {
		idx, found := m.Servers.FindIndex(runServerID)
		if !found {
			fmt.Fprintf(os.Stderr, "Error: server %s not found\n", runServerID)
			os.Exit(1)
		}
		serverIdx = idx
	}
	server := m.Servers.Raw()[serverIdx]

	if runIsInteractive() {
		fmt.Printf("Selected server: %s (%s)\n", server.Name, server.Country)
	}

	opts := model.RunOptions{
		SkipDownload: runNoDownload,
		SkipUpload:   runNoUpload,
	}
	if runIsInteractive() {
		opts.ProgressFn = func(phase string) {
			fmt.Fprintf(os.Stderr, "  %s\n", phase)
		}
	}

	// Load history once before the loop so results accumulate correctly
	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load history: %v\n", err)
	}

	// Collect results for JSON mode so we can emit valid JSON after all runs.
	// (Printing one object per iteration produces invalid JSON when --count > 1.)
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
			fmt.Fprintf(os.Stderr, "Error running test: %v\n", err)
			os.Exit(1)
		}

		if m.Warning != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", m.Warning)
			m.Warning = ""
		}

		// Persist result to history
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

	// Emit JSON output after all runs so the result is always valid JSON.
	// --count 1 → bare object (backward-compatible with existing scripts).
	// --count N → JSON array.
	if runJSON {
		data, err := marshalJSONResults(jsonResults)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serialising results: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	}

	if runCSV {
		writeCSVRows(model.SpeedTestCSVHeader, csvRows)
	}
}

// formatSimpleResult formats a speed test result as a one-line string.
func formatSimpleResult(res *model.SpeedTestResult) string {
	return fmt.Sprintf("DL: %.2f Mbps | UL: %.2f Mbps | Ping: %.2f ms", res.DownloadSpeed, res.UploadSpeed, res.Ping)
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
