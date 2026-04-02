package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jkleinne/lazyspeed/model"
)

const (
	analyticsBarMaxWidth = 25
)

// trendArrow returns a styled trend indicator string.
func trendArrow(trend model.TrendDirection, pct float64) string {
	switch trend {
	case model.TrendUp:
		return latencyGreenStyle.Render(fmt.Sprintf("↑ %.1f%%", pct))
	case model.TrendDown:
		return latencyRedStyle.Render(fmt.Sprintf("↓ %.1f%%", -pct))
	default:
		return dimStyle.Render("stable")
	}
}

// renderBar draws a horizontal bar scaled to maxVal.
func renderBar(value, maxVal float64, width int) string {
	if maxVal <= 0 {
		return ""
	}
	barLen := int(value / maxVal * float64(width))
	if barLen < 1 && value > 0 {
		barLen = 1
	}
	return metricValueStyle.Render(strings.Repeat("━", barLen))
}

// renderPeakSection renders a peak vs off-peak comparison for a metric.
func renderPeakSection(pc model.PeakComparison, unit string, width int) string {
	var b strings.Builder

	maxVal := max(pc.PeakAvg, pc.OffPeakAvg)
	barWidth := min(analyticsBarMaxWidth, width/3)

	peakLabel := hintDescStyle.Render(fmt.Sprintf("  %-10s", "Peak"))
	offLabel := hintDescStyle.Render(fmt.Sprintf("  %-10s", "Off-Peak"))

	if pc.PeakCount > 0 {
		bar := renderBar(pc.PeakAvg, maxVal, barWidth)
		b.WriteString(fmt.Sprintf("%s %s %s\n", peakLabel, bar, infoStyle.Render(fmt.Sprintf("%.1f %s", pc.PeakAvg, unit))))
	} else {
		b.WriteString(fmt.Sprintf("%s %s\n", peakLabel, dimStyle.Render("no data")))
	}

	if pc.OffPeakCount > 0 {
		bar := renderBar(pc.OffPeakAvg, maxVal, barWidth)
		b.WriteString(fmt.Sprintf("%s %s %s", offLabel, bar, infoStyle.Render(fmt.Sprintf("%.1f %s", pc.OffPeakAvg, unit))))
	} else {
		b.WriteString(fmt.Sprintf("%s %s", offLabel, dimStyle.Render("no data")))
	}

	return b.String()
}

// RenderAnalytics renders the analytics view.
// Returns a "no data" message when summary is nil.
func RenderAnalytics(summary *model.Summary, width int) string {
	if summary == nil {
		msg := dimStyle.Render("No test data yet. Run a speed test first.")
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, msg)
	}

	var b strings.Builder

	// Header
	header := sectionLabelStyle.Render("Analytics")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, header))
	b.WriteString("\n")

	// Date range + count
	dateFrom := summary.DateRange[0].Format("Jan 2")
	dateTo := summary.DateRange[1].Format("Jan 2")
	meta := hintDescStyle.Render(fmt.Sprintf("%d tests from %s - %s", summary.TotalTests, dateFrom, dateTo))
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, meta))
	b.WriteString("\n\n")

	// Sparkline trends
	type metricRow struct {
		label   string
		summary model.MetricSummary
		unit    string
	}
	metrics := []metricRow{
		{"Download", summary.Download, "Mbps"},
		{"Upload", summary.Upload, "Mbps"},
		{"Ping", summary.Ping, "ms"},
	}

	for _, m := range metrics {
		label := hintDescStyle.Render(fmt.Sprintf("  %-10s", m.label))
		spark := metricValueStyle.Render(m.summary.Sparkline)
		avg := infoStyle.Render(fmt.Sprintf("%.1f %s avg", m.summary.Average, m.unit))

		line := fmt.Sprintf("%s %s  %s", label, spark, avg)
		if summary.TotalTests >= 2 {
			line += "  " + trendArrow(m.summary.Trend, m.summary.TrendPct)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Minimum data guard
	if summary.TotalTests < 2 {
		b.WriteString("\n")
		msg := dimStyle.Render("  Run more tests to see trends.")
		b.WriteString(msg)
		b.WriteString("\n")
	} else {
		// Peak vs off-peak
		b.WriteString("\n")
		peakHeader := sectionLabelStyle.Render("  Peak vs Off-Peak") +
			hintDescStyle.Render(fmt.Sprintf("  (peak: %02d:00-%02d:00)", model.PeakStartHour, model.PeakEndHour))
		b.WriteString(peakHeader)
		b.WriteString("\n\n")

		b.WriteString(hintDescStyle.Render("  Download"))
		b.WriteString("\n")
		b.WriteString(renderPeakSection(summary.PeakDownload, "Mbps", width))
		b.WriteString("\n\n")

		b.WriteString(hintDescStyle.Render("  Upload"))
		b.WriteString("\n")
		b.WriteString(renderPeakSection(summary.PeakUpload, "Mbps", width))
		b.WriteString("\n\n")

		peakTotal := summary.PeakDownload.PeakCount
		offTotal := summary.PeakDownload.OffPeakCount
		counts := dimStyle.Render(fmt.Sprintf("  %d peak tests, %d off-peak tests", peakTotal, offTotal))
		b.WriteString(counts)
		b.WriteString("\n")
	}

	// Hints
	b.WriteString("\n")
	hint := formatHint(ContextAnalytics)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, hint))

	return b.String()
}
