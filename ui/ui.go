package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/jkleinne/lazyspeed/internal/timeutil"
	"github.com/jkleinne/lazyspeed/model"
)

const (
	historyOverhead      = 22 // title + latest-results box + table header + controls + padding
	historyMinVisible    = 3
	serverListOverhead   = 8 // title + header + hint + padding
	serverListMinVisible = 3

	// resultSeparator is the horizontal rule drawn below the "Latest Results" heading.
	// Width is chosen to match the typical content width of the results box.
	resultSeparator = "──────────────────────"

	// serverDividerWidth is the number of em-dashes drawn between the favorites
	// group and the remaining servers in the selection list.
	serverDividerWidth = 40
)

// Viewport groups the display dimensions shared by windowed render functions.
type Viewport struct {
	Width  int
	Height int
	Offset int
	Cursor int
}

// clampViewport clamps offset into [0, total-maxVisible] and returns
// the adjusted offset and the exclusive end index for slicing.
func clampViewport(total, maxVisible, offset int) (clampedOffset, end int) {
	clampedOffset = max(0, offset)
	if total > maxVisible {
		clampedOffset = min(clampedOffset, total-maxVisible)
	}
	end = min(clampedOffset+maxVisible, total)
	return clampedOffset, end
}

// DefaultSpinner is a pre-configured spinner used by the TUI loading state.
// The frame set and style are intentionally fixed so that all callers share a
// consistent animation; do not mutate this value directly.
var DefaultSpinner = spinner.New(
	spinner.WithSpinner(spinner.Spinner{
		Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		FPS:    time.Second / 3,
	}),
	spinner.WithStyle(spinnerStyle),
)

// RenderTitle renders the application banner centered within width columns.
func RenderTitle(width int) string {
	name := gradientText("LazySpeed", gradientColors)
	banner := bannerBoxStyle.Render(name)

	bannerWidth := lipgloss.Width(banner)
	separator := gradientText(strings.Repeat("─", bannerWidth), gradientColors)

	title := lipgloss.JoinVertical(lipgloss.Center, banner, separator)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

// RenderSpinner renders the animated spinner with a progress bar and phase label.
// phase may contain a colon-separated label:value pair; both sides are styled
// differently for visual hierarchy.
func RenderSpinner(s spinner.Model, width int, phase string, progressAmount float64) string {
	spinnerView := spinnerStyle.Render(s.View())

	// Split phase into label and value if it contains a colon with trailing data
	var phaseText string
	if parts := strings.SplitN(phase, ":", 2); len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		phaseText = phaseStyle.Render(parts[0]+":") + " " + phaseValueStyle.Render(strings.TrimSpace(parts[1]))
	} else {
		phaseText = phaseStyle.Render(phase)
	}

	bw := spinnerBoxWidth(width)
	prog := newProgress(bw)
	progressBar := prog.ViewAs(progressAmount)
	progressView := progressStyle.Render(progressBar)

	content := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Left,
			spinnerView,
			"  ",
			phaseText,
		),
		progressView,
	)

	sized := boxStyle.Width(bw).Render(
		lipgloss.PlaceHorizontal(bw-2, lipgloss.Center, content),
	)

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, sized)
}

