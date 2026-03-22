package diag

import (
	"context"
	"time"
)

type DiagBackend interface {
	Traceroute(ctx context.Context, target string, maxHops int) ([]Hop, string, error)
	ResolveDNS(ctx context.Context, host string) (string, time.Duration, error)
}

type MockDiagBackend struct {
	TracerouteFn func(ctx context.Context, target string, maxHops int) ([]Hop, string, error)
	ResolveDNSFn func(ctx context.Context, host string) (string, time.Duration, error)
}

func (m *MockDiagBackend) Traceroute(ctx context.Context, target string, maxHops int) ([]Hop, string, error) {
	if m.TracerouteFn != nil {
		return m.TracerouteFn(ctx, target, maxHops)
	}
	return []Hop{}, MethodUDP, nil
}

func (m *MockDiagBackend) ResolveDNS(ctx context.Context, host string) (string, time.Duration, error) {
	if m.ResolveDNSFn != nil {
		return m.ResolveDNSFn(ctx, host)
	}
	return "127.0.0.1", time.Millisecond, nil
}
