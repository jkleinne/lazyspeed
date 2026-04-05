package diag

import (
	"context"
	"time"
)

type mockBackend struct {
	TracerouteFn func(ctx context.Context, target string, maxHops int, resolvedIP string) ([]Hop, string, error)
	ResolveDNSFn func(ctx context.Context, host string) (string, time.Duration, error)
}

func (m *mockBackend) Traceroute(ctx context.Context, target string, maxHops int, resolvedIP string) ([]Hop, string, error) {
	if m.TracerouteFn != nil {
		return m.TracerouteFn(ctx, target, maxHops, resolvedIP)
	}
	return []Hop{}, MethodUDP, nil
}

func (m *mockBackend) ResolveDNS(ctx context.Context, host string) (string, time.Duration, error) {
	if m.ResolveDNSFn != nil {
		return m.ResolveDNSFn(ctx, host)
	}
	return "127.0.0.1", time.Millisecond, nil
}
