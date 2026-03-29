package diag

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

const testExampleIP = "93.184.216.34"

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		backend    *mockDiagBackend
		wantMethod string
		wantDNSNil bool
		wantErr    bool
	}{
		{
			name:   "happy path with hostname",
			target: "example.com",
			backend: &mockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return []Hop{
						{Number: 1, IP: "192.168.1.1", Host: "router.local", Latency: 1 * time.Millisecond},
						{Number: 2, IP: testExampleIP, Host: "example.com", Latency: 20 * time.Millisecond},
					}, MethodICMP, nil
				},
				ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
					return testExampleIP, 15 * time.Millisecond, nil
				},
			},
			wantMethod: MethodICMP,
			wantDNSNil: false,
		},
		{
			name:   "IP target skips DNS",
			target: "8.8.8.8",
			backend: &mockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return []Hop{
						{Number: 1, IP: "192.168.1.1", Host: "router.local", Latency: 1 * time.Millisecond},
					}, MethodUDP, nil
				},
			},
			wantMethod: MethodUDP,
			wantDNSNil: true,
		},
		{
			name:   "all hops timeout",
			target: "example.com",
			backend: &mockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return []Hop{
						{Number: 1, Timeout: true},
						{Number: 2, Timeout: true},
						{Number: 3, Timeout: true},
					}, MethodUDP, nil
				},
				ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
					return testExampleIP, 10 * time.Millisecond, nil
				},
			},
			wantMethod: MethodUDP,
			wantDNSNil: false,
		},
		{
			name:   "traceroute error",
			target: "example.com",
			backend: &mockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return nil, "", fmt.Errorf("network unreachable")
				},
				ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
					return testExampleIP, 10 * time.Millisecond, nil
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultDiagConfig()
			result, err := Run(context.Background(), tt.backend, tt.target, cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Method != tt.wantMethod {
				t.Errorf("method = %q, want %q", result.Method, tt.wantMethod)
			}
			if tt.wantDNSNil && result.DNS != nil {
				t.Error("expected DNS to be nil for IP target")
			}
			if !tt.wantDNSNil && result.DNS == nil {
				t.Error("expected DNS to be non-nil for hostname target")
			}
			if result.Quality.Grade == "" {
				t.Error("quality grade should not be empty")
			}
		})
	}
}

func TestRunContextCancellationImmediate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	backend := &mockDiagBackend{
		ResolveDNSFn: func(ctx context.Context, _ string) (string, time.Duration, error) {
			return "", 0, ctx.Err()
		},
	}

	cfg := DefaultDiagConfig()
	_, err := Run(ctx, backend, "example.com", cfg)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestRunContextCancellationPartialResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	backend := &mockDiagBackend{
		ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
			return testExampleIP, 10 * time.Millisecond, nil
		},
		TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
			cancel()
			return []Hop{
				{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 1 * time.Millisecond},
			}, MethodUDP, nil
		},
	}

	cfg := DefaultDiagConfig()
	result, err := Run(ctx, backend, "example.com", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DNS == nil {
		t.Error("expected DNS result even with cancellation after traceroute")
	}
	if len(result.Hops) != 1 {
		t.Errorf("expected 1 hop from partial result, got %d", len(result.Hops))
	}
}

func TestDNSResultMarshalJSON(t *testing.T) {
	dns := DNSResult{
		Host:    "example.com",
		IP:      testExampleIP,
		Latency: 12500 * time.Microsecond,
		Cached:  false,
	}

	data, err := json.Marshal(dns)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal into map failed: %v", err)
	}

	latency, ok := raw["latency"].(float64)
	if !ok {
		t.Fatal("latency field missing or not a float64")
	}
	if latency != 12.5 {
		t.Errorf("latency = %v, want 12.5", latency)
	}
}

