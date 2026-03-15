package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/showwin/speedtest-go/speedtest"
)

func TestMain(m *testing.M) {
	// Ensure deterministic rendering for tests regardless of terminal
	lipgloss.SetHasDarkBackground(true)
	m.Run()
}

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

func TestRenderSpinner(t *testing.T) {
	res := RenderSpinner(DefaultSpinner, 100, "Testing phase", 0.5)
	if !strings.Contains(res, "Testing phase") {
		t.Errorf("Expected phase text to be present")
	}

	// Negative progress
	res = RenderSpinner(DefaultSpinner, 100, "Testing phase", -0.1)
	if !strings.Contains(res, "Testing phase") {
		t.Errorf("Should handle negative progress gracefully")
	}
}

func TestRenderResults(t *testing.T) {
	m := model.NewDefaultModel()

	// Case 1: Empty history
	res := RenderResults(m, 100)
	if res != "" {
		t.Errorf("Expected empty string for empty history, got %q", res)
	}

	// Case 2: One entry
	m.TestHistory = []*model.SpeedTestResult{
		{
			DownloadSpeed: 100.0,
			UploadSpeed:   50.0,
			Ping:          10.5,
			Jitter:        2.1,
			ServerName:    "Test Server",
			ServerLoc:     "Test City",
			Timestamp:     time.Now(),
		},
	}
	res = RenderResults(m, 100)
	if !strings.Contains(res, "Latest Test Results:") {
		t.Errorf("Expected Latest Test Results block")
	}
	if strings.Contains(res, "Previous Tests") {
		t.Errorf("Did not expect Previous Tests table for 1 result")
	}

	// Case 3: Two entries (adds table)
	m.TestHistory = append(m.TestHistory, &model.SpeedTestResult{
		DownloadSpeed: 200.0,
		UploadSpeed:   100.0,
		Ping:          5.0,
		ServerName:    "New Server",
		ServerSponsor: "Great Sponsor",
		Distance:      42.5,
		UserIP:        "1.1.1.1",
		UserISP:       "Cloudflare",
		Timestamp:     time.Now(),
	})
	res = RenderResults(m, 100)
	if !strings.Contains(res, "Latest Test Results:") {
		t.Errorf("Expected Latest Test Results block")
	}
	if !strings.Contains(res, "Previous Tests") {
		t.Errorf("Expected Previous Tests table for 2+ results")
	}
	if !strings.Contains(res, "Great Sponsor") {
		t.Errorf("Expected sponsor to be printed")
	}
	if !strings.Contains(res, "42.5 km") {
		t.Errorf("Expected distance to be printed")
	}
	if !strings.Contains(res, "1.1.1.1 (Cloudflare)") {
		t.Errorf("Expected IP and ISP to be printed")
	}
}

func TestRenderError(t *testing.T) {
	if RenderError(nil, 100) != "" {
		t.Errorf("Expected empty string for nil error")
	}

	res := RenderError(errors.New("test error"), 100)
	if !strings.Contains(res, "Error: test error") {
		t.Errorf("Expected error message to be present")
	}
}

func TestRenderWarning(t *testing.T) {
	if RenderWarning("", 100) != "" {
		t.Errorf("Expected empty string for empty warning")
	}

	res := RenderWarning("test warning", 100)
	if !strings.Contains(res, "Warning: test warning") {
		t.Errorf("Expected warning message to be present")
	}
}

func TestRenderHelp(t *testing.T) {
	// Without a result: no export hint
	res := RenderHelp(100, false)
	if !strings.Contains(res, "Controls:") || !strings.Contains(res, "n: New Test") {
		t.Errorf("Expected help controls to be present")
	}
	if strings.Contains(res, "e: Export") {
		t.Errorf("Did not expect export hint when hasResult is false")
	}

	// With a result: export hint shown
	res = RenderHelp(100, true)
	if !strings.Contains(res, "e: Export Result") {
		t.Errorf("Expected export hint when hasResult is true")
	}
}

func TestRenderExportPrompt(t *testing.T) {
	res := RenderExportPrompt(100)
	if !strings.Contains(res, "[j] JSON") {
		t.Errorf("Expected JSON option in export prompt")
	}
	if !strings.Contains(res, "[c] CSV") {
		t.Errorf("Expected CSV option in export prompt")
	}
	if !strings.Contains(res, "[Esc] cancel") {
		t.Errorf("Expected cancel option in export prompt")
	}

	// Zero width should not panic
	res = RenderExportPrompt(0)
	if !strings.Contains(res, "[j] JSON") {
		t.Errorf("Expected JSON option even at zero width")
	}
}

func TestRenderExportMessage(t *testing.T) {
	// Empty message returns empty string
	if RenderExportMessage("", 100) != "" {
		t.Errorf("Expected empty string for empty message")
	}

	// Non-empty message is rendered
	res := RenderExportMessage("Saved to /tmp/lazyspeed_20260101_120000.json", 100)
	if !strings.Contains(res, "Saved to") {
		t.Errorf("Expected message text in render output")
	}
}

func TestRenderServerSelection(t *testing.T) {
	m := model.NewDefaultModel()

	// Case 1: Empty list
	res := RenderServerSelection(m, 100)
	if !strings.Contains(res, "Select a server:") {
		t.Errorf("Expected selection header")
	}

	// Case 2: Populated list
	m.ServerList = speedtest.Servers{
		&speedtest.Server{Name: "Server 1", Sponsor: "Sponsor 1", Country: "Country 1", Latency: 10 * time.Millisecond},
		&speedtest.Server{Name: "Server 2", Sponsor: "Sponsor 2", Country: "Country 2", Latency: 20 * time.Millisecond},
	}
	m.Cursor = 1

	res = RenderServerSelection(m, 100)
	if !strings.Contains(res, "> Sponsor 2") {
		t.Errorf("Expected cursor on Server 2")
	}
	if !strings.Contains(res, "  Sponsor 1") {
		t.Errorf("Expected no cursor on Server 1")
	}
}
