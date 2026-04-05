package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/spf13/cobra"
)

const historyServerMaxLen = 20

type historyFlags struct {
	clear  bool
	format string
	last   int
}

var historyF historyFlags

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View or export test history",
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := validateFormat(historyF.format); err != nil {
			return err
		}
		if historyF.last < 0 {
			return fmt.Errorf("--last must be >= 0, got %d", historyF.last)
		}
		runHistory()
		return nil
	},
}

func runHistory() {
	m := model.NewDefaultModel()

	if historyF.clear {
		// Wipe history by setting to empty and saving
		m.History.Entries = nil
		m.History.Results = nil
		if err := m.History.Save(); err != nil {
			exitWithError("clearing history: %v", err)
		}
		fmt.Println("History cleared.")
		return
	}

	if err := m.History.Load(); err != nil {
		exitWithError("loading history: %v", err)
	}

	if len(m.History.Entries) == 0 {
		fmt.Println("No history found.")
		return
	}

	// Apply --last slice: take the last N entries
	entries := tailSlice(m.History.Entries, historyF.last)

	format := resolveFormatString(historyF.format)
	csvRows := make([][]string, len(entries))
	for i, res := range entries {
		csvRows[i] = res.CSVRow()
	}
	formatOutput(format, entries, model.SpeedTestCSVHeader(), csvRows, func() {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "DATE\tSERVER\tDL (Mbps)\tUL (Mbps)\tPING (ms)")
		for _, res := range entries {
			dateStr := res.Timestamp.Format("2006-01-02 15:04")
			serverStr := ui.Truncate(res.ServerName, historyServerMaxLen)
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\n", dateStr, serverStr, res.DownloadSpeed, res.UploadSpeed, res.Ping)
		}
		_ = tw.Flush()
	})
}

func init() {
	historyCmd.Flags().BoolVar(&historyF.clear, "clear", false, "Clear all history")
	historyCmd.Flags().StringVar(&historyF.format, "format", "", "Output format: json or csv (default: table)")
	historyCmd.Flags().IntVar(&historyF.last, "last", 0, "Limit output to the last N results (0 = all)")
	rootCmd.AddCommand(historyCmd)
}
