package diag

import (
	"context"
	"testing"
	"time"
)

func TestRealDiagBackendResolveDNS(t *testing.T) {
	b := &RealDiagBackend{}
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

func TestRealDiagBackendResolveDNSCancelled(t *testing.T) {
	b := &RealDiagBackend{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := b.ResolveDNS(ctx, "example.com")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
