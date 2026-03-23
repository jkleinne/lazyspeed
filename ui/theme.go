package ui

import (
	"github.com/charmbracelet/bubbles/progress"
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
