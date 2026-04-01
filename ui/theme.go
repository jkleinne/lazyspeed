package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// Color palette — single source of truth for the entire UI.
const (
	// Purple palette
	colorPurpleDark  = "#5B3CC4"
	colorPurple      = "#7D56F4"
	colorPurpleLight = "#9F7AEA"
	colorPurpleMuted = "#B794F6"

	// Surfaces
	colorSurfaceDark  = "#1a1a2e"
	colorSurfaceLight = "#2a2a3e"

	// Text
	colorTextPrimary   = "#FAFAFA"
	colorTextSecondary = "#888888"
	colorTextDim       = "#555555"

	// Status (unchanged)
	colorError       = "#FF0000"
	colorWarning     = "#FFA500"
	colorStatusGreen = "#22c55e"
	colorStatusAmber = "#f59e0b"
	colorStatusRed   = "#ef4444"
)

var (
	// Banner styles — used by the new bordered title
	bannerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorPurple)).
			Background(lipgloss.Color(colorSurfaceDark)).
			PaddingLeft(2).
			PaddingRight(2)

	// Text styles
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextPrimary))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorWarning))

	hintDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextSecondary))

	hintKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurpleLight))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim))

	// Spinner
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurple))

	// Results box
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorPurple)).
			Background(lipgloss.Color(colorSurfaceDark)).
			PaddingLeft(1).
			PaddingRight(1)

	// Section headers
	sectionLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurpleLight)).
				Bold(true)

	// Latest results styling
	metricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurpleMuted)).
				Bold(true)

	metadataStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextSecondary))

	// Table styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPurpleLight)).
			Padding(0, 1)

	evenRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextPrimary)).
			Background(lipgloss.Color(colorSurfaceDark)).
			Padding(0, 1)

	oddRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextSecondary)).
			Background(lipgloss.Color(colorSurfaceLight)).
			Padding(0, 1)

	tableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurple))

	// Server selection
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextPrimary)).
				Background(lipgloss.Color(colorSurfaceLight)).
				Bold(true)

	selectedAccentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurple)).
				Background(lipgloss.Color(colorSurfaceLight)).
				Bold(true)

	unselectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextSecondary))

	// Spinner box & progress
	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurple)).
			PaddingLeft(2).
			PaddingRight(2)

	phaseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurpleLight))

	phaseValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurpleMuted)).
			Bold(true)

	// Diagnostics styles
	latencyGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorStatusGreen))

	latencyAmberStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorStatusAmber))

	latencyRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorStatusRed))

	diagHeaderLightStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorPurpleLight))

	diagSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurpleDark))

	diagEvenRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextPrimary)).
				Background(lipgloss.Color(colorSurfaceDark))

	diagOddRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextSecondary)).
			Background(lipgloss.Color(colorSurfaceLight))

	diagTargetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurpleMuted))

	diagLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPurpleLight))

	diagSummaryLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextSecondary))

	diagSummarySepStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurpleDark))
)

var (
	scoreGreenStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorStatusGreen))
	scoreAmberStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorStatusAmber))
	scoreRedStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorStatusRed))
	scoreDefaultStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPurple))
)

// scoreStyle returns a bold style color-coded by grade.
func scoreStyle(grade string) lipgloss.Style {
	switch grade {
	case "A":
		return scoreGreenStyle
	case "B", "C":
		return scoreAmberStyle
	case "D", "F":
		return scoreRedStyle
	default:
		return scoreDefaultStyle
	}
}

const (
	latencyGoodMs = 50
	latencyFairMs = 100
)

// latencyStyle returns the appropriate style for a given latency.
func latencyStyle(d time.Duration) lipgloss.Style {
	ms := d.Milliseconds()
	switch {
	case ms < latencyGoodMs:
		return latencyGreenStyle
	case ms <= latencyFairMs:
		return latencyAmberStyle
	default:
		return latencyRedStyle
	}
}

// gradientColors is the banner gradient palette.
var gradientColors = []lipgloss.Color{
	lipgloss.Color(colorPurpleDark),
	lipgloss.Color(colorPurple),
	lipgloss.Color(colorPurpleLight),
	lipgloss.Color(colorPurpleMuted),
}

// gradientText renders each character of text with an interpolated color from colors.
func gradientText(text string, colors []lipgloss.Color) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 || len(colors) == 0 {
		return text
	}
	if len(colors) == 1 {
		return lipgloss.NewStyle().Foreground(colors[0]).Render(text)
	}

	var b strings.Builder
	for i, r := range runes {
		// Map character index to a position in [0, len(colors)-1]
		pos := float64(i) / float64(max(1, n-1)) * float64(len(colors)-1)
		idx := min(int(pos), len(colors)-1)
		style := lipgloss.NewStyle().Foreground(colors[idx])
		b.WriteString(style.Render(string(r)))
	}
	return b.String()
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
		progress.WithGradient(colorPurpleDark, colorPurpleMuted),
		progress.WithWidth(barWidth),
		progress.WithoutPercentage(),
	)
}
