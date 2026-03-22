package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/spf13/cobra"
)

const (
	historyFormatJSON = "json"
	historyFormatCSV  = "csv"
)

var (
	historyClear  bool
	historyFormat string
	historyLast   int
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View or export test history",
	RunE: func(_ *cobra.Command, _ []string) error {
		if historyFormat != "" && historyFormat != historyFormatJSON && historyFormat != historyFormatCSV {
			return fmt.Errorf("invalid --format %q: must be %q or %q", historyFormat, historyFormatJSON, historyFormatCSV)
		}
		if historyLast < 0 {
			return fmt.Errorf("--last must be >= 0, got %d", historyLast)
		}
		runHistory()
		return nil
	},
}

func runHistory() {
	m := model.NewDefaultModel()

	if historyClear {
		// Wipe history by setting to empty and saving
		m.TestHistory = []*model.SpeedTestResult{}
		if err := m.SaveHistory(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing history: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("History cleared.")
		return
	}

	if err := m.LoadHistory(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading history: %v\n", err)
		os.Exit(1)
	}

	if len(m.TestHistory) == 0 {
		fmt.Println("No history found.")
		return
	}

	// Apply --last slice: take the last N entries
	entries := m.TestHistory
	if historyLast > 0 && historyLast < len(entries) {
		entries = entries[len(entries)-historyLast:]
	}

	switch historyFormat {
	case historyFormatJSON:
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serialising history: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))

	case historyFormatCSV:
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"timestamp", "server", "country", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "ip", "isp"})
		for _, res := range entries {
			_ = w.Write([]string{
				res.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				res.ServerName,
				res.ServerCountry,
				fmt.Sprintf("%.2f", res.DownloadSpeed),
				fmt.Sprintf("%.2f", res.UploadSpeed),
				fmt.Sprintf("%.2f", res.Ping),
				fmt.Sprintf("%.2f", res.Jitter),
				res.UserIP,
				res.UserISP,
			})
		}
		w.Flush()

	default:
		// Default: table view
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "DATE\tSERVER\tDL (Mbps)\tUL (Mbps)\tPING (ms)")
		for _, res := range entries {
			dateStr := res.Timestamp.Format("2006-01-02 15:04")
			serverStr := res.ServerName
			if len(serverStr) > 20 {
				serverStr = serverStr[:17] + "..."
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%.2f\n", dateStr, serverStr, res.DownloadSpeed, res.UploadSpeed, res.Ping)
		}
		_ = w.Flush()
	}
}

func init() {
	historyCmd.Flags().BoolVar(&historyClear, "clear", false, "Clear all history")
	historyCmd.Flags().StringVar(&historyFormat, "format", "", "Output format: json or csv (default: table)")
	historyCmd.Flags().IntVar(&historyLast, "last", 0, "Limit output to the last N results (0 = all)")
	rootCmd.AddCommand(historyCmd)
}