func TestDiagResultJSONRoundTrip(t *testing.T) {
	original := DiagResult{
		Target: "example.com",
		Method: MethodICMP,
		Hops: []Hop{
			{Number: 1, IP: "192.168.1.1", Host: "router.local", Latency: 5 * time.Millisecond},
			{Number: 2, Timeout: true},
		},
		DNS: &DNSResult{
			Host:    "example.com",
			IP:      testExampleIP,
			Latency: 15 * time.Millisecond,
			Cached:  false,
		},
		Quality:   QualityScore{Score: 72, Grade: "B", Label: "Good for most activities"},
		Timestamp: time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded DiagResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Target != original.Target {
		t.Errorf("target = %q, want %q", decoded.Target, original.Target)
	}
	if len(decoded.Hops) != len(original.Hops) {
		t.Fatalf("hop count = %d, want %d", len(decoded.Hops), len(original.Hops))
	}
	if decoded.Hops[0].Latency != original.Hops[0].Latency {
		t.Errorf("hop[0] latency = %v, want %v", decoded.Hops[0].Latency, original.Hops[0].Latency)
	}
	if !decoded.Hops[1].Timeout {
		t.Error("hop[1] should be a timeout")
	}
	if decoded.DNS == nil {
		t.Fatal("DNS should not be nil")
	}
	if decoded.DNS.Latency != original.DNS.Latency {
		t.Errorf("DNS latency = %v, want %v", decoded.DNS.Latency, original.DNS.Latency)
	}
	if decoded.Quality.Score != original.Quality.Score {
		t.Errorf("quality score = %d, want %d", decoded.Quality.Score, original.Quality.Score)
	}
}

func TestDiagResultUnmarshalPartialJSON(t *testing.T) {
	partial := `{"target":"example.com","method":"icmp","hops":[],"dns":null,"timestamp":"2026-03-26T12:00:00Z"}`

	var result DiagResult
	if err := json.Unmarshal([]byte(partial), &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if result.Target != "example.com" {
		t.Errorf("target = %q, want %q", result.Target, "example.com")
	}
	if result.Quality.Score != 0 {
		t.Errorf("score = %d, want 0 for missing quality", result.Quality.Score)
	}
}

func TestRunDNSFailureContinuesTraceroute(t *testing.T) {
	backend := &mockDiagBackend{
		TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
			return []Hop{
				{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 2 * time.Millisecond},
			}, MethodICMP, nil
		},
		ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
			return "", 0, fmt.Errorf("dns resolution failed")
		},
	}

	result, err := Run(context.Background(), backend, "example.com", DefaultDiagConfig())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.DNS == nil {
		t.Fatal("DNS should be non-nil even on resolution failure")
	}
	if len(result.Hops) != 1 {
		t.Errorf("hop count = %d, want 1", len(result.Hops))
	}
}

func TestComputeScoreZeroHops(t *testing.T) {
	result := &DiagResult{
		Target: "example.com",
		Hops:   []Hop{},
		DNS: &DNSResult{
			Host:    "example.com",
			IP:      testExampleIP,
			Latency: 10 * time.Millisecond,
		},
	}

	score := ComputeScore(result)
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("score = %d, want 0-100 range", score.Score)
	}
	if score.Grade == "" {
		t.Error("grade should not be empty")
	}
}

func TestComputeScoreAllHopsTimeout(t *testing.T) {
	var hops []Hop
	for i := 1; i <= 10; i++ {
		hops = append(hops, Hop{Number: i, Timeout: true})
	}

	result := &DiagResult{
		Target: "example.com",
		Hops:   hops,
		DNS: &DNSResult{
			Host:    "example.com",
			IP:      testExampleIP,
			Latency: 500 * time.Millisecond,
		},
	}

	score := ComputeScore(result)
	if score.Grade != "F" {
		t.Errorf("grade = %q, want %q", score.Grade, "F")
	}
}

func TestConfigFromModel(t *testing.T) {
	tests := []struct {
		name     string
		input    model.DiagnosticsConfig
		wantHops int
		wantTime int
		wantMax  int
		wantPath string
	}{
		{
			name:     "all zeros returns defaults",
			input:    model.DiagnosticsConfig{},
			wantHops: 30,
			wantTime: 60,
			wantMax:  20,
			wantPath: "",
		},
		{
			name:     "non-zero values override defaults",
			input:    model.DiagnosticsConfig{MaxHops: 15, Timeout: 45, MaxEntries: 10, Path: "/custom/path.json"},
			wantHops: 15,
			wantTime: 45,
			wantMax:  10,
			wantPath: "/custom/path.json",
		},
		{
			name:     "partial override",
			input:    model.DiagnosticsConfig{MaxHops: 20},
			wantHops: 20,
			wantTime: 60,
			wantMax:  20,
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ConfigFromModel(tt.input)
			if cfg.MaxHops != tt.wantHops {
				t.Errorf("MaxHops = %d, want %d", cfg.MaxHops, tt.wantHops)
			}
			if cfg.Timeout != tt.wantTime {
				t.Errorf("Timeout = %d, want %d", cfg.Timeout, tt.wantTime)
			}
			if cfg.MaxEntries != tt.wantMax {
				t.Errorf("MaxEntries = %d, want %d", cfg.MaxEntries, tt.wantMax)
			}
			if cfg.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", cfg.Path, tt.wantPath)
			}
		})
	}
}

func TestRunWarmDNSFailureSetsCachedFalse(t *testing.T) {
	callCount := 0
	backend := &mockDiagBackend{
		TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
			return []Hop{
				{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 1 * time.Millisecond},
			}, MethodICMP, nil
		},
		ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
			callCount++
			if callCount == 1 {
				return testExampleIP, 15 * time.Millisecond, nil
			}
			return "", 0, fmt.Errorf("warm DNS lookup failed")
		},
	}

	result, err := Run(context.Background(), backend, "example.com", DefaultDiagConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DNS == nil {
		t.Fatal("expected DNS result")
	}
	if result.DNS.Cached {
		t.Error("expected cached=false when warm DNS resolution fails, got true")
	}
}

func TestIsIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"8.8.8.8", true},
		{"::1", true},
		{"example.com", false},
		{"not-an-ip", false},
	}
	for _, tt := range tests {
		got := net.ParseIP(tt.input) != nil
		if got != tt.want {
			t.Errorf("isIP(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
