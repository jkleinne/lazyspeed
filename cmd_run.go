package main

import (
	"context"
	"encoding/csv"
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
	Run: func(_ *cobra.Command, _ []string) {
		runHeadlessTest()
	},
}

func init() {
	runCmd.Flags().BoolVar(&runJSON, "json", false, "Output results as JSON to stdout")
	runCmd.Flags().BoolVar(&runCSV, "csv", false, "Output results as CSV to stdout")
	runCmd.Flags().BoolVar(&runSimple, "simple", false, "Minimal output (one line: DL/UL/Ping)")
	runCmd.Flags().StringVar(&runServerID, "server", "", "Skip server selection, use a specific server ID")
	runCmd.Flags().BoolVar(&runNoUpload, "no-upload", false, "Skip upload phase")
	runCmd.Flags().BoolVar(&runNoDownload, "no-download", false, "Skip download phase")
	runCmd.Flags().IntVar(&runCount, "count", 1, "Run multiple tests sequentially")

	rootCmd.AddCommand(runCmd)
}

func runHeadlessTest() {
	for i := 0; i < runCount; i++ {
		if runCount > 1 && !runJSON && !runCSV {
			fmt.Printf("\n--- Test %d of %d ---\n", i+1, runCount)
		}

		m := model.NewDefaultModel()

		ctx := context.Background()

		if !runJSON && !runCSV && !runSimple {
			fmt.Println("Fetching server list...")
		}

		if err := m.FetchServerList(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching servers: %v\n", err)
			os.Exit(1)
		}

		if len(m.ServerList) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no servers found\n")
			os.Exit(1)
		}

		server := m.ServerList[0] // Auto-select fastest by default
		if runServerID != "" {
			found := false
			for _, s := range m.ServerList {
				if s.ID == runServerID {
					server = s
					found = true
					break
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "Error: server %s not found\n", runServerID)
				os.Exit(1)
			}
		}

		if !runJSON && !runCSV && !runSimple {
			fmt.Printf("Selected server: %s (%s)\n", server.Name, server.Country)
			fmt.Println("Running speed test...")
		}

		opts := model.RunOptions{
			SkipDownload: runNoDownload,
			SkipUpload:   runNoUpload,
		}

		res, err := m.RunHeadless(ctx, server, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running test: %v\n", err)
			os.Exit(1)
		}

		// Persist headless tests to history as well
		if err := m.LoadHistory(); err == nil {
			m.TestHistory = append(m.TestHistory, res)
			_ = m.SaveHistory() // ignore headless save errors
		}

		if runJSON {
			data, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(data))
		} else if runCSV {
			w := csv.NewWriter(os.Stdout)
			if i == 0 { // Write header only once
				_ = w.Write([]string{"timestamp", "server", "country", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "ip", "isp"})
			}
			_ = w.Write([]string{
				res.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				res.ServerName,
				res.ServerLoc,
				fmt.Sprintf("%.2f", res.DownloadSpeed),
				fmt.Sprintf("%.2f", res.UploadSpeed),
				fmt.Sprintf("%.2f", res.Ping),
				fmt.Sprintf("%.2f", res.Jitter),
				res.UserIP,
				res.UserISP,
			})
			w.Flush()
		} else if runSimple {
			fmt.Printf("DL: %.2f MBps | UL: %.2f MBps | Ping: %.2f ms\n", res.DownloadSpeed, res.UploadSpeed, res.Ping)
		} else {
			fmt.Printf("\n📥 Download: %.2f MBps\n", res.DownloadSpeed)
			fmt.Printf("📤 Upload: %.2f MBps\n", res.UploadSpeed)
			fmt.Printf("🔄 Ping: %.2f ms\n", res.Ping)
			fmt.Printf("📊 Jitter: %.2f ms\n", res.Jitter)
		}
	}
}
