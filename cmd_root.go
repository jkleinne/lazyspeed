package main

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lazyspeed",
	Short: "A terminal-based internet speed test",
	Long:  `LazySpeed is a terminal-based internet speed test built in Go with a rich TUI.`,
	Run: func(_ *cobra.Command, _ []string) {
		// Default behavior is to launch the TUI
		runTUI()
	},
}

// flushCSV flushes the CSV writer and exits on error.
func flushCSV(w *csv.Writer) {
	w.Flush()
	if err := w.Error(); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
		os.Exit(1)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
