package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
)

const (
	historyOverhead      = 22 // title + latest-results box + table header + controls + padding
	historyMinVisible    = 3
	serverListOverhead   = 8 // title + header + hint + padding
	serverListMinVisible = 3
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

var DefaultSpinner = spinner.New(
	spinner.WithSpinner(spinner.Spinner{
		Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		FPS:    time.Second / 3,
	}),
	spinner.WithStyle(spinnerStyle),
)

func RenderTitle(width int) string {
	name := gradientText("LazySpeed", gradientColors)
	banner := bannerBoxStyle.Render(name)

	bannerWidth := lipgloss.Width(banner)
	separator := gradientText(strings.Repeat("─", bannerWidth), gradientColors)

	title := lipgloss.JoinVertical(lipgloss.Center, banner, separator)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

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
	b.WriteString(diagSeparatorStyle.Render("──────────────────────"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "Download  %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f Mbps", latest.DownloadSpeed)))
	fmt.Fprintf(&b, "Upload    %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f Mbps", latest.UploadSpeed)))
	fmt.Fprintf(&b, "Ping      %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f ms", latest.Ping)))
	fmt.Fprintf(&b, "Jitter    %s\n", metricValueStyle.Render(fmt.Sprintf("%.2f ms", latest.Jitter)))
	fmt.Fprintf(&b, "Server    %s\n", infoStyle.Render(fmt.Sprintf("%s (%s)", latest.ServerName, latest.ServerCountry)))
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
			fmt.Sprintf("%d", rowNum),
			test.Timestamp.Format("Jan 02 03:04 PM"),
			fmt.Sprintf("%s (%s)", test.ServerName, test.ServerCountry),
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

func RenderError(err error, width int) string {
	if err == nil {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		errorStyle.Render(fmt.Sprintf("Error: %v", err)))
}

func RenderWarning(warning string, width int) string {
	if warning == "" {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		warningStyle.Render(fmt.Sprintf("Warning: %s", warning)))
}

// RenderHelp renders the help overlay. Pass hasResult=true to include the export hint.
func RenderHelp(width int, hasResult bool) string {
	help := strings.Builder{}
	help.WriteString("\n")
	help.WriteString(sectionLabelStyle.Render("Controls:"))
	help.WriteString("\n")
	for _, b := range BindingsForContext(ContextHome) {
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
	for _, b := range BindingsForContext(ContextServerSelection) {
		fmt.Fprintf(&help, "  %s: %s\n",
			hintKeyStyle.Render(b.Key),
			hintDescStyle.Render(b.Description))
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, help.String())
}

// RenderExportPrompt renders the inline format selection prompt shown when the
// user presses 'e' after a test completes.
func RenderExportPrompt(width int) string {
	bindings := BindingsForContext(ContextExport)
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

// RenderServerSelection renders the server list with viewport-based windowing.
// The selected row is highlighted with a purple accent; unselected rows are dimmed.
// Latency is color-coded green/amber/red based on duration thresholds.
func RenderServerSelection(servers []model.Server, vp Viewport) string {
	var b strings.Builder
	b.WriteString(sectionLabelStyle.Render("Select a server:"))
	b.WriteString("\n\n")

	total := len(servers)
	if total == 0 {
		b.WriteString(hintDescStyle.Render("  No servers available."))
		b.WriteString("\n")
		return lipgloss.PlaceHorizontal(vp.Width, lipgloss.Center, b.String())
	}

	visible := ServerListVisibleLines(vp.Height, total)
	offset, end := clampViewport(total, visible, vp.Offset)

	if offset > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more", offset)))
		b.WriteString("\n")
	}

	for i := offset; i < end; i++ {
		server := servers[i]
		latencyMs := diag.DurationMs(server.Latency)
		latencyStr := latencyStyle(server.Latency).Render(fmt.Sprintf("%.2f ms", latencyMs))

		line := fmt.Sprintf("%s: %s (%s) — %s",
			server.Sponsor, server.Name, server.Country, latencyStr)

		if vp.Cursor == i {
			accent := selectedAccentStyle.Render("▸ ")
			row := selectedRowStyle.Render(line)
			b.WriteString(accent + row)
		} else {
			b.WriteString(unselectedRowStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	remaining := total - end
	if remaining > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(formatHint(ContextServerSelection))

	return lipgloss.PlaceHorizontal(vp.Width, lipgloss.Center, b.String())
}
