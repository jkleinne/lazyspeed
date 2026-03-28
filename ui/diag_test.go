package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/diag"
)

func TestRenderDiagCompact(t *testing.T) {
	result := &diag.DiagResult{
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
	result := &diag.DiagResult{
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
	result := &diag.DiagResult{
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
	result := &diag.DiagResult{
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
	result := &diag.DiagResult{
		Target:  "example.com",
		Method:  "udp",
		Hops:    []diag.Hop{},
		Quality: diag.QualityScore{Score: 50, Grade: "C", Label: "Adequate for browsing, poor for real-time"},
	}

	out := RenderDiagExpanded(result, 100, 30, 0)

	// Should not panic and should still contain header elements
	if !strings.Contains(out, "Network Diagnostics") {
		t.Error("expected title in output")
	}
	if !strings.Contains(out, "example.com") {
		t.Error("expected target in output")
	}
}

func TestTruncateMultiByteRunes(t *testing.T) {
	// "日本語テスト" is 6 runes but 18 bytes. The old byte-based truncate(s, 4)
	// would slice at byte 3, corrupting the string. The rune-based fix
	// correctly keeps 3 full runes + "…".
	got := Truncate("日本語テスト", 4)
	want := "日本語…"
	if got != want {
		t.Errorf("Truncate(CJK, 4) = %q, want %q", got, want)
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