// renderLatestResult renders the "Latest Results" detail box.
func renderLatestResult(latest *model.SpeedTestResult) string {
	var b strings.Builder
	b.WriteString(sectionLabelStyle.Render("Latest Results"))
	b.WriteString("\n")
	b.WriteString(diagSeparatorStyle.Render(resultSeparator))
	b.WriteString("\n")
	fmt.Fprintf(&b, "Download  %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f Mbps", latest.DownloadSpeed)))
	fmt.Fprintf(&b, "Upload    %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f Mbps", latest.UploadSpeed)))
	fmt.Fprintf(&b, "Ping      %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f ms", latest.Ping)))
	fmt.Fprintf(&b, "Jitter    %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f ms", latest.Jitter)))
	fmt.Fprintf(&b, "Server    %s\n", infoStyle.Render(fmt.Sprintf("%s (%s)", latest.ServerName, latest.Country)))
	if latest.ServerSponsor != "" {
		fmt.Fprintf(&b, "Sponsor   %s\n", infoStyle.Render(latest.ServerSponsor))
	}
	if latest.Distance > 0 {
		fmt.Fprintf(&b, "Distance  %s\n", infoStyle.Render(fmt.Sprintf("%.1f km", latest.Distance)))
	}
	fmt.Fprintf(&b, "%s\n", metadataStyle.Render(latest.Timestamp.Format("03:04:05 PM")))
	if latest.UserIP != "" {
		ispInfo := latest.UserIP
		if latest.UserISP != "" {
			ispInfo = fmt.Sprintf("%s (%s)", latest.UserIP, latest.UserISP)
		}
		fmt.Fprintf(&b, "%s\n", metadataStyle.Render(ispInfo))
	}
	return boxStyle.Render(b.String())
}

// buildHistoryRows builds table rows newest-first, skipping the last entry
// (which is displayed in the latest-result box).
func buildHistoryRows(history []*model.SpeedTestResult) [][]string {
	rows := make([][]string, 0, len(history)-1)
	for i := len(history) - 2; i >= 0; i-- {
		test := history[i]
		rowNum := i + 1

		sponsorStr := "-"
		if test.ServerSponsor != "" {
			sponsorStr = test.ServerSponsor
		}
		distStr := "-"
		if test.Distance > 0 {
			distStr = fmt.Sprintf("%.1f", test.Distance)
		}

		rows = append(rows, []string{
			strconv.Itoa(rowNum),
			test.Timestamp.Format("Jan 02 03:04 PM"),
			fmt.Sprintf("%s (%s)", test.ServerName, test.Country),
			sponsorStr,
			distStr,
			fmt.Sprintf("%.2f", test.DownloadSpeed),
			fmt.Sprintf("%.2f", test.UploadSpeed),
			fmt.Sprintf("%.1f", test.Ping),
			fmt.Sprintf("%.1f", test.Jitter),
		})
	}
	return rows
}

// renderHistoryTable renders the "Previous Tests" table with viewport pagination.
func renderHistoryTable(history []*model.SpeedTestResult, vp Viewport) string {
	headers := []string{"#", "Time", "Server", "Sponsor", "Dist (km)", "DL (Mbps)", "UL (Mbps)", "Ping (ms)", "Jitter (ms)"}
	allRows := buildHistoryRows(history)

	totalRows := len(allRows)
	maxVisible := HistoryVisibleRows(vp.Height, totalRows)
	offset, end := clampViewport(totalRows, maxVisible, vp.Offset)

	t := table.New().
		Headers(headers...).
		Rows(allRows[offset:end]...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(tableBorderStyle).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if row%2 == 0 {
				return evenRowStyle
			}
			return oddRowStyle
		})

	label := sectionLabelStyle.Render("Previous Tests")
	historyContent := lipgloss.JoinVertical(lipgloss.Left, label, "", t.Render())

	if totalRows > maxVisible {
		paginationStr := dimStyle.Render(
			fmt.Sprintf("  Showing %d-%d of %d (↑/↓ to scroll)", offset+1, end, totalRows))
		historyContent = lipgloss.JoinVertical(lipgloss.Left, historyContent, paginationStr)
	}

	return historyContent
}

// RenderResults renders the full results view: latest-result box for the most
// recent entry plus a paginated history table for all prior entries.
// Returns an empty string when history is empty.
func RenderResults(history []*model.SpeedTestResult, vp Viewport) string {
	if len(history) == 0 {
		return ""
	}

	latestContent := renderLatestResult(history[len(history)-1])

	if len(history) == 1 {
		return lipgloss.PlaceHorizontal(vp.Width, lipgloss.Center, latestContent)
	}

	historyContent := renderHistoryTable(history, vp)
	content := lipgloss.JoinVertical(lipgloss.Center, latestContent, "\n", historyContent)

	return lipgloss.PlaceHorizontal(vp.Width, lipgloss.Center, content)
}

