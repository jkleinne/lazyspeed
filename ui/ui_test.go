package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/muesli/termenv"
)

var testTimestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

func TestMain(m *testing.M) {
	// Ensure deterministic rendering for tests regardless of terminal
	lipgloss.SetHasDarkBackground(true)
	lipgloss.SetColorProfile(termenv.TrueColor)
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

			if result == "" {
				t.Errorf("RenderTitle(%d) returned empty string", tt.width)
			}

			// Strip ANSI codes before checking text content — gradient renders
			// each character with its own escape sequence.
			plain := ansi.Strip(result)
			if !strings.Contains(plain, "LazySpeed") {
				t.Errorf("RenderTitle(%d) plain text = %q, want to contain %q", tt.width, plain, "LazySpeed")
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
	res := RenderResults(m.History.Entries, Viewport{Width: 100})
	if res != "" {
		t.Errorf("Expected empty string for empty history, got %q", res)
	}

	// Case 2: One entry
	m.History.Entries = []*model.SpeedTestResult{
		{
			DownloadSpeed: 100.0,
			UploadSpeed:   50.0,
			Ping:          10.5,
			Jitter:        2.1,
			ServerName:    "Test Server",
			ServerCountry: "US",
			Timestamp:     testTimestamp,
		},
	}
	res = RenderResults(m.History.Entries, Viewport{Width: 100})
	if !strings.Contains(res, "Latest Results") {
		t.Errorf("Expected Latest Test Results block")
	}
	if strings.Contains(res, "Previous Tests") {
		t.Errorf("Did not expect Previous Tests table for 1 result")
	}

	// Case 3: Two entries (adds table)
	m.History.Entries = append(m.History.Entries, &model.SpeedTestResult{
		DownloadSpeed: 200.0,
		UploadSpeed:   100.0,
		Ping:          5.0,
		ServerName:    "New Server",
		ServerSponsor: "Great Sponsor",
		Distance:      42.5,
		UserIP:        "1.1.1.1",
		UserISP:       "Cloudflare",
		Timestamp:     testTimestamp,
	})
	res = RenderResults(m.History.Entries, Viewport{Width: 100})
	if !strings.Contains(res, "Latest Results") {
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
	// Without a result: no export or scroll hint
	res := RenderHelp(100, false)
	if !strings.Contains(res, "Controls:") || !strings.Contains(res, "n: New Test") {
		t.Errorf("Expected help controls to be present")
	}
	if strings.Contains(res, "e: Export") {
		t.Errorf("Did not expect export hint when hasResult is false")
	}
	if strings.Contains(res, "Scroll History") {
		t.Errorf("Did not expect scroll hint when hasResult is false")
	}

	// With a result: export and scroll hints shown
	res = RenderHelp(100, true)
	if !strings.Contains(res, "e: Export Result") {
		t.Errorf("Expected export hint when hasResult is true")
	}
	if !strings.Contains(res, "Scroll History") {
		t.Errorf("Expected scroll history hint when hasResult is true")
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
	if !strings.Contains(res, "[Esc] Cancel") {
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
	m.Height = 40

	// Case 1: Empty list
	res := RenderServerSelection([]model.Server{}, Viewport{Width: 100, Height: m.Height})
	if !strings.Contains(res, "No servers available") {
		t.Errorf("Expected 'No servers available' for empty list")
	}

	// Case 2: Populated list
	servers := []model.Server{
		{Name: "Server 1", Sponsor: "Sponsor 1", Country: "Country 1", Latency: 10 * time.Millisecond},
		{Name: "Server 2", Sponsor: "Sponsor 2", Country: "Country 2", Latency: 20 * time.Millisecond},
	}

	res = RenderServerSelection(servers, Viewport{Width: 100, Height: m.Height, Cursor: 1})
	plain := ansi.Strip(res)
	if !strings.Contains(plain, "▸") {
		t.Errorf("Expected cursor indicator '▸' on selected row")
	}
	if !strings.Contains(plain, "Sponsor 2") {
		t.Errorf("Expected selected sponsor 'Sponsor 2' to be present")
	}
	if !strings.Contains(plain, "Sponsor 1") {
		t.Errorf("Expected unselected sponsor 'Sponsor 1' to be present")
	}
}

func TestRenderServerSelectionViewport(t *testing.T) {
	tests := []struct {
		name          string
		serverCount   int
		cursor        int
		offset        int
		height        int
		wantUpArrow   bool
		wantDownArrow bool
	}{
		{
			name:        "All servers fit",
			serverCount: 3,
			cursor:      0,
			offset:      0,
			height:      30,
		},
		{
			name:          "Scrolled down shows both arrows",
			serverCount:   20,
			cursor:        5,
			offset:        3,
			height:        20,
			wantUpArrow:   true,
			wantDownArrow: true,
		},
		{
			name:        "At bottom shows only up arrow",
			serverCount: 20,
			cursor:      19,
			offset:      8,
			height:      20,
			wantUpArrow: true,
		},
		{
			name:        "Empty server list",
			serverCount: 0,
			cursor:      0,
			offset:      0,
			height:      20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers := make([]model.Server, tt.serverCount)
			for i := range servers {
				servers[i] = model.Server{
					Name:    fmt.Sprintf("Server %d", i),
					Sponsor: fmt.Sprintf("Sponsor %d", i),
					Country: "US",
					Latency: time.Duration(10+i) * time.Millisecond,
				}
			}

			res := RenderServerSelection(servers, Viewport{Width: 100, Height: tt.height, Offset: tt.offset, Cursor: tt.cursor})
			plain := ansi.Strip(res)

			// Scroll indicators include a count: "↑ N more" / "↓ N more".
			// The hint bar always contains "↑/↓" so we match on the indicator pattern.
			if tt.wantUpArrow && !strings.Contains(plain, "↑") {
				t.Errorf("Expected up arrow scroll indicator")
			}
			if !tt.wantUpArrow && strings.Contains(plain, "↑ ") {
				t.Errorf("Did not expect up arrow scroll indicator")
			}
			if tt.wantDownArrow && !strings.Contains(plain, "↓") {
				t.Errorf("Expected down arrow scroll indicator")
			}
			if !tt.wantDownArrow && strings.Contains(plain, "↓ ") {
				t.Errorf("Did not expect down arrow scroll indicator")
			}
		})
	}
}

func TestRenderResultsMissingSponsorDistance(t *testing.T) {
	m := model.NewDefaultModel()
	m.History.Entries = []*model.SpeedTestResult{
		{
			DownloadSpeed: 80.0,
			UploadSpeed:   40.0,
			Ping:          15.0,
			Jitter:        2.0,
			ServerName:    "Old Server",
			ServerCountry: "DE",
			ServerSponsor: "",
			Distance:      0,
			Timestamp:     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			DownloadSpeed: 100.0,
			UploadSpeed:   50.0,
			Ping:          10.0,
			Jitter:        1.0,
			ServerName:    "New Server",
			ServerCountry: "US",
			ServerSponsor: "Valid Sponsor",
			Distance:      42.5,
			Timestamp:     time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
		},
	}

	res := RenderResults(m.History.Entries, Viewport{Width: 120, Height: m.Height})

	if !strings.Contains(res, "Previous Tests") {
		t.Errorf("Expected 'Previous Tests' label for 2 entries")
	}
	// Old server entry should appear in the history table
	if !strings.Contains(res, "Old Server") {
		t.Errorf("Expected 'Old Server' in table row")
	}
	// Latest entry's sponsor should appear in the results box
	if !strings.Contains(res, "Valid Sponsor") {
		t.Errorf("Expected 'Valid Sponsor' in latest results")
	}
	if !strings.Contains(res, "42.5 km") {
		t.Errorf("Expected '42.5 km' distance in latest results")
	}
	// Old entry has zero distance — table should render "-", not "0.0"
	if strings.Contains(res, "0.0 km") {
		t.Errorf("Expected zero distance to render as '-', not '0.0 km'")
	}
}

func TestRenderResultsManyEntries(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 60
	m.History.Entries = make([]*model.SpeedTestResult, 5)
	for i := range m.History.Entries {
		m.History.Entries[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			UploadSpeed:   float64(50 + i),
			Ping:          float64(10 + i),
			Jitter:        1.0,
			ServerName:    "TestServer",
			ServerCountry: "US",
			Timestamp:     testTimestamp,
		}
	}

	res := RenderResults(m.History.Entries, Viewport{Width: 120, Height: m.Height})
	if !strings.Contains(res, "Previous Tests") {
		t.Errorf("Expected 'Previous Tests' label in output")
	}
}

func TestRenderResultsPagination(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 30
	m.History.Entries = make([]*model.SpeedTestResult, 20)
	for i := range m.History.Entries {
		m.History.Entries[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			UploadSpeed:   float64(50 + i),
			Ping:          float64(10 + i),
			Jitter:        1.0,
			ServerName:    "TestServer",
			ServerCountry: "US",
			Timestamp:     testTimestamp,
		}
	}

	res := RenderResults(m.History.Entries, Viewport{Width: 120, Height: m.Height})
	if !strings.Contains(res, "Showing") {
		t.Errorf("Expected pagination indicator for large history")
	}
	if !strings.Contains(res, "Previous Tests") {
		t.Errorf("Expected Previous Tests label")
	}
}

func TestRenderResultsNoPaginationSmallHistory(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 60
	m.History.Entries = make([]*model.SpeedTestResult, 3)
	for i := range m.History.Entries {
		m.History.Entries[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			UploadSpeed:   float64(50 + i),
			Ping:          float64(10 + i),
			Jitter:        1.0,
			ServerName:    "TestServer",
			ServerCountry: "US",
			Timestamp:     testTimestamp,
		}
	}

	res := RenderResults(m.History.Entries, Viewport{Width: 120, Height: m.Height})
	if strings.Contains(res, "Showing") {
		t.Errorf("Did not expect pagination indicator when all rows fit")
	}
}

func TestRenderResultsWithHistoryOffset(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 30
	m.History.Entries = make([]*model.SpeedTestResult, 20)
	for i := range m.History.Entries {
		m.History.Entries[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			UploadSpeed:   float64(50 + i),
			Ping:          float64(10 + i),
			Jitter:        1.0,
			ServerName:    fmt.Sprintf("Server%d", i),
			ServerCountry: "US",
			Timestamp:     testTimestamp,
		}
	}
	res := RenderResults(m.History.Entries, Viewport{Width: 120, Height: m.Height, Offset: 3})
	if !strings.Contains(res, "Showing 4-") {
		t.Errorf("Expected pagination to start at 4 with offset 3, got: %s", res)
	}
}

func TestRenderResultsHistoryOffsetClamped(t *testing.T) {
	m := model.NewDefaultModel()
	m.Height = 30
	m.History.Entries = make([]*model.SpeedTestResult, 20)
	for i := range m.History.Entries {
		m.History.Entries[i] = &model.SpeedTestResult{
			DownloadSpeed: float64(100 + i),
			UploadSpeed:   float64(50 + i),
			Ping:          float64(10 + i),
			Jitter:        1.0,
			ServerName:    "TestServer",
			ServerCountry: "US",
			Timestamp:     testTimestamp,
		}
	}
	res := RenderResults(m.History.Entries, Viewport{Width: 120, Height: m.Height, Offset: 999})
	if !strings.Contains(res, "Showing") {
		t.Errorf("Expected pagination indicator even with clamped offset")
	}
	// Should show the last page, ending at totalRows (19)
	if !strings.Contains(res, "of 19") {
		t.Errorf("Expected 'of 19' in pagination, got: %s", res)
	}
}

func TestServerListVisibleLines(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		total    int
		expected int
	}{
		{"Large terminal, few servers", 40, 5, 5},
		{"Small terminal, many servers", 15, 30, 7},
		{"Tiny terminal enforces minimum", 5, 30, 3},
		{"Zero height enforces minimum", 0, 30, 3},
		{"Total less than visible", 40, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ServerListVisibleLines(tt.height, tt.total)
			if got != tt.expected {
				t.Errorf("ServerListVisibleLines(%d, %d) = %d, want %d",
					tt.height, tt.total, got, tt.expected)
			}
		})
	}
}

func TestHistoryVisibleRows(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		total    int
		expected int
	}{
		{"Large terminal, few rows", 60, 5, 5},
		{"Small terminal, many rows", 30, 20, 8},
		{"Tiny terminal enforces minimum", 10, 20, 3},
		{"Zero height enforces minimum", 0, 20, 3},
		{"Total less than visible", 60, 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HistoryVisibleRows(tt.height, tt.total)
			if got != tt.expected {
				t.Errorf("HistoryVisibleRows(%d, %d) = %d, want %d",
					tt.height, tt.total, got, tt.expected)
			}
		})
	}
}

func TestSpinnerBoxWidth(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		expected  int
	}{
		{"Narrow terminal clamps to min", 30, 40},
		{"Medium terminal", 60, 50},
		{"Standard terminal", 90, 80},
		{"Wide terminal clamps to max", 200, 80},
		{"Exact boundary low", 50, 40},
		{"Exact boundary high", 90, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spinnerBoxWidth(tt.termWidth)
			if got != tt.expected {
				t.Errorf("spinnerBoxWidth(%d) = %d, want %d",
					tt.termWidth, got, tt.expected)
			}
		})
	}
}

func TestNewProgress(t *testing.T) {
	tests := []struct {
		name     string
		boxWidth int
	}{
		{"Standard box width", 70},
		{"Minimum box width", 40},
		{"Very small box triggers floor", 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newProgress(tt.boxWidth)
			// ViewAs should not panic for any box width
			out := p.ViewAs(0.5)
			if out == "" {
				t.Error("expected non-empty progress bar output")
			}
		})
	}
}
