package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/jkleinne/lazyspeed/model"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	DefaultSpinner = spinner.New(
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
			FPS:    3,
		}),
		spinner.WithStyle(spinnerStyle),
	)

	// Table styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	evenRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			Padding(0, 1)

	oddRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#999999")).
			Padding(0, 1)
)

func RenderTitle(width int) string {
	title := titleStyle.Render(" LazySpeed - Terminal Speed Test ")
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, title)
}

var DefaultProgress = progress.New(
	progress.WithDefaultGradient(),
	progress.WithWidth(50),
	progress.WithoutPercentage(),
)

func RenderSpinner(s spinner.Model, width int, phase string, progressAmount float64) string {
	spinnerView := spinnerStyle.Render(s.View())
	phaseText := fmt.Sprintf("⏳ %s", phase)

	progressBar := DefaultProgress.ViewAs(progressAmount)
	progressStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		PaddingLeft(2).
		PaddingRight(2)

	progressView := progressStyle.Render(progressBar)

	content := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Left,
			spinnerView,
			"  ",
			phaseText,
		),
		progressView,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		PaddingLeft(1).
		PaddingRight(1).
		Width(70)

	boxedContent := boxStyle.Render(
		lipgloss.PlaceHorizontal(68, lipgloss.Center, content),
	)

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, boxedContent)
}

func RenderResults(m *model.Model, width int) string {
	if len(m.TestHistory) == 0 {
		return ""
	}

	latest := m.TestHistory[len(m.TestHistory)-1]
	
	latestBox := strings.Builder{}
	latestBox.WriteString("Latest Test Results:\n")
	latestBox.WriteString("──────────────────────\n")
	latestBox.WriteString(fmt.Sprintf("📥 Download: %.2f MBps\n", latest.DownloadSpeed))
	latestBox.WriteString(fmt.Sprintf("📤 Upload: %.2f MBps\n", latest.UploadSpeed))
	latestBox.WriteString(fmt.Sprintf("🔄 Ping: %.2f ms\n", latest.Ping))
	latestBox.WriteString(fmt.Sprintf("📊 Jitter: %.2f ms\n", latest.Jitter))
	latestBox.WriteString(fmt.Sprintf("🌍 Server: %s (%s)\n", latest.ServerName, latest.ServerLoc))
	if latest.ServerSponsor != "" {
		latestBox.WriteString(fmt.Sprintf("🏢 Sponsor: %s\n", latest.ServerSponsor))
	}
	if latest.Distance > 0 {
		latestBox.WriteString(fmt.Sprintf("📍 Distance: %.1f km\n", latest.Distance))
	}
	latestBox.WriteString(fmt.Sprintf("🕒 Timestamp: %s\n", latest.Timestamp.Format("03:04:05 PM")))
	if latest.UserIP != "" {
		ispInfo := latest.UserIP
		if latest.UserISP != "" {
			ispInfo = fmt.Sprintf("%s (%s)", latest.UserIP, latest.UserISP)
		}
		latestBox.WriteString(fmt.Sprintf("👤 IP: %s\n", ispInfo))
	}
	
	latestContent := infoStyle.Render(latestBox.String())

	if len(m.TestHistory) == 1 {
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, latestContent)
	}

	headers := []string{"#", "Time", "Server", "Sponsor", "Dist (km)", "DL (MBps)", "UL (MBps)", "Ping (ms)", "Jitter (ms)"}

	// Build rows newest-first (omitting the latest which is at index len-1)
	rows := make([][]string, 0, len(m.TestHistory)-1)
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

		rows = append(rows, []string{
			fmt.Sprintf("%d", rowNum),
			test.Timestamp.Format("Jan 02 03:04 PM"),
			fmt.Sprintf("%s (%s)", test.ServerName, test.ServerLoc),
			sponsorStr,
			distStr,
			fmt.Sprintf("%.2f", test.DownloadSpeed),
			fmt.Sprintf("%.2f", test.UploadSpeed),
			fmt.Sprintf("%.1f", test.Ping),
			fmt.Sprintf("%.1f", test.Jitter),
		})
	}

	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if row%2 == 0 {
				return evenRowStyle
			}
			return oddRowStyle
		})

	tableStr := t.Render()

	label := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("📊 Previous Tests")

	historyContent := lipgloss.JoinVertical(lipgloss.Left, label, "", tableStr)

	content := lipgloss.JoinVertical(lipgloss.Left, latestContent, "\n", historyContent)

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, content)
}

func RenderError(err error, width int) string {
	if err == nil {
		return ""
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		errorStyle.Render(fmt.Sprintf("❌ Error: %v", err)))
}

func RenderHelp(width int) string {
	help := strings.Builder{}
	help.WriteString("\n")
	help.WriteString("Controls:\n")
	help.WriteString("  n: New Test\n")
	help.WriteString("  h: Toggle Help\n")
	help.WriteString("  q: Quit\n")
	help.WriteString("\nIn Server Selection:\n")
	help.WriteString("  ↑/↓, j/k: Navigate\n")
	help.WriteString("  Enter: Select Server\n")

	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		helpStyle.Render(help.String()))
}

func RenderServerSelection(m *model.Model, width int) string {
	var b strings.Builder
	b.WriteString("Select a server:\n\n")

	for i, server := range m.ServerList {
		if m.Cursor == i {
			b.WriteString(fmt.Sprintf("> %s: %s (%s) - %.2f ms\n", server.Sponsor, server.Name, server.Country, server.Latency.Seconds()*1000))
		} else {
			b.WriteString(fmt.Sprintf("  %s: %s (%s) - %.2f ms\n", server.Sponsor, server.Name, server.Country, server.Latency.Seconds()*1000))
		}
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center, infoStyle.Render(b.String()))
}
