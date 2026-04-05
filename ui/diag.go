package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/internal/timeutil"
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
	s := fmt.Sprintf("%.0fms", timeutil.DurationMs(d))
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

// routeHealth classifies the overall health of a traceroute path by the
// number of timed-out hops. Three severity levels map directly to the three
// colour bands used in the diagnostics UI.
type routeHealth int

const (
	routeHealthy      routeHealth = iota // no timed-out hops
	routeMinorTimeout                    // exactly one timed-out hop
	routeDegraded                        // two or more timed-out hops
)

// countTimeouts returns the number of timed-out hops.
func countTimeouts(hops []diag.Hop) int {
	var n int
	for _, h := range hops {
		if h.Timeout {
			n++
		}
	}
	return n
}

// routeStatus returns a short human-readable health summary and the
// corresponding health level for the hop list.
func routeStatus(hops []diag.Hop) (string, routeHealth) {
	timeouts := countTimeouts(hops)
	switch timeouts {
	case 0:
		return "healthy", routeHealthy
	case 1:
		return "1 timeout", routeMinorTimeout
	default:
		return fmt.Sprintf("%d timeouts", timeouts), routeDegraded
	}
}

// routeStatusStyled returns a color-coded route health string.
func routeStatusStyled(hops []diag.Hop) string {
	status, health := routeStatus(hops)
	switch health {
	case routeHealthy:
		return latencyGreenStyle.Render(status)
	case routeMinorTimeout:
		return latencyAmberStyle.Render(status)
	case routeDegraded:
		return latencyRedStyle.Render(status)
	}
	// Unreachable: all routeHealth values are covered above.
	return status
}

// RenderDiagCompact renders a compact single-screen diagnostics summary.
func RenderDiagCompact(result *diag.Result, width int) string {
	var b strings.Builder

	// Target
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
		diagTargetStyle.Render(result.Target)))
	b.WriteString("\n\n")

	// Score line
	scoreStr := scoreStyle(string(result.Quality.Grade)).Render(
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
	hint := formatHint(contextDiagCompact)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, hint))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// renderDiagHeader renders the score/label inline header and meta line.
func renderDiagHeader(result *diag.Result, width int) string {
	var b strings.Builder

	scoreText := scoreStyle(string(result.Quality.Grade)).Render(
		fmt.Sprintf("%d/100 (%s)", result.Quality.Score, result.Quality.Grade))
	labelText := diagLabelStyle.Render(result.Quality.Label)
	header := lipgloss.JoinHorizontal(lipgloss.Top, scoreText, "  ", labelText)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, header))
	b.WriteString("\n")

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

	return b.String()
}

// renderDiagColumnHeader renders the column header row and separator.
func renderDiagColumnHeader(width int) string {
	var b strings.Builder
	colHeader := diagHeaderLightStyle.Render(
		fmt.Sprintf("%-6s %-18s %-30s %s", "Hop", "IP", "Host", "Latency"),
	)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, colHeader))
	b.WriteString("\n")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
		diagSeparatorStyle.Render(strings.Repeat("─", hopTableWidth))))
	b.WriteString("\n")
	return b.String()
}

// renderHopRow renders a single hop row with latency styling and alternating background.
func renderHopRow(hop diag.Hop, absoluteIndex int) string {
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

	rowRunes := []rune(row)
	if len(rowRunes) < hopTableWidth {
		row += strings.Repeat(" ", hopTableWidth-len(rowRunes))
	}

	if absoluteIndex%2 == 0 {
		return diagEvenRowStyle.Render(row)
	}
	return diagOddRowStyle.Render(row)
}

// renderScrollHeader renders the "N more above" indicator.
func renderScrollHeader(offset, width int) string {
	if offset <= 0 {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		dimStyle.Render(fmt.Sprintf("^ %d more above", offset))) + "\n"
}

// scrollState holds viewport pagination state for scrollable content.
type scrollState struct {
	offset     int
	end        int
	total      int
	maxVisible int
}

// renderScrollFooter renders the down indicator and pagination count.
func renderScrollFooter(scroll scrollState, width int) string {
	var b strings.Builder

	remaining := scroll.total - scroll.end
	if remaining > 0 {
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
			dimStyle.Render(fmt.Sprintf("v %d more below", remaining))))
		b.WriteString("\n")
	}

	if scroll.total > scroll.maxVisible {
		b.WriteString("\n")
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center,
			dimStyle.Render(fmt.Sprintf("Showing %d-%d of %d hops", scroll.offset+1, scroll.end, scroll.total))))
		b.WriteString("\n")
	}

	return b.String()
}

// RenderDiagExpanded renders the full scrollable hop table.
// The title is rendered by the main View(); this function begins with score + label inline.
func RenderDiagExpanded(result *diag.Result, width, height, offset int) string {
	var b strings.Builder

	b.WriteString(renderDiagHeader(result, width))
	b.WriteString(renderDiagColumnHeader(width))

	totalRows := len(result.Hops)
	maxVisible := min(totalRows, max(diagMinVisible, height-diagExpandedOverhead))
	offset, end := clampViewport(totalRows, maxVisible, offset)

	b.WriteString(renderScrollHeader(offset, width))

	for i, hop := range result.Hops[offset:end] {
		b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, renderHopRow(hop, offset+i)))
		b.WriteString("\n")
	}

	b.WriteString(renderScrollFooter(scrollState{
		offset:     offset,
		end:        end,
		total:      totalRows,
		maxVisible: maxVisible,
	}, width))

	b.WriteString("\n")
	hint := formatHint(contextDiagExpanded)
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, hint))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// RenderDiagInput renders the diagnostics target input screen.
// inputView is the rendered output from the textinput component.
func RenderDiagInput(inputView string, width int) string {
	var b strings.Builder

	instruction := diagSummaryLabelStyle.Render("Enter target hostname or IP address")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, instruction))
	b.WriteString("\n")

	defaultHint := dimStyle.Render("(press Enter for default)")
	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, defaultHint))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.PlaceHorizontal(width, lipgloss.Center, inputView))
	b.WriteString("\n\n")

	hint := formatHint(contextDiagInput)
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
	return fmt.Sprintf("%.0fms (%s)", timeutil.DurationMs(dns.Latency), cacheLabel)
}
