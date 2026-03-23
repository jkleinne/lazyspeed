package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jkleinne/lazyspeed/diag"
)

const (
	diagExpandedOverhead = 10 // title + score + meta + column headers + separator + hints + padding
	diagMinVisible       = 3
	anomalyMultiplier    = 2  // latency must exceed this factor of the median
	anomalyAbsoluteMinMs = 50 // latency must also exceed this absolute floor (ms)
)

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
	slices.Sort(sorted)

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
		if ms > anomalyMultiplier*median && ms > anomalyAbsoluteMinMs {
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

// routeStatusStyled returns a color-coded route health string.
func routeStatusStyled(hops []diag.Hop) string {
	status := routeStatus(hops)
	switch status {
	case "healthy":
		return latencyGreenStyle.Render(status)
	case "1 timeout":
		return latencyAmberStyle.Render(status)
	default:
		return latencyRedStyle.Render(status)
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
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, infoStyle.Render(fmt.Sprintf("Target: %s", result.Target))))
	b.WriteString("\n\n")

	// Score line
	scoreStr := scoreStyle(result.Quality.Grade).Render(fmt.Sprintf("%d/100 (%s)", result.Quality.Score, result.Quality.Grade))
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, scoreStr))
	b.WriteString("\n")

	// Label
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, infoStyle.Render(result.Quality.Label)))
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
	summaryPrefix := fmt.Sprintf("DNS: %s | Hops: %d | Route: ",
		dnsStr,
		len(result.Hops),
	)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(summaryPrefix)+routeStatusStyled(result.Hops)))
	b.WriteString("\n")

	// Anomaly warnings
	anomalies := findAnomalies(result.Hops)
	for _, a := range anomalies {
		warn := fmt.Sprintf("Warning: hop %d (%s) has unusually high latency: %dms",
			a.Number, a.IP, a.Latency.Milliseconds())
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, warningStyle.Render(warn)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Hint
	hint := "Enter: expand trace | d: new diagnostic | n: speed test | q: quit"
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(hint)))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// RenderDiagExpanded renders the full scrollable hop table.
func RenderDiagExpanded(result *diag.DiagResult, width, height, offset int) string {
	var b strings.Builder

	// Compact header: title + score inline
	titleText := diagTitleStyle.Render("~ Network Diagnostics ~")
	scoreText := scoreStyle(result.Quality.Grade).Render(
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
	colHeader := diagHeaderLightStyle.Render(
		fmt.Sprintf("%-6s %-18s %-30s %s", "Hop", "IP", "Host", "Latency"),
	)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, colHeader))
	b.WriteString("\n")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(strings.Repeat("─", 76))))
	b.WriteString("\n")

	// Viewport windowing — same pattern as HistoryVisibleRows
	totalRows := len(result.Hops)
	maxVisible := min(totalRows, max(diagMinVisible, height-diagExpandedOverhead))

	var end int
	offset, end = clampViewport(totalRows, maxVisible, offset)

	// Up indicator
	if offset > 0 {
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(fmt.Sprintf("^ %d more above", offset))))
		b.WriteString("\n")
	}

	// Hop rows
	for i, hop := range result.Hops[offset:end] {
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

		// Alternate by absolute hop position so color is stable when scrolling
		var styledRow string
		if (offset+i)%2 == 0 {
			styledRow = diagEvenRowStyle.Render(row)
		} else {
			styledRow = diagOddRowStyle.Render(row)
		}
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, styledRow))
		b.WriteString("\n")
	}

	// Down indicator / pagination
	remaining := totalRows - end
	if remaining > 0 {
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(fmt.Sprintf("v %d more below", remaining))))
		b.WriteString("\n")
	}

	if totalRows > maxVisible {
		b.WriteString("\n")
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(
			fmt.Sprintf("Showing %d-%d of %d hops", offset+1, end, totalRows),
		)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Hint
	hint := "Up/Down: scroll | Esc: compact view | d: new diagnostic | q: quit"
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(hint)))

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
