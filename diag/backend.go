package diag

import (
	"context"
	"time"
)

type DiagBackend interface {
	Traceroute(ctx context.Context, target string, maxHops int) ([]Hop, string, error)
	ResolveDNS(ctx context.Context, host string) (string, time.Duration, error)
}
