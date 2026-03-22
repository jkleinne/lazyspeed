package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jkleinne/lazyspeed/diag"
)

var (
	diagTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2)

	diagScoreStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	latencyGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22c55e"))

	latencyAmberStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f59e0b"))

	latencyRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef4444"))

	diagHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
)

// latencyStyle returns the appropriate lipgloss style for a given latency.
func latencyStyle(d time.Duration) lipgloss.Style {
	ms := d.Milliseconds()
	switch {
	case ms < 50:
		return latencyGreenStyle
	case ms <= 100:
		return latencyAmberStyle
	default:
		return latencyRedStyle
	}
}

// renderLatency formats a duration value with colour coding.
func renderLatency(d time.Duration) string {
	s := fmt.Sprintf("%dms", d.Milliseconds())
	return latencyStyle(d).Render(s)
}

// findAnomalies returns hops whose latency is more than 2x the median and
// more than 50 ms absolute.
func findAnomalies(hops []diag.Hop) []diag.Hop {
	var latencies []float64
	for _, h := range hops {
		if !h.Timeout {
			latencies = append(latencies, float64(h.Latency.Milliseconds()))
		}
	}
	if len(latencies) == 0 {
		return nil
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	var median float64
	n := len(sorted)
	if n%2 == 0 {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		median = sorted[n/2]
	}

	var anomalies []diag.Hop
	for _, h := range hops {
		if h.Timeout {
			continue
		}
		ms := float64(h.Latency.Milliseconds())
		if ms > 2*median && ms > 50 {
			anomalies = append(anomalies, h)
		}
	}
	return anomalies
}

// routeStatus returns a short health summary string for the hop list.
func routeStatus(hops []diag.Hop) string {
	var timeouts int
	for _, h := range hops {
		if h.Timeout {
			timeouts++
		}
	}
	switch timeouts {
	case 0:
		return "healthy"
	case 1:
		return "1 timeout"
	default:
		return fmt.Sprintf("%d timeouts", timeouts)
	}
}

// RenderDiagCompact renders a compact single-screen diagnostics summary.
func RenderDiagCompact(result *diag.DiagResult, width int) string {
	var b strings.Builder

	// Title
	title := diagTitleStyle.Render("~ Network Diagnostics ~")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, title))
	b.WriteString("\n\n")

	// Target
	b.WriteString(infoStyle.Render(fmt.Sprintf("Target: %s", result.Target)))
	b.WriteString("\n\n")

	// Score line
	scoreStr := diagScoreStyle.Render(fmt.Sprintf("%d/100 (%s)", result.Quality.Score, result.Quality.Grade))
	b.WriteString(scoreStr)
	b.WriteString("\n")

	// Label
	b.WriteString(infoStyle.Render(result.Quality.Label))
	b.WriteString("\n\n")

	// Summary line
	dnsStr := "N/A"
	if result.DNS != nil {
		cacheLabel := "cold"
		if result.DNS.Cached {
			cacheLabel = "cached"
		}
		dnsStr = fmt.Sprintf("%dms (%s)", result.DNS.Latency.Milliseconds(), cacheLabel)
	}
	summary := fmt.Sprintf("DNS: %s | Hops: %d | Route: %s",
		dnsStr,
		len(result.Hops),
		routeStatus(result.Hops),
	)
	b.WriteString(helpStyle.Render(summary))
	b.WriteString("\n")

	// Anomaly warnings
	anomalies := findAnomalies(result.Hops)
	for _, a := range anomalies {
		warn := fmt.Sprintf("Warning: hop %d (%s) has unusually high latency: %dms",
			a.Number, a.IP, a.Latency.Milliseconds())
		b.WriteString(warningStyle.Render(warn))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Hint
	hint := "Enter: expand trace | d: new diagnostic | n: speed test | q: quit"
	b.WriteString(helpStyle.Render(hint))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// RenderDiagExpanded renders the full scrollable hop table.
func RenderDiagExpanded(result *diag.DiagResult, width, height, offset int) string {
	var b strings.Builder

	// Compact header: title + score inline
	titleText := diagTitleStyle.Render("~ Network Diagnostics ~")
	scoreText := diagScoreStyle.Render(
		fmt.Sprintf("%d/100 (%s) — %s", result.Quality.Score, result.Quality.Grade, result.Quality.Label),
	)
	header := lipgloss.JoinHorizontal(lipgloss.Top, titleText, "  ", scoreText)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, header))
	b.WriteString("\n")

	// Meta line
	dnsStr := "N/A"
	if result.DNS != nil {
		cacheLabel := "cold"
		if result.DNS.Cached {
			cacheLabel = "cached"
		}
		dnsStr = fmt.Sprintf("%dms (%s)", result.DNS.Latency.Milliseconds(), cacheLabel)
	}
	meta := helpStyle.Render(fmt.Sprintf("Target: %s | Method: %s | DNS: %s",
		result.Target, result.Method, dnsStr))
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, meta))
	b.WriteString("\n\n")

	// Column headers
	hopCol := diagHeaderStyle.Render("Hop")
	ipCol := diagHeaderStyle.Render("IP")
	hostCol := diagHeaderStyle.Render("Host")
	latCol := diagHeaderStyle.Render("Latency")
	colHeader := fmt.Sprintf("%-6s %-18s %-30s %s", hopCol, ipCol, hostCol, latCol)
	b.WriteString(colHeader)
	b.WriteString("\n")

	// Viewport windowing — same pattern as HistoryVisibleRows
	totalRows := len(result.Hops)
	maxVisible := height - 10
	if maxVisible < 3 {
		maxVisible = 3
	}
	if maxVisible > totalRows {
		maxVisible = totalRows
	}

	if offset < 0 {
		offset = 0
	}
	if totalRows > maxVisible && offset > totalRows-maxVisible {
		offset = totalRows - maxVisible
	}
	if offset < 0 {
		offset = 0
	}

	end := offset + maxVisible
	if end > totalRows {
		end = totalRows
	}

	// Up indicator
	if offset > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  ^ %d more above", offset)))
		b.WriteString("\n")
	}

	// Hop rows
	for _, hop := range result.Hops[offset:end] {
		var latStr string
		if hop.Timeout {
			latStr = latencyRedStyle.Render("timeout")
		} else {
			latStr = renderLatency(hop.Latency)
		}

		ipStr := hop.IP
		if ipStr == "" {
			ipStr = "*"
		}
		hostStr := hop.Host
		if hostStr == "" {
			hostStr = "*"
		}

		row := fmt.Sprintf("%-6d %-18s %-30s %s",
			hop.Number,
			truncate(ipStr, 17),
			truncate(hostStr, 29),
			latStr,
		)
		b.WriteString(infoStyle.Render(row))
		b.WriteString("\n")
	}

	// Down indicator / pagination
	remaining := totalRows - end
	if remaining > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("  v %d more below", remaining)))
		b.WriteString("\n")
	}

	if totalRows > maxVisible {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(
			fmt.Sprintf("Showing %d-%d of %d hops", offset+1, end, totalRows),
		))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Hint
	hint := "Up/Down: scroll | Esc: compact view | d: new diagnostic | q: quit"
	b.WriteString(helpStyle.Render(hint))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// truncate shortens s to at most maxLen characters, appending "…" if trimmed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}