// RenderError renders err as a centered error banner. Returns empty string for nil.
func RenderError(err error, width int) string {
	if err == nil {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		errorStyle.Render(fmt.Sprintf("Error: %v", err)))
}

// RenderWarning renders warning as a centered warning banner. Returns empty string for "".
func RenderWarning(warning string, width int) string {
	if warning == "" {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		warningStyle.Render("Warning: "+warning))
}

// RenderHelp renders the help overlay. Pass hasResult=true to include the export hint.
func RenderHelp(width int, hasResult bool) string { //nolint:revive // hasResult describes input state, not behavior
	help := strings.Builder{}
	help.WriteString("\n")
	help.WriteString(sectionLabelStyle.Render("Controls:"))
	help.WriteString("\n")
	for _, b := range bindingsForContext(contextHome) {
		if b.ResultOnly && !hasResult {
			continue
		}
		fmt.Fprintf(&help, "  %s: %s\n",
			hintKeyStyle.Render(b.Key),
			hintDescStyle.Render(b.Description))
	}
	help.WriteString("\n")
	help.WriteString(sectionLabelStyle.Render("In Server Selection:"))
	help.WriteString("\n")
	for _, b := range bindingsForContext(contextServerSelection) {
		fmt.Fprintf(&help, "  %s: %s\n",
			hintKeyStyle.Render(b.Key),
			hintDescStyle.Render(b.Description))
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, help.String())
}

// RenderExportPrompt renders the inline format selection prompt shown when the
// user presses 'e' after a test completes.
func RenderExportPrompt(width int) string {
	bindings := bindingsForContext(contextExport)
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		parts = append(parts, fmt.Sprintf("[%s] %s",
			hintKeyStyle.Render(b.Key),
			hintDescStyle.Render(b.Description)))
	}
	prompt := hintDescStyle.Render("Export result:  ") + strings.Join(parts, "  ")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, prompt)
}

// RenderExportMessage renders the success or error message after an export.
func RenderExportMessage(msg string, width int) string {
	if msg == "" {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		infoStyle.Render(msg))
}

// HistoryVisibleRows returns how many history rows fit in the viewport.
func HistoryVisibleRows(height, total int) int {
	return min(total, max(historyMinVisible, height-historyOverhead))
}

// ServerListVisibleLines returns how many server entries fit in the viewport.
func ServerListVisibleLines(height, total int) int {
	return min(total, max(serverListMinVisible, height-serverListOverhead))
}

// countLeadingFavorites returns the number of servers at the front of the
// slice that are favorites. Assumes favorites are grouped at the front.
func countLeadingFavorites(servers []model.Server, favoriteIDs map[string]bool) int {
	for i, s := range servers {
		if !favoriteIDs[s.ID] {
			return i
		}
	}
	return len(servers)
}

// renderServerRow returns a styled server row with the appropriate prefix
// (▸ for cursor, ✓ for selected, ★ for favorite) and styling applied.
func renderServerRow(isCursor, isSelected, isFav bool, line string) string { //nolint:revive // isFav describes input state, not behavior
	switch {
	case isCursor:
		prefix := "▸ "
		if isFav {
			prefix = "▸★"
		}
		return selectedAccentStyle.Render(prefix) + selectedRowStyle.Render(line)
	case isSelected:
		prefix := "✓ "
		if isFav {
			prefix = "✓★"
		}
		return selectedAccentStyle.Render(prefix) + unselectedRowStyle.Render(line)
	default:
		prefix := "  "
		if isFav {
			prefix = "★ "
		}
		return unselectedRowStyle.Render(prefix + line)
	}
}

