package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
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

	resultBox := strings.Builder{}
	// Pre-allocate buffer. ~250 bytes for latest test + ~85 bytes per history entry
	estimatedSize := 250
	if len(m.TestHistory) > 1 {
		estimatedSize += (len(m.TestHistory) - 1) * 85
	}
	resultBox.Grow(estimatedSize)

	resultBox.WriteString("\n")

	latest := m.TestHistory[len(m.TestHistory)-1]
	resultBox.WriteString("Latest Test Results:\n")
	resultBox.WriteString("──────────────────────\n")
	resultBox.WriteString(fmt.Sprintf("📥 Download: %.2f MBps\n", latest.DownloadSpeed))
	resultBox.WriteString(fmt.Sprintf("📤 Upload: %.2f MBps\n", latest.UploadSpeed))
	resultBox.WriteString(fmt.Sprintf("🔄 Ping: %.2f ms\n", latest.Ping))
	resultBox.WriteString(fmt.Sprintf("📊 Jitter: %.2f ms\n", latest.Jitter))
	resultBox.WriteString(fmt.Sprintf("🌍 Server: %s (%s)\n", latest.ServerName, latest.ServerLoc))
	resultBox.WriteString(fmt.Sprintf("🕒 Timestamp: %s\n", latest.Timestamp.Format("03:04:05 PM")))

	if len(m.TestHistory) > 1 {
		resultBox.WriteString("\nPrevious Tests:\n")
		resultBox.WriteString("──────────────\n")
		var buf [128]byte
		for i := len(m.TestHistory) - 2; i >= 0; i-- {
			test := m.TestHistory[i]
			b := buf[:0]
			b = append(b, '[')
			b = test.Timestamp.AppendFormat(b, "03:04:05 PM")
			b = append(b, "] DL: "...)
			b = strconv.AppendFloat(b, test.DownloadSpeed, 'f', 1, 64)
			b = append(b, " MBps, UL: "...)
			b = strconv.AppendFloat(b, test.UploadSpeed, 'f', 1, 64)
			b = append(b, " MBps, Ping: "...)
			b = strconv.AppendFloat(b, test.Ping, 'f', 1, 64)
			b = append(b, " ms, Jitter: "...)
			b = strconv.AppendFloat(b, test.Jitter, 'f', 1, 64)
			b = append(b, " ms\n"...)
			resultBox.Write(b)
		}
	}

	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		infoStyle.Render(resultBox.String()))
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
