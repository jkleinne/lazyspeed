package main

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/diag"
)

func TestDiagCmdFlagDefaults(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{"json default", "json", "false"},
		{"csv default", "csv", "false"},
		{"simple default", "simple", "false"},
		{"history default", "history", "false"},
		{"server default", "server", ""},
		{"last default", "last", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := diagCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag %q not found", tt.flag)
				return
			}
			if f.DefValue != tt.want {
				t.Errorf("flag %q default = %q, want %q", tt.flag, f.DefValue, tt.want)
			}
		})
	}
}

func TestDiagCmdExists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "diag [target]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("diag command not registered on rootCmd")
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"host with port", "192.168.1.1:443", "192.168.1.1"},
		{"IPv6 with port", "[::1]:80", "::1"},
		{"hostname without port", "example.com", "example.com"},
		{"IP without port", "10.0.0.1", "10.0.0.1"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPort(tt.input)
			if got != tt.want {
				t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDiagPacketLossPct(t *testing.T) {
	tests := []struct {
		name string
		hops []diag.Hop
		want float64
	}{
		{"nil hops", nil, 0},
		{"no timeouts", []diag.Hop{
			{Number: 1, Timeout: false},
			{Number: 2, Timeout: false},
		}, 0},
		{"all timeout", []diag.Hop{
			{Number: 1, Timeout: true},
			{Number: 2, Timeout: true},
		}, 100},
		{"one of three timeout", []diag.Hop{
			{Number: 1, Timeout: false},
			{Number: 2, Timeout: true},
			{Number: 3, Timeout: false},
		}, 100.0 / 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagPacketLossPct(tt.hops)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("diagPacketLossPct() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestDiagFinalHopLatencyMs(t *testing.T) {
	tests := []struct {
		name string
		hops []diag.Hop
		want float64
	}{
		{"nil hops", nil, 0},
		{"all timeout", []diag.Hop{
			{Number: 1, Timeout: true},
			{Number: 2, Timeout: true},
		}, 0},
		{"last hop valid", []diag.Hop{
			{Number: 1, Latency: 5 * time.Millisecond},
			{Number: 2, Latency: 10 * time.Millisecond},
		}, 10},
		{"last timeout skips to previous", []diag.Hop{
			{Number: 1, Latency: 5 * time.Millisecond},
			{Number: 2, Latency: 12 * time.Millisecond},
			{Number: 3, Timeout: true},
		}, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagFinalHopLatencyMs(tt.hops)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("diagFinalHopLatencyMs() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestDiagCSVRow(t *testing.T) {
	ts := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	t.Run("with DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target: "example.com",
			Method: diag.MethodICMP,
			Hops: []diag.Hop{
				{Number: 1, Latency: 5 * time.Millisecond},
			},
			DNS: &diag.DNSResult{
				Host:    "example.com",
				IP:      "93.184.216.34",
				Latency: 2 * time.Millisecond,
				Cached:  false,
			},
			Quality:   diag.QualityScore{Score: 85, Grade: "B", Label: "Good"},
			Timestamp: ts,
		}

		row := diagCSVRow(r)
		if len(row) != len(diagCSVHeader) {
			t.Fatalf("column count = %d, want %d", len(row), len(diagCSVHeader))
		}
		if row[1] != "example.com" {
			t.Errorf("target column = %q, want %q", row[1], "example.com")
		}
		if row[4] != "B" {
			t.Errorf("grade column = %q, want %q", row[4], "B")
		}
	})

	t.Run("nil DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target:    "10.0.0.1",
			Method:    diag.MethodUDP,
			Hops:      []diag.Hop{},
			DNS:       nil,
			Quality:   diag.QualityScore{Score: 70, Grade: "C", Label: "Fair"},
			Timestamp: ts,
		}

		row := diagCSVRow(r)
		if len(row) != len(diagCSVHeader) {
			t.Fatalf("column count = %d, want %d", len(row), len(diagCSVHeader))
		}
		// dns_ms is column index 5, dns_cached is column index 6
		if row[5] != "" {
			t.Errorf("dns_ms column = %q, want empty string", row[5])
		}
		if row[6] != "" {
			t.Errorf("dns_cached column = %q, want empty string", row[6])
		}
	})
}

func TestDiagSimpleLine(t *testing.T) {
	t.Run("with DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target: "example.com",
			Method: diag.MethodICMP,
			Hops: []diag.Hop{
				{Number: 1, Latency: 5 * time.Millisecond},
				{Number: 2, Latency: 10 * time.Millisecond},
			},
			DNS: &diag.DNSResult{
				Host:    "example.com",
				IP:      "93.184.216.34",
				Latency: 2 * time.Millisecond,
			},
			Quality: diag.QualityScore{Score: 85, Grade: "B"},
		}

		got := diagSimpleLine(r)
		if !strings.Contains(got, "Score: 85/B") {
			t.Errorf("expected score/grade in output, got %q", got)
		}
		if !strings.Contains(got, "Hops: 2") {
			t.Errorf("expected hop count in output, got %q", got)
		}
	})

	t.Run("nil DNS", func(t *testing.T) {
		r := &diag.DiagResult{
			Target:  "10.0.0.1",
			Method:  diag.MethodUDP,
			Hops:    []diag.Hop{},
			DNS:     nil,
			Quality: diag.QualityScore{Score: 70, Grade: "C"},
		}

		got := diagSimpleLine(r)
		if !strings.Contains(got, "DNS: -") {
			t.Errorf("expected 'DNS: -' for nil DNS, got %q", got)
		}
	})
}

func TestDiagDefaultOutput(t *testing.T) {
	r := &diag.DiagResult{
		Target: "example.com",
		Method: diag.MethodICMP,
		Hops: []diag.Hop{
			{Number: 1, IP: "192.168.1.1", Host: "gateway.local", Latency: 2 * time.Millisecond},
			{Number: 2, Timeout: true},
			{Number: 3, IP: "93.184.216.34", Host: "example.com", Latency: 15 * time.Millisecond},
		},
		DNS: &diag.DNSResult{
			Host:    "example.com",
			IP:      "93.184.216.34",
			Latency: 3 * time.Millisecond,
		},
		Quality:   diag.QualityScore{Score: 80, Grade: "B", Label: "Good"},
		Timestamp: time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
	}

	got := diagDefaultOutput(r)

	checks := []struct {
		label    string
		contains string
	}{
		{"target", "example.com"},
		{"method", "icmp"},
		{"score and grade", "80 / B"},
		{"quality label", "Good"},
		{"host with IP", "gateway.local (192.168.1.1)"},
		{"timeout marker", "*"},
		{"hop count", "Hops (3)"},
	}

	for _, c := range checks {
		if !strings.Contains(got, c.contains) {
			t.Errorf("expected output to contain %s (%q), got:\n%s", c.label, c.contains, got)
		}
	}
}
