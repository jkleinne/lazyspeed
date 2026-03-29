package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/spf13/cobra"
)

const (
	historyFormatJSON   = "json"
	historyFormatCSV    = "csv"
	historyServerMaxLen = 20
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
		m.History.Entries = nil
		m.History.Results = nil
		if err := m.History.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing history: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("History cleared.")
		return
	}

	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading history: %v\n", err)
		os.Exit(1)
	}

	if len(m.History.Entries) == 0 {
		fmt.Println("No history found.")
		return
	}

	// Apply --last slice: take the last N entries
	entries := m.History.Entries
	if historyLast > 0 && historyLast < len(entries) {
		entries = entries[len(entries)-historyLast:]
	}

	switch historyFormat {
	case historyFormatJSON:
		printJSON(entries)

	case historyFormatCSV:
		rows := make([][]string, len(entries))
		for i, res := range entries {
			rows[i] = res.CSVRow()
		}
		writeCSVRows(model.SpeedTestCSVHeader, rows)

	default:
		// Default: table view
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "DATE\tSERVER\tDL (Mbps)\tUL (Mbps)\tPING (ms)")
		for _, res := range entries {
			dateStr := res.Timestamp.Format("2006-01-02 15:04")
			serverStr := ui.Truncate(res.ServerName, historyServerMaxLen)
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\n", dateStr, serverStr, res.DownloadSpeed, res.UploadSpeed, res.Ping)
		}
		_ = tw.Flush()
	}
}

func init() {
	historyCmd.Flags().BoolVar(&historyClear, "clear", false, "Clear all history")
	historyCmd.Flags().StringVar(&historyFormat, "format", "", "Output format: json or csv (default: table)")
	historyCmd.Flags().IntVar(&historyLast, "last", 0, "Limit output to the last N results (0 = all)")
	rootCmd.AddCommand(historyCmd)
}
