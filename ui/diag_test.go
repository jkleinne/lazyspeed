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
