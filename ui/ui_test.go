package ui

import (
	"strings"
	"testing"
)

func TestRenderHelp(t *testing.T) {
	width := 80
	result := RenderHelp(width)

	expectedStrings := []string{
		"Controls:",
		"n: New Test",
		"h: Toggle Help",
		"q: Quit",
		"In Server Selection:",
		"↑/↓, j/k: Navigate",
		"Enter: Select Server",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(result, expected) {
			t.Errorf("RenderHelp() missing expected string %q", expected)
		}
	}
}
