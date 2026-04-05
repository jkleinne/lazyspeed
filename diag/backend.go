package diag

import (
	"context"
	"time"
)

// Backend abstracts network diagnostics operations for testability.
type Backend interface {
	// Traceroute runs a traceroute to the target. If resolvedIP is non-empty, it
	// is used directly, avoiding a redundant DNS lookup when the caller already
	// resolved the target.
	Traceroute(ctx context.Context, target string, maxHops int, resolvedIP string) ([]Hop, string, error)

	// ResolveDNS resolves a hostname to an IP address and measures DNS lookup latency.
	ResolveDNS(ctx context.Context, host string) (string, time.Duration, error)
}
