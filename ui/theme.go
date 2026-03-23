package ui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// Color palette — single source of truth for the entire UI.
const (
	colorPrimary     = "#7D56F4"
	colorTextBright  = "#FAFAFA"
	colorTextDefault = "#FFFFFF"
	colorError       = "#FF0000"
	colorWarning     = "#FFA500"
	colorMuted       = "#626262"
	colorTableEven   = "#CCCCCC"
	colorTableOdd    = "#999999"
	colorStatusGreen = "#22c55e"
	colorStatusAmber = "#f59e0b"
	colorStatusRed   = "#ef4444"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorTextBright)).
			Background(lipgloss.Color(colorPrimary)).
			PaddingLeft(2).
			PaddingRight(2)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDefault))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary))

	// Table styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorTextBright)).
			Background(lipgloss.Color(colorPrimary)).
			Padding(0, 1)

	evenRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTableEven)).
			Padding(0, 1)

	oddRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTableOdd)).
			Padding(0, 1)
)

// spinnerBoxWidth returns a responsive spinner box width clamped to [40, 80].
func spinnerBoxWidth(termWidth int) int {
	return max(40, min(80, termWidth-10))
}

// newProgress creates a progress bar sized to the given spinner box width.
func newProgress(boxWidth int) progress.Model {
	barWidth := boxWidth - 20
	if barWidth < 10 {
		barWidth = 10
	}
	return progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(barWidth),
		progress.WithoutPercentage(),
	)
}
