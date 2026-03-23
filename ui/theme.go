package ui

import (
	"time"

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

	// Diagnostics styles
	diagTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorTextBright)).
			Background(lipgloss.Color(colorPrimary)).
			PaddingLeft(2).
			PaddingRight(2)

	latencyGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorStatusGreen))

	latencyAmberStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorStatusAmber))

	latencyRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorStatusRed))

	diagHeaderLightStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorPrimary))

	diagEvenRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTableEven))

	diagOddRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTableOdd))

	// Styles extracted from inline definitions in render functions
	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary)).
			PaddingLeft(2).
			PaddingRight(2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorPrimary)).
			PaddingLeft(1).
			PaddingRight(1)

	tableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPrimary))

	sectionLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPrimary)).
				Bold(true)
)

// scoreStyle returns a bold style color-coded by grade.
func scoreStyle(grade string) lipgloss.Style {
	switch grade {
	case "A":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorStatusGreen))
	case "B", "C":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorStatusAmber))
	case "D", "F":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorStatusRed))
	default:
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPrimary))
	}
}

// latencyStyle returns the appropriate style for a given latency.
func latencyStyle(d time.Duration) lipgloss.Style {
	ms := d.Milliseconds()
	switch {
	case ms < 50:
		return latencyGreenStyle
	case ms <= 100:
		return latencyAmberStyle
	default:
		return latencyRedStyle
	}
}

const (
	spinnerBoxMin       = 40
	spinnerBoxMax       = 80
	spinnerBoxMargin    = 10 // horizontal padding deducted from terminal width
	progressBarMin      = 10
	progressBarOverhead = 20 // border + padding deducted from box width
)

// spinnerBoxWidth returns a responsive spinner box width clamped to [spinnerBoxMin, spinnerBoxMax].
func spinnerBoxWidth(termWidth int) int {
	return max(spinnerBoxMin, min(spinnerBoxMax, termWidth-spinnerBoxMargin))
}

// newProgress creates a progress bar sized to the given spinner box width.
func newProgress(boxWidth int) progress.Model {
	barWidth := max(progressBarMin, boxWidth-progressBarOverhead)
	return progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(barWidth),
		progress.WithoutPercentage(),
	)
}
