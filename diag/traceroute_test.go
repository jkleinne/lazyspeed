package diag

import (
	"context"
	"errors"
	"os"
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
