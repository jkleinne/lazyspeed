package diag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRealBackendResolveDNS(t *testing.T) {
	b := NewRealBackend()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ip, latency, err := b.ResolveDNS(ctx, "localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip == "" {
		t.Error("expected non-empty IP")
	}
	if latency < 0 {
		t.Errorf("expected non-negative latency, got %v", latency)
	}
}

func TestRealBackendResolveDNSCancelled(t *testing.T) {
	b := NewRealBackend()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := b.ResolveDNS(ctx, "example.com")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestTraceLoop(t *testing.T) {
	t.Run("reaches destination", func(t *testing.T) {
		destIP := "10.0.0.3"
		hops := traceLoop(context.Background(), destIP, 30, func(ttl int) Hop {
			return Hop{Number: ttl, IP: fmt.Sprintf("10.0.0.%d", ttl)}
		})
		if len(hops) != 3 {
			t.Fatalf("got %d hops, want 3 (should stop at destIP)", len(hops))
		}
		if hops[2].IP != destIP {
			t.Errorf("last hop IP = %q, want %q", hops[2].IP, destIP)
		}
	})

	t.Run("respects maxHops", func(t *testing.T) {
		hops := traceLoop(context.Background(), "unreachable", 5, func(ttl int) Hop {
			return Hop{Number: ttl, IP: fmt.Sprintf("10.0.0.%d", ttl)}
		})
		if len(hops) != 5 {
			t.Fatalf("got %d hops, want 5 (maxHops limit)", len(hops))
		}
	})

	t.Run("context cancellation stops loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		hops := traceLoop(ctx, "10.0.0.5", 30, func(ttl int) Hop {
			return Hop{Number: ttl, IP: fmt.Sprintf("10.0.0.%d", ttl)}
		})
		if len(hops) != 0 {
			t.Fatalf("got %d hops, want 0 (context already cancelled)", len(hops))
		}
	})

	t.Run("cancel mid-loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		hops := traceLoop(ctx, "unreachable", 30, func(ttl int) Hop {
			if ttl == 3 {
				cancel()
			}
			return Hop{Number: ttl, IP: fmt.Sprintf("10.0.0.%d", ttl)}
		})
		// Should have hops 1-3 (cancel happens at ttl=3, but that hop completes;
		// ctx.Err() is checked at the start of the next iteration)
		if len(hops) != 3 {
			t.Fatalf("got %d hops, want 3", len(hops))
		}
	})
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"bare os.ErrPermission", os.ErrPermission, true},
		{"wrapped in PathError", &os.PathError{Op: "open", Err: os.ErrPermission}, true},
		{"wrapped in SyscallError", &os.SyscallError{Syscall: "socket", Err: os.ErrPermission}, true},
		{"unrelated error", errors.New("network timeout"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermissionError(tt.err); got != tt.want {
				t.Errorf("isPermissionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
