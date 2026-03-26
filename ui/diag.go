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

	hopColWidth   = 6
	ipColWidth    = 18
	hostColWidth  = 30
	hopTableWidth = 76
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
	title := titleStyle.Render("~ Network Diagnostics ~")
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
	dnsStr := dnsDisplayStr(result.DNS)
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
	hint := formatHint(ContextDiagCompact)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(hint)))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// RenderDiagExpanded renders the full scrollable hop table.
func RenderDiagExpanded(result *diag.DiagResult, width, height, offset int) string {
	var b strings.Builder

	// Compact header: title + score inline
	titleText := titleStyle.Render("~ Network Diagnostics ~")
	scoreText := scoreStyle(result.Quality.Grade).Render(
		fmt.Sprintf("%d/100 (%s) — %s", result.Quality.Score, result.Quality.Grade, result.Quality.Label),
	)
	header := lipgloss.JoinHorizontal(lipgloss.Top, titleText, "  ", scoreText)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, header))
	b.WriteString("\n")

	// Meta line
	dnsStr := dnsDisplayStr(result.DNS)
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
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(strings.Repeat("─", hopTableWidth))))
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
			truncate(ipStr, ipColWidth-1),
			truncate(hostStr, hostColWidth-1),
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
	hint := formatHint(ContextDiagExpanded)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, helpStyle.Render(hint)))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// truncate shortens s to at most maxLen runes, appending "…" if trimmed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// dnsDisplayStr formats a DNS result for display.
func dnsDisplayStr(dns *diag.DNSResult) string {
	if dns == nil {
		return "N/A"
	}
	cacheLabel := "cold"
	if dns.Cached {
		cacheLabel = "cached"
	}
	return fmt.Sprintf("%dms (%s)", dns.Latency.Milliseconds(), cacheLabel)
}
