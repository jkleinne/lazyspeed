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

	ipColWidth    = 18
	hostColWidth  = 30
	hopTableWidth = 76
)

// renderLatency formats a duration value with colour coding.
func renderLatency(d time.Duration) string {
	s := fmt.Sprintf("%.0fms", diag.DurationMs(d))
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

	sorted := slices.Clone(latencies)
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

	// Target
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
		diagTargetStyle.Render(result.Target)))
	b.WriteString("\n\n")

	// Score line
	scoreStr := scoreStyle(result.Quality.Grade).Render(
		fmt.Sprintf("%d/100 (%s)", result.Quality.Score, result.Quality.Grade))
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, scoreStr))
	b.WriteString("\n")

	// Label
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
		diagLabelStyle.Render(result.Quality.Label)))
	b.WriteString("\n\n")

	// Summary line with split label/value colors
	dnsStr := dnsDisplayStr(result.DNS)
	summary := diagSummaryLabelStyle.Render("DNS: ") +
		infoStyle.Render(dnsStr) +
		diagSummarySepStyle.Render(" │ ") +
		diagSummaryLabelStyle.Render("Hops: ") +
		infoStyle.Render(fmt.Sprintf("%d", len(result.Hops))) +
		diagSummarySepStyle.Render(" │ ") +
		diagSummaryLabelStyle.Render("Route: ") +
		routeStatusStyled(result.Hops)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, summary))
	b.WriteString("\n")

	// Anomaly warnings
	anomalies := findAnomalies(result.Hops)
	for _, a := range anomalies {
		warn := fmt.Sprintf("Warning: hop %d (%s) has unusually high latency: %dms",
			a.Number, a.IP, a.Latency.Milliseconds())
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
			warningStyle.Render(warn)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Hint
	hint := formatHint(ContextDiagCompact)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, hint))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// RenderDiagExpanded renders the full scrollable hop table.
// The title is rendered by the main View(); this function begins with score + label inline.
func RenderDiagExpanded(result *diag.DiagResult, width, height, offset int) string {
	var b strings.Builder

	// Compact header: score inline (title comes from main View())
	scoreText := scoreStyle(result.Quality.Grade).Render(
		fmt.Sprintf("%d/100 (%s)", result.Quality.Score, result.Quality.Grade))
	labelText := diagLabelStyle.Render(result.Quality.Label)
	header := lipgloss.JoinHorizontal(lipgloss.Top, scoreText, "  ", labelText)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, header))
	b.WriteString("\n")

	// Meta line
	dnsStr := dnsDisplayStr(result.DNS)
	meta := diagSummaryLabelStyle.Render("Target: ") +
		diagTargetStyle.Render(result.Target) +
		diagSummarySepStyle.Render(" │ ") +
		diagSummaryLabelStyle.Render("Method: ") +
		infoStyle.Render(result.Method) +
		diagSummarySepStyle.Render(" │ ") +
		diagSummaryLabelStyle.Render("DNS: ") +
		infoStyle.Render(dnsStr)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, meta))
	b.WriteString("\n\n")

	// Column headers
	colHeader := diagHeaderLightStyle.Render(
		fmt.Sprintf("%-6s %-18s %-30s %s", "Hop", "IP", "Host", "Latency"),
	)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, colHeader))
	b.WriteString("\n")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
		diagSeparatorStyle.Render(strings.Repeat("─", hopTableWidth))))
	b.WriteString("\n")

	// Viewport windowing
	totalRows := len(result.Hops)
	maxVisible := min(totalRows, max(diagMinVisible, height-diagExpandedOverhead))

	var end int
	offset, end = clampViewport(totalRows, maxVisible, offset)

	// Up indicator
	if offset > 0 {
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
			dimStyle.Render(fmt.Sprintf("^ %d more above", offset))))
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
			Truncate(ipStr, ipColWidth-1),
			Truncate(hostStr, hostColWidth-1),
			latStr,
		)

		// Pad to fixed width for consistent background fill
		rowRunes := []rune(row)
		if len(rowRunes) < hopTableWidth {
			row += strings.Repeat(" ", hopTableWidth-len(rowRunes))
		}

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
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
			dimStyle.Render(fmt.Sprintf("v %d more below", remaining))))
		b.WriteString("\n")
	}

	if totalRows > maxVisible {
		b.WriteString("\n")
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
			dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d hops", offset+1, end, totalRows))))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Hint
	hint := formatHint(ContextDiagExpanded)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, hint))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// Truncate shortens s to at most maxLen runes, appending "…" if trimmed.
func Truncate(s string, maxLen int) string {
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
	return fmt.Sprintf("%.0fms (%s)", diag.DurationMs(dns.Latency), cacheLabel)
}
