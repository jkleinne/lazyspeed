package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/spf13/cobra"
)

var historyClear bool

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View test history",
	Run: func(_ *cobra.Command, _ []string) {
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

		// Print history
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tSERVER\tDL (MBps)\tUL (MBps)\tPING (ms)")
		for _, res := range m.TestHistory {
			dateStr := res.Timestamp.Format("2006-01-02 15:04")
			serverStr := res.ServerName
			if len(serverStr) > 20 {
				serverStr = serverStr[:17] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t%.2f\n", dateStr, serverStr, res.DownloadSpeed, res.UploadSpeed, res.Ping)
		}
		w.Flush()
	},
}

func init() {
	historyCmd.Flags().BoolVar(&historyClear, "clear", false, "Clear all history")
	rootCmd.AddCommand(historyCmd)
}
