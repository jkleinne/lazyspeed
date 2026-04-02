package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jkleinne/lazyspeed/model"
)

const (
	analyticsBarMaxWidth = 25
)

// trendArrow returns a styled trend indicator string.
func trendArrow(ms model.MetricSummary) string {
	label := ms.TrendLabel()
	switch ms.Trend {
	case model.TrendUp:
		return latencyGreenStyle.Render(label)
	case model.TrendDown:
		return latencyRedStyle.Render(label)
	case model.TrendStable:
		return dimStyle.Render(label)
	default:
		return dimStyle.Render(label)
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

	peakLabel := hintDescStyle.Render(fmt.Sprintf("%-10s", "Peak"))
	offLabel := hintDescStyle.Render(fmt.Sprintf("%-10s", "Off-Peak"))

	if pc.PeakCount > 0 {
		bar := renderBar(pc.PeakAvg, maxVal, barWidth)
		fmt.Fprintf(&b, "%s %s %s\n", peakLabel, bar, infoStyle.Render(fmt.Sprintf("%.1f %s", pc.PeakAvg, unit)))
	} else {
		fmt.Fprintf(&b, "%s %s\n", peakLabel, dimStyle.Render("no data"))
	}

	if pc.OffPeakCount > 0 {
		bar := renderBar(pc.OffPeakAvg, maxVal, barWidth)
		fmt.Fprintf(&b, "%s %s %s", offLabel, bar, infoStyle.Render(fmt.Sprintf("%.1f %s", pc.OffPeakAvg, unit)))
	} else {
		fmt.Fprintf(&b, "%s %s", offLabel, dimStyle.Render("no data"))
	}

	return b.String()
}

// centerBlock centers a multi-line block as a unit within the given width.
// Each line gets the same left padding so internal alignment is preserved.
func centerBlock(block string, width int) string {
	lines := strings.Split(strings.TrimRight(block, "\n"), "\n")

	maxW := 0
	for _, line := range lines {
		w := ansi.StringWidth(line)
		if w > maxW {
			maxW = w
		}
	}

	pad := (width - maxW) / 2
	if pad < 0 {
		pad = 0
	}
	prefix := strings.Repeat(" ", pad)

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(prefix)
		b.WriteString(line)
	}
	return b.String()
}

// renderSparklineTrends renders the sparkline + average + trend row for each metric.
func renderSparklineTrends(summary *model.Summary) string {
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

	var b strings.Builder
	for _, m := range metrics {
		label := hintDescStyle.Render(fmt.Sprintf("%-10s", m.label))
		spark := metricValueStyle.Render(m.summary.Sparkline)
		avg := infoStyle.Render(fmt.Sprintf("%.1f %s avg", m.summary.Average, m.unit))

		line := fmt.Sprintf("%s %s  %s", label, spark, avg)
		if summary.TotalTests >= 2 {
			line += "  " + trendArrow(m.summary)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// renderPeakComparison renders the peak vs off-peak section with bar charts.
func renderPeakComparison(summary *model.Summary, width int) string {
	var b strings.Builder

	header := sectionLabelStyle.Render("Peak vs Off-Peak") +
		hintDescStyle.Render(fmt.Sprintf("  (peak: %02d:00-%02d:00)", model.PeakStartHour, model.PeakEndHour))
	b.WriteString(header)
	b.WriteString("\n\n")

	b.WriteString(hintDescStyle.Render("Download"))
	b.WriteString("\n")
	b.WriteString(renderPeakSection(summary.PeakDownload, "Mbps", width))
	b.WriteString("\n\n")

	b.WriteString(hintDescStyle.Render("Upload"))
	b.WriteString("\n")
	b.WriteString(renderPeakSection(summary.PeakUpload, "Mbps", width))
	b.WriteString("\n\n")

	peakTotal := summary.PeakDownload.PeakCount
	offTotal := summary.PeakDownload.OffPeakCount
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d peak tests, %d off-peak tests", peakTotal, offTotal)))
	b.WriteString("\n")

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

	header := sectionLabelStyle.Render("Analytics")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, header))
	b.WriteString("\n")

	dateFrom := summary.DateRange[0].Format("Jan 2")
	dateTo := summary.DateRange[1].Format("Jan 2")
	meta := hintDescStyle.Render(fmt.Sprintf("%d tests from %s - %s", summary.TotalTests, dateFrom, dateTo))
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, meta))
	b.WriteString("\n\n")

	var content strings.Builder
	content.WriteString(renderSparklineTrends(summary))

	if summary.TotalTests < 2 {
		content.WriteString("\n")
		content.WriteString(dimStyle.Render("Run more tests to see trends."))
		content.WriteString("\n")
	} else {
		content.WriteString("\n")
		content.WriteString(renderPeakComparison(summary, width))
	}

	b.WriteString(centerBlock(content.String(), width))

	b.WriteString("\n")
	hint := formatHint(ContextAnalytics)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, hint))

	return b.String()
}
