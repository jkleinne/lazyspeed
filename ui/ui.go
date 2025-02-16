package ui

import (
	"fmt"
	"strings"

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

func RenderSpinner(s spinner.Model, width int, phase string) string {
	spinnerView := spinnerStyle.Render(s.View())
	phaseText := fmt.Sprintf("⏳ %s", phase)

	content := lipgloss.JoinHorizontal(lipgloss.Left,
		spinnerView,
		"  ",
		phaseText,
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
		for i := len(m.TestHistory) - 2; i >= 0; i-- {
			test := m.TestHistory[i]
			resultBox.WriteString(fmt.Sprintf("[%s] DL: %.1f MBps, UL: %.1f MBps, Ping: %.1f ms, Jitter: %.1f ms\n",
				test.Timestamp.Format("03:04:05 PM"),
				test.DownloadSpeed,
				test.UploadSpeed,
				test.Ping,
				test.Jitter))
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
	help.WriteString("Press 'n' to start a new test\n")
	help.WriteString("Press 'h' to toggle help\n")
	help.WriteString("Press 'q' to quit\n")

	return lipgloss.PlaceHorizontal(width, lipgloss.Center,
		helpStyle.Render(help.String()))
}