// RenderServerSelection renders the server list with viewport-based windowing.
// The cursor row is highlighted with a purple accent; rows in selected are marked
// with a ✓; favorite servers show a ★ prefix. Latency is color-coded green/amber/red.
// A nil selected map is treated as empty — no rows are marked as selected.
// A divider separates favorites from non-favorites when favorites exist.
// Precondition: servers must be ordered with favorites grouped at the front.
func RenderServerSelection(servers []model.Server, vp Viewport, selected map[int]bool, favoriteIDs map[string]bool) string {
	var b strings.Builder
	b.WriteString(sectionLabelStyle.Render("Select a server:"))
	b.WriteString("\n\n")

	total := len(servers)
	if total == 0 {
		b.WriteString(hintDescStyle.Render("  No servers available."))
		b.WriteString("\n")
		return lipgloss.PlaceHorizontal(vp.Width, lipgloss.Center, b.String())
	}

	favCount := countLeadingFavorites(servers, favoriteIDs)
	visible := ServerListVisibleLines(vp.Height, total)
	offset, end := clampViewport(total, visible, vp.Offset)

	if offset > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more", offset)))
		b.WriteString("\n")
	}

	for i := offset; i < end; i++ {
		if i == favCount && favCount > 0 && i > offset {
			b.WriteString(dimStyle.Render("  " + strings.Repeat("─", serverDividerWidth)))
			b.WriteString("\n")
		}

		server := servers[i]
		latencyMs := timeutil.DurationMs(server.Latency)
		latencyStr := latencyStyle(server.Latency).Render(fmt.Sprintf("%.2f ms", latencyMs))

		line := fmt.Sprintf("%s: %s (%s) — %s",
			server.Sponsor, server.Name, server.Country, latencyStr)

		b.WriteString(renderServerRow(vp.Cursor == i, selected[i], favoriteIDs[server.ID], line))
		b.WriteString("\n")
	}

	remaining := total - end
	if remaining > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(formatHint(contextServerSelection))

	return lipgloss.PlaceHorizontal(vp.Width, lipgloss.Center, b.String())
}

// RenderComparison renders a styled comparison table of multi-server speed test
// results. Servers holding the best value for each metric (highest DL/UL, lowest
// Ping/Jitter) are marked with ★. Failed servers are listed below the table as
// dimmed warning lines. A hint bar for the Comparison context is shown at the bottom.
func RenderComparison(results []*model.SpeedTestResult, errs []model.ServerError, width int) string {
	var b strings.Builder
	b.WriteString(sectionLabelStyle.Render("Server Comparison"))
	b.WriteString("\n\n")

	if len(results) > 0 {
		bm := model.FindBestMetrics(results)

		headers := []string{"Server", "Country", "DL (Mbps)", "UL (Mbps)", "Ping (ms)", "Jitter (ms)", ""}
		rows := buildComparisonRows(results, bm)

		t := table.New().
			Headers(headers...).
			Rows(rows...).
			Border(lipgloss.RoundedBorder()).
			BorderStyle(tableBorderStyle).
			StyleFunc(func(row, _ int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				if row%2 == 0 {
					return evenRowStyle
				}
				return oddRowStyle
			})

		b.WriteString(t.Render())
		b.WriteString("\n")
	}

	for _, se := range errs {
		line := fmt.Sprintf("✗ %s: %v", se.ServerName, se.Err)
		b.WriteString(warningStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(formatHint(contextComparison))

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, b.String())
}

// buildComparisonRows converts results into table rows, appending ★ to the last
// column for any result that holds a best-metric value.
func buildComparisonRows(results []*model.SpeedTestResult, bm model.BestMetrics) [][]string {
	rows := make([][]string, 0, len(results))
	for i, r := range results {
		star := ""
		if i == bm.DownloadIdx || i == bm.UploadIdx || i == bm.PingIdx || i == bm.JitterIdx {
			star = "★"
		}
		rows = append(rows, []string{
			r.ServerName,
			r.Country,
			fmt.Sprintf("%.2f", r.DownloadSpeed),
			fmt.Sprintf("%.2f", r.UploadSpeed),
			fmt.Sprintf("%.1f", r.Ping),
			fmt.Sprintf("%.1f", r.Jitter),
			star,
		})
	}
	return rows
}
