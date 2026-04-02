// cmd_analytics.go
package main

import (
	"fmt"

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
		s.Download.Average, trendLabel(s.Download),
		s.Upload.Average, trendLabel(s.Upload),
		s.Ping.Average, trendLabel(s.Ping))
}

func trendLabel(m model.MetricSummary) string {
	switch m.Trend {
	case model.TrendUp:
		return fmt.Sprintf("↑%.1f%%", m.TrendPct)
	case model.TrendDown:
		return fmt.Sprintf("↓%.1f%%", -m.TrendPct)
	default:
		return "stable"
	}
}

func analyticsDefaultOutput(s *model.Summary) string {
	dateFrom := s.DateRange[0].Format("Jan 2")
	dateTo := s.DateRange[1].Format("Jan 2")

	out := fmt.Sprintf("Analytics (%d tests, %s - %s)\n\n", s.TotalTests, dateFrom, dateTo)
	out += fmt.Sprintf("Download  %s  %.1f Mbps avg  %s\n", s.Download.Sparkline, s.Download.Average, trendLabel(s.Download))
	out += fmt.Sprintf("Upload    %s  %.1f Mbps avg  %s\n", s.Upload.Sparkline, s.Upload.Average, trendLabel(s.Upload))
	out += fmt.Sprintf("Ping      %s  %.1f ms avg    %s\n", s.Ping.Sparkline, s.Ping.Average, trendLabel(s.Ping))

	if s.TotalTests >= 2 {
		out += fmt.Sprintf("\nPeak (09-21)     DL: %.1f Mbps  UL: %.1f Mbps  (%d tests)\n",
			s.PeakDownload.PeakAvg, s.PeakUpload.PeakAvg, s.PeakDownload.PeakCount)
		out += fmt.Sprintf("Off-Peak (21-09) DL: %.1f Mbps  UL: %.1f Mbps  (%d tests)",
			s.PeakDownload.OffPeakAvg, s.PeakUpload.OffPeakAvg, s.PeakDownload.OffPeakCount)
	}

	return out
}

func init() {
	analyticsCmd.Flags().BoolVar(&analyticsJSON, "json", false, "Output as JSON")
	analyticsCmd.Flags().BoolVar(&analyticsSimple, "simple", false, "Minimal one-line output")
	analyticsCmd.Flags().IntVar(&analyticsLast, "last", 0, "Analyze only the last N results (0 = all)")

	rootCmd.AddCommand(analyticsCmd)
}
