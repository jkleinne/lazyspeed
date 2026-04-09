package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/jkleinne/lazyspeed/diag"
)

func TestRenderDiagInput(t *testing.T) {
	// Simulate textinput.View() output — it's just a string
	inputView := "Target: █"

	out := RenderDiagInput(inputView, 80)

	if !strings.Contains(out, "Enter target hostname or IP address") {
		t.Error("expected instruction text in output")
	}
	if !strings.Contains(out, "press Enter for default") {
		t.Error("expected default hint in output")
	}
	if !strings.Contains(out, "Target:") {
		t.Error("expected input field in output")
	}
}

func TestRenderDiagCompact(t *testing.T) {
	result := &diag.Result{
		Target: "example.com",
		Method: "udp",
		Hops: []diag.Hop{
			{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 1 * time.Millisecond},
			{Number: 2, IP: "93.184.216.34", Host: "example.com", Latency: 25 * time.Millisecond},
		},
		DNS:     &diag.DNSResult{Host: "example.com", IP: "93.184.216.34", Latency: 20 * time.Millisecond, Cached: false},
		Quality: diag.QualityScore{Score: 85, Grade: "B", Label: "Good for most activities"},
	}

	out := RenderDiagCompact(result, 80)
	if !strings.Contains(out, "85") {
		t.Error("expected score in output")
	}
	if !strings.Contains(out, "B") {
		t.Error("expected grade in output")
	}
	if !strings.Contains(out, "Good for most activities") {
		t.Error("expected label in output")
	}
}

func TestRenderDiagExpanded(t *testing.T) {
	result := &diag.Result{
		Target: "example.com",
		Method: "udp",
		Hops: []diag.Hop{
			{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 1 * time.Millisecond},
			{Number: 2, IP: "10.0.0.2", Host: "isp", Latency: 75 * time.Millisecond},
			{Number: 3, Timeout: true},
			{Number: 4, IP: "93.184.216.34", Host: "example.com", Latency: 120 * time.Millisecond},
		},
		Quality: diag.QualityScore{Score: 60, Grade: "C", Label: "Adequate for browsing, poor for real-time"},
	}

	out := RenderDiagExpanded(result, 100, 30, 0)
	if !strings.Contains(out, "10.0.0.1") {
		t.Error("expected hop IP in output")
	}
	if !strings.Contains(out, "timeout") {
		t.Error("expected timeout hop in output")
	}
}

func TestRenderDiagCompactNilDNS(t *testing.T) {
	result := &diag.Result{
		Target:  "8.8.8.8",
		Method:  "icmp",
		Hops:    []diag.Hop{{Number: 1, IP: "8.8.8.8", Host: "dns.google", Latency: 10 * time.Millisecond}},
		Quality: diag.QualityScore{Score: 95, Grade: "A", Label: "Great for streaming and video calls"},
	}

	out := RenderDiagCompact(result, 80)
	if !strings.Contains(out, "N/A") {
		t.Error("expected N/A for DNS when nil")
	}
}

func TestRouteStatusStyled(t *testing.T) {
	tests := []struct {
		name     string
		hops     []diag.Hop
		contains string
	}{
		{
			"healthy with no hops",
			nil,
			"healthy",
		},
		{
			"healthy with no timeouts",
			[]diag.Hop{
				{Number: 1, IP: "10.0.0.1", Latency: 5 * time.Millisecond},
			},
			"healthy",
		},
		{
			"1 timeout",
			[]diag.Hop{
				{Number: 1, IP: "10.0.0.1", Latency: 5 * time.Millisecond},
				{Number: 2, Timeout: true},
			},
			"1 timeout",
		},
		{
			"multiple timeouts",
			[]diag.Hop{
				{Number: 1, Timeout: true},
				{Number: 2, Timeout: true},
				{Number: 3, IP: "10.0.0.1", Latency: 5 * time.Millisecond},
			},
			"2 timeouts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := routeStatusStyled(tt.hops)
			if !strings.Contains(out, tt.contains) {
				t.Errorf("expected output to contain %q, got %q", tt.contains, out)
			}
		})
	}
}

func TestRenderDiagExpandedAlternatingRows(t *testing.T) {
	result := &diag.Result{
		Target: "example.com",
		Method: "udp",
		Hops: []diag.Hop{
			{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 1 * time.Millisecond},
			{Number: 2, IP: "10.0.0.2", Host: "isp", Latency: 8 * time.Millisecond},
			{Number: 3, IP: "10.0.0.3", Host: "core", Latency: 15 * time.Millisecond},
			{Number: 4, IP: "10.0.0.4", Host: "edge", Latency: 22 * time.Millisecond},
		},
		Quality: diag.QualityScore{Score: 90, Grade: "A", Label: "Great for streaming and video calls"},
	}

	out := RenderDiagExpanded(result, 100, 30, 0)

	// Verify separator line exists
	if !strings.Contains(out, "─") {
		t.Error("expected separator line in output")
	}

	// Verify all hops still present
	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4"} {
		if !strings.Contains(out, ip) {
			t.Errorf("expected hop IP %s in output", ip)
		}
	}

	// Verify even and odd rows produce different ANSI output
	evenRendered := diagEvenRowStyle.Render("X")
	oddRendered := diagOddRowStyle.Render("X")
	if evenRendered == oddRendered {
		t.Error("expected diagEvenRowStyle and diagOddRowStyle to produce different output")
	}
}

