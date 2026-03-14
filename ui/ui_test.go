package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderTitle(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		expected string
	}{
		{
			name:  "Zero width",
			width: 0,
		},
		{
			name:  "Negative width",
			width: -10,
		},
		{
			name:  "Small width",
			width: 20,
		},
		{
			name:  "Exact width",
			width: 37,
		},
		{
			name:  "Large width",
			width: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderTitle(tt.width)

			expectedText := "LazySpeed - Terminal Speed Test"
			if !strings.Contains(result, expectedText) {
				t.Errorf("RenderTitle() = %q, want to contain %q", result, expectedText)
			}

			actualWidth := lipgloss.Width(result)

			// The raw text is 33 chars (" LazySpeed - Terminal Speed Test ")
			// Padding is 2 left, 2 right => total base width 37.
			baseWidth := 37

			expectedWidth := tt.width
			if tt.width < baseWidth {
				expectedWidth = baseWidth
			}

			if actualWidth != expectedWidth {
				t.Errorf("RenderTitle() width = %d, want %d", actualWidth, expectedWidth)
			}
		})
	}
}
