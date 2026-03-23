package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/jkleinne/lazyspeed/model"
)

var DefaultSpinner = spinner.New(
	spinner.WithSpinner(spinner.Spinner{
		Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
		FPS:    3,
	}),
	spinner.WithStyle(spinnerStyle),
)

func RenderTitle(width int) string {
	title := titleStyle.Render(" LazySpeed - Terminal Speed Test ")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

func RenderSpinner(s spinner.Model, width int, phase string, progressAmount float64) string {
	spinnerView := spinnerStyle.Render(s.View())
	phaseText := fmt.Sprintf("⏳ %s", phase)

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

func RenderResults(m *model.Model, width int) string {
	if len(m.TestHistory) == 0 {
		return ""
	}

	latest := m.TestHistory[len(m.TestHistory)-1]

	latestBox := strings.Builder{}
	latestBox.WriteString("Latest Test Results:\n")
	latestBox.WriteString("──────────────────────\n")
	fmt.Fprintf(&latestBox, "📥 Download: %.2f Mbps\n", latest.DownloadSpeed)
	fmt.Fprintf(&latestBox, "📤 Upload: %.2f Mbps\n", latest.UploadSpeed)
	fmt.Fprintf(&latestBox, "🔄 Ping: %.2f ms\n", latest.Ping)
	fmt.Fprintf(&latestBox, "📊 Jitter: %.2f ms\n", latest.Jitter)
	fmt.Fprintf(&latestBox, "🌍 Server: %s (%s)\n", latest.ServerName, latest.ServerCountry)
	if latest.ServerSponsor != "" {
		fmt.Fprintf(&latestBox, "🏢 Sponsor: %s\n", latest.ServerSponsor)
	}
	if latest.Distance > 0 {
		fmt.Fprintf(&latestBox, "📍 Distance: %.1f km\n", latest.Distance)
	}
	fmt.Fprintf(&latestBox, "🕒 Timestamp: %s\n", latest.Timestamp.Format("03:04:05 PM"))
	if latest.UserIP != "" {
		ispInfo := latest.UserIP
		if latest.UserISP != "" {
			ispInfo = fmt.Sprintf("%s (%s)", latest.UserIP, latest.UserISP)
		}
		fmt.Fprintf(&latestBox, "👤 IP: %s\n", ispInfo)
	}

	latestContent := infoStyle.Render(latestBox.String())

	if len(m.TestHistory) == 1 {
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, latestContent)
	}

	headers := []string{"#", "Time", "Server", "Sponsor", "Dist (km)", "DL (Mbps)", "UL (Mbps)", "Ping (ms)", "Jitter (ms)"}

	// Build rows newest-first (omitting the latest which is at index len-1)
	allRows := make([][]string, 0, len(m.TestHistory)-1)
	for i := len(m.TestHistory) - 2; i >= 0; i-- {
		test := m.TestHistory[i]
		rowNum := i + 1

		sponsorStr := "-"
		if test.ServerSponsor != "" {
			sponsorStr = test.ServerSponsor
		}
		distStr := "-"
		if test.Distance > 0 {
			distStr = fmt.Sprintf("%.1f", test.Distance)
		}

		allRows = append(allRows, []string{
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

	totalRows := len(allRows)
	maxVisible := HistoryVisibleRows(m.Height, totalRows)

	offset := m.HistoryOffset
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

	visibleRows := allRows[offset:end]

	t := table.New().
		Headers(headers...).
		Rows(visibleRows...).
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

	tableStr := t.Render()

	label := sectionLabelStyle.Render("📊 Previous Tests")

	historyContent := lipgloss.JoinVertical(lipgloss.Left, label, "", tableStr)

	if totalRows > maxVisible {
		paginationStr := helpStyle.Render(
			fmt.Sprintf("  Showing %d-%d of %d (↑/↓ to scroll)", offset+1, end, totalRows))
		historyContent = lipgloss.JoinVertical(lipgloss.Left, historyContent, paginationStr)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, latestContent, "\n", historyContent)

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

func RenderError(err error, width int) string {
	if err == nil {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		errorStyle.Render(fmt.Sprintf("❌ Error: %v", err)))
}

func RenderWarning(warning string, width int) string {
	if warning == "" {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		warningStyle.Render(fmt.Sprintf("⚠️ Warning: %s", warning)))
}

// RenderHelp renders the help overlay. Pass hasResult=true to include the export hint.
func RenderHelp(width int, hasResult bool) string {
	help := strings.Builder{}
	help.WriteString("\n")
	help.WriteString("Controls:\n")
	help.WriteString("  n: New Test\n")
	help.WriteString("  d: Diagnostics\n")
	if hasResult {
		help.WriteString("  e: Export Result\n")
		help.WriteString("  ↑/↓, j/k: Scroll History\n")
	}
	help.WriteString("  h: Toggle Help\n")
	help.WriteString("  q: Quit\n")
	help.WriteString("\nIn Server Selection:\n")
	help.WriteString("  ↑/↓, j/k: Navigate\n")
	help.WriteString("  Enter: Select Server\n")
	help.WriteString("  Esc: Back to Home\n")

	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		helpStyle.Render(help.String()))
}

// RenderExportPrompt renders the inline format selection prompt shown when the
// user presses 'e' after a test completes.
func RenderExportPrompt(width int) string {
	prompt := helpStyle.Render("Export result:  [j] JSON  [c] CSV  [Esc] cancel")
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
	visible := height - 22
	if visible < 3 {
		visible = 3
	}
	if visible > total {
		visible = total
	}
	return visible
}

// ServerListVisibleLines returns how many server entries fit in the viewport.
func ServerListVisibleLines(height, total int) int {
	visible := height - 8
	if visible < 3 {
		visible = 3
	}
	if visible > total {
		visible = total
	}
	return visible
}

// RenderServerSelection renders the server list with viewport-based windowing.
func RenderServerSelection(m *model.Model, width int) string {
	var b strings.Builder
	b.WriteString("Select a server:\n\n")

	total := len(m.ServerList)
	if total == 0 {
		b.WriteString("  No servers available.\n")
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, infoStyle.Render(b.String()))
	}

	visible := ServerListVisibleLines(m.Height, total)
	offset := m.ServerListOffset
	if offset < 0 {
		offset = 0
	}
	if offset > total-visible {
		offset = total - visible
	}

	if offset > 0 {
		fmt.Fprintf(&b, "  ↑ %d more\n", offset)
	}

	end := offset + visible
	if end > total {
		end = total
	}

	for i := offset; i < end; i++ {
		server := m.ServerList[i]
		if m.Cursor == i {
			fmt.Fprintf(&b, "> %s: %s (%s) - %.2f ms\n",
				server.Sponsor, server.Name, server.Country,
				server.Latency.Seconds()*1000)
		} else {
			fmt.Fprintf(&b, "  %s: %s (%s) - %.2f ms\n",
				server.Sponsor, server.Name, server.Country,
				server.Latency.Seconds()*1000)
		}
	}

	remaining := total - end
	if remaining > 0 {
		fmt.Fprintf(&b, "  ↓ %d more\n", remaining)
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, infoStyle.Render(b.String()))
}