func TestRenderDiagExpandedEmptyHops(t *testing.T) {
	result := &diag.Result{
		Target:  "example.com",
		Method:  "udp",
		Hops:    []diag.Hop{},
		Quality: diag.QualityScore{Score: 50, Grade: "C", Label: "Adequate for browsing, poor for real-time"},
	}

	out := RenderDiagExpanded(result, 100, 30, 0)

	// Should not panic and should still contain header elements
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(out, "50") {
		t.Error("expected score in output")
	}
	if !strings.Contains(out, "example.com") {
		t.Error("expected target in output")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"no truncation needed", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"standard ASCII truncation", "hello world", 6, "hello…"},
		{"multi-byte runes", "日本語テスト", 4, "日本語…"},
		{"maxLen 1 returns first rune", "hello", 1, "h"},
		{"maxLen 0 returns empty", "hello", 0, ""},
		{"empty string", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFindAnomalies(t *testing.T) {
	tests := []struct {
		name      string
		hops      []diag.Hop
		wantCount int
	}{
		{
			"no anomalies with uniform latency",
			[]diag.Hop{
				{Number: 1, Latency: 10 * time.Millisecond},
				{Number: 2, Latency: 12 * time.Millisecond},
				{Number: 3, Latency: 11 * time.Millisecond},
			},
			0,
		},
		{
			"anomaly exceeds both thresholds",
			[]diag.Hop{
				{Number: 1, Latency: 10 * time.Millisecond},
				{Number: 2, Latency: 12 * time.Millisecond},
				{Number: 3, Latency: 200 * time.Millisecond}, // >2x median(10,11,12) and >50ms
			},
			1,
		},
		{
			"high multiplier but below absolute floor",
			[]diag.Hop{
				{Number: 1, Latency: 5 * time.Millisecond},
				{Number: 2, Latency: 6 * time.Millisecond},
				{Number: 3, Latency: 20 * time.Millisecond}, // >2x median but <50ms
			},
			0,
		},
		{
			"all hops timeout returns nil",
			[]diag.Hop{
				{Number: 1, Timeout: true},
				{Number: 2, Timeout: true},
			},
			0,
		},
		{
			"mixed timeouts with anomaly",
			[]diag.Hop{
				{Number: 1, Latency: 10 * time.Millisecond},
				{Number: 2, Timeout: true},
				{Number: 3, Latency: 12 * time.Millisecond},
				{Number: 4, Latency: 150 * time.Millisecond}, // >2x median(10,12)=11 and >50ms
			},
			1,
		},
		{
			"even number of latencies for median",
			[]diag.Hop{
				{Number: 1, Latency: 10 * time.Millisecond},
				{Number: 2, Latency: 20 * time.Millisecond},
				{Number: 3, Latency: 30 * time.Millisecond},
				{Number: 4, Latency: 200 * time.Millisecond}, // median of 10,20,30,200 = (20+30)/2=25, 200 > 2*25 and >50
			},
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findAnomalies(tt.hops)
			if len(got) != tt.wantCount {
				t.Errorf("findAnomalies() returned %d anomalies, want %d", len(got), tt.wantCount)
			}
		})
	}
}

func TestRenderDiagCompactAnomalyWarnings(t *testing.T) {
	result := &diag.Result{
		Target: "example.com",
		Method: "udp",
		Hops: []diag.Hop{
			{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 10 * time.Millisecond},
			{Number: 2, IP: "10.0.0.2", Host: "isp", Latency: 12 * time.Millisecond},
			{Number: 3, IP: "10.0.0.3", Host: "slow-hop", Latency: 200 * time.Millisecond},
		},
		DNS:     &diag.DNSResult{Host: "example.com", IP: "93.184.216.34", Latency: 20 * time.Millisecond},
		Quality: diag.QualityScore{Score: 70, Grade: "B", Label: "Good for most activities"},
	}

	out := RenderDiagCompact(result, 120)
	if !strings.Contains(out, "Warning") {
		t.Error("expected anomaly warning in output")
	}
	if !strings.Contains(out, "hop 3") {
		t.Error("expected hop number in anomaly warning")
	}
	if !strings.Contains(out, "10.0.0.3") {
		t.Error("expected hop IP in anomaly warning")
	}
}

func TestDnsDisplayStr(t *testing.T) {
	tests := []struct {
		name string
		dns  *diag.DNSResult
		want string
	}{
		{"nil DNS", nil, "N/A"},
		{"cold DNS", &diag.DNSResult{Latency: 20 * time.Millisecond}, "20ms (cold)"},
		{"cached DNS", &diag.DNSResult{Latency: 2 * time.Millisecond, Cached: true}, "2ms (cached)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dnsDisplayStr(tt.dns)
			if !strings.Contains(got, tt.want) {
				t.Errorf("dnsDisplayStr() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestScoreStyle(t *testing.T) {
	tests := []struct {
		name  string
		grade string
	}{
		{"grade A is green", "A"},
		{"grade B is amber", "B"},
		{"grade C is amber", "C"},
		{"grade D is red", "D"},
		{"grade F is red", "F"},
		{"unknown grade is purple", "Z"},
		{"empty grade is purple", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := scoreStyle(tt.grade)
			rendered := style.Render("test")
			if rendered == "" {
				t.Error("expected non-empty styled output")
			}
			if rendered == "test" {
				t.Error("expected styled output to differ from plain text")
			}
		})
	}
}

func TestRenderHopRowVisualWidth(t *testing.T) {
	hop := diag.Hop{
		Number:  3,
		IP:      "10.0.0.1",
		Host:    "gateway.local",
		Latency: 25 * time.Millisecond,
	}

	rendered := renderHopRow(hop, 0)
	// Strip the outer row style (diagEvenRowStyle/diagOddRowStyle) to measure
	// the inner content width. The row styles only add foreground/background
	// color, not padding or margins, so stripping ANSI gives us the plain text
	// whose length equals the visual width before row styling.
	plain := ansi.Strip(rendered)
	if len([]rune(plain)) != hopTableWidth {
		t.Errorf("visual width = %d, want %d", len([]rune(plain)), hopTableWidth)
	}
}
