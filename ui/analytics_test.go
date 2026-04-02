package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/jkleinne/lazyspeed/model"
)

// No TestMain or init() needed — ui_test.go already has TestMain
// that sets lipgloss.SetHasDarkBackground and SetColorProfile for the package.

func TestRenderAnalytics(t *testing.T) {
	t.Run("nil summary shows no data message", func(t *testing.T) {
		got := RenderAnalytics(nil, 80)
		plain := ansi.Strip(got)
		if !strings.Contains(plain, "No test data yet") {
			t.Errorf("expected 'No test data yet' message, got %q", plain)
		}
	})

	t.Run("summary renders sparklines and averages", func(t *testing.T) {
		summary := &model.Summary{
			Download: model.MetricSummary{
				Average:   94.2,
				Sparkline: "▃▄▅▆▇",
				Trend:     model.TrendUp,
				TrendPct:  12.3,
			},
			Upload: model.MetricSummary{
				Average:   38.7,
				Sparkline: "▂▃▃▄▅",
				Trend:     model.TrendDown,
				TrendPct:  -3.1,
			},
			Ping: model.MetricSummary{
				Average:   12.4,
				Sparkline: "▆▅▄▃▂",
				Trend:     model.TrendStable,
			},
			PeakDownload: model.PeakComparison{
				PeakAvg: 82.3, OffPeakAvg: 103.1, PeakCount: 14, OffPeakCount: 10,
			},
			PeakUpload: model.PeakComparison{
				PeakAvg: 35.1, OffPeakAvg: 41.8, PeakCount: 14, OffPeakCount: 10,
			},
			TotalTests: 24,
			DateRange: [2]time.Time{
				time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
			},
		}

		got := RenderAnalytics(summary, 80)
		plain := ansi.Strip(got)

		checks := []string{
			"Analytics",
			"24 tests",
			"▃▄▅▆▇", // download sparkline
			"94.2",  // download avg
			"▂▃▃▄▅", // upload sparkline
			"38.7",  // upload avg
			"▆▅▄▃▂", // ping sparkline
			"12.4",  // ping avg
			"Peak",
			"Off-Peak",
		}
		for _, want := range checks {
			if !strings.Contains(plain, want) {
				t.Errorf("output missing %q", want)
			}
		}
	})

	t.Run("single test shows insufficient data message", func(t *testing.T) {
		summary := &model.Summary{
			Download:   model.MetricSummary{Average: 100, Sparkline: "▄"},
			Upload:     model.MetricSummary{Average: 50, Sparkline: "▄"},
			Ping:       model.MetricSummary{Average: 10, Sparkline: "▄"},
			TotalTests: 1,
			DateRange: [2]time.Time{
				time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			},
		}

		got := RenderAnalytics(summary, 80)
		plain := ansi.Strip(got)
		if !strings.Contains(plain, "Run more tests") {
			t.Errorf("expected 'Run more tests' message for single test, got %q", plain)
		}
	})
}
