// cmd_analytics.go
package main

import (
	"fmt"
	"strings"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/spf13/cobra"
)

var (
	analyticsJSON   bool
	analyticsSimple bool
	analyticsLast   int
)

var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "Show speed test analytics and trends",
	Long:  `Compute and display trends, averages, and peak/off-peak comparisons from test history.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if analyticsLast < 0 {
			return fmt.Errorf("--last must be >= 0, got %d", analyticsLast)
		}
		runAnalytics()
		return nil
	},
}

func runAnalytics() {
	m := model.NewDefaultModel()
	if err := m.History.Load(); err != nil {
		exitWithError("loading history: %v", err)
	}

	entries := m.History.Entries
	if len(entries) == 0 {
		fmt.Println("No test data yet. Run a speed test first.")
		return
	}

	if analyticsLast > 0 && analyticsLast < len(entries) {
		entries = entries[len(entries)-analyticsLast:]
	}

	summary := model.ComputeSummary(entries)

	if analyticsJSON {
		printJSON(summary)
		return
	}

	if analyticsSimple {
		fmt.Println(analyticsSimpleLine(summary))
		return
	}

	fmt.Println(analyticsDefaultOutput(summary))
}

func analyticsSimpleLine(s *model.Summary) string {
	return fmt.Sprintf("DL: %.1f avg (%s) | UL: %.1f avg (%s) | Ping: %.1f avg (%s)",
		s.Download.Average, s.Download.TrendLabel(),
		s.Upload.Average, s.Upload.TrendLabel(),
		s.Ping.Average, s.Ping.TrendLabel())
}

func analyticsDefaultOutput(s *model.Summary) string {
	dateFrom := s.DateRange[0].Format("Jan 2")
	dateTo := s.DateRange[1].Format("Jan 2")

	var b strings.Builder
	fmt.Fprintf(&b, "Analytics (%d tests, %s - %s)\n\n", s.TotalTests, dateFrom, dateTo)
	fmt.Fprintf(&b, "Download  %s  %.1f Mbps avg  %s\n", s.Download.Sparkline, s.Download.Average, s.Download.TrendLabel())
	fmt.Fprintf(&b, "Upload    %s  %.1f Mbps avg  %s\n", s.Upload.Sparkline, s.Upload.Average, s.Upload.TrendLabel())
	fmt.Fprintf(&b, "Ping      %s  %.1f ms avg    %s\n", s.Ping.Sparkline, s.Ping.Average, s.Ping.TrendLabel())

	if s.TotalTests >= 2 {
		fmt.Fprintf(&b, "\nPeak (%02d-%02d)     DL: %.1f Mbps  UL: %.1f Mbps  (%d tests)\n",
			model.PeakStartHour, model.PeakEndHour,
			s.PeakDownload.PeakAvg, s.PeakUpload.PeakAvg, s.PeakDownload.PeakCount)
		fmt.Fprintf(&b, "Off-Peak (%02d-%02d) DL: %.1f Mbps  UL: %.1f Mbps  (%d tests)",
			model.PeakEndHour, model.PeakStartHour,
			s.PeakDownload.OffPeakAvg, s.PeakUpload.OffPeakAvg, s.PeakDownload.OffPeakCount)
	}

	return b.String()
}

func init() {
	analyticsCmd.Flags().BoolVar(&analyticsJSON, "json", false, "Output as JSON")
	analyticsCmd.Flags().BoolVar(&analyticsSimple, "simple", false, "Minimal one-line output")
	analyticsCmd.Flags().IntVar(&analyticsLast, "last", 0, "Analyze only the last N results (0 = all)")

	rootCmd.AddCommand(analyticsCmd)
}
