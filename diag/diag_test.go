package diag

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		backend    *MockDiagBackend
		wantMethod string
		wantDNSNil bool
		wantErr    bool
	}{
		{
			name:   "happy path with hostname",
			target: "example.com",
			backend: &MockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return []Hop{
						{Number: 1, IP: "192.168.1.1", Host: "router.local", Latency: 1 * time.Millisecond},
						{Number: 2, IP: "93.184.216.34", Host: "example.com", Latency: 20 * time.Millisecond},
					}, "icmp", nil
				},
				ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
					return "93.184.216.34", 15 * time.Millisecond, nil
				},
			},
			wantMethod: "icmp",
			wantDNSNil: false,
		},
		{
			name:   "IP target skips DNS",
			target: "8.8.8.8",
			backend: &MockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return []Hop{
						{Number: 1, IP: "192.168.1.1", Host: "router.local", Latency: 1 * time.Millisecond},
					}, "udp", nil
				},
			},
			wantMethod: "udp",
			wantDNSNil: true,
		},
		{
			name:   "all hops timeout",
			target: "example.com",
			backend: &MockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return []Hop{
						{Number: 1, Timeout: true},
						{Number: 2, Timeout: true},
						{Number: 3, Timeout: true},
					}, "udp", nil
				},
				ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
					return "93.184.216.34", 10 * time.Millisecond, nil
				},
			},
			wantMethod: "udp",
			wantDNSNil: false,
		},
		{
			name:   "traceroute error",
			target: "example.com",
			backend: &MockDiagBackend{
				TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
					return nil, "", fmt.Errorf("network unreachable")
				},
				ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
					return "93.184.216.34", 10 * time.Millisecond, nil
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

	backend := &MockDiagBackend{
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

	backend := &MockDiagBackend{
		ResolveDNSFn: func(_ context.Context, _ string) (string, time.Duration, error) {
			return "93.184.216.34", 10 * time.Millisecond, nil
		},
		TracerouteFn: func(_ context.Context, _ string, _ int) ([]Hop, string, error) {
			cancel()
			return []Hop{
				{Number: 1, IP: "10.0.0.1", Host: "gw", Latency: 1 * time.Millisecond},
			}, "udp", nil
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
