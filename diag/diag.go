package diag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jkleinne/lazyspeed/internal/timeutil"
)

const (
	MethodICMP = "icmp"
	MethodUDP  = "udp"

	// dnsCacheThresholdDivisor is the factor by which warm DNS latency must be lower
	// than cold latency to be considered a cached result.
	dnsCacheThresholdDivisor = 2

	defaultMaxHops    = 30
	defaultTimeoutSec = 60
	defaultMaxEntries = 20
)

type Hop struct {
	Number  int           `json:"number"`
	IP      string        `json:"ip"`
	Host    string        `json:"host"`
	Latency time.Duration `json:"latency"`
	Timeout bool          `json:"timeout"`
}

func (h Hop) MarshalJSON() ([]byte, error) {
	type Alias Hop
	return json.Marshal(&struct {
		Alias
		Latency float64 `json:"latency"`
	}{
		Alias:   (Alias)(h),
		Latency: marshalDurationAsMs(h.Latency),
	})
}

func (h *Hop) UnmarshalJSON(data []byte) error {
	type Alias Hop
	aux := &struct {
		*Alias
		Latency float64 `json:"latency"`
	}{Alias: (*Alias)(h)}
	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal hop: %v", err) //nolint:errorlint // project convention: %v not %w
	}
	h.Latency = unmarshalMsAsDuration(aux.Latency)
	return nil
}

type DNSResult struct {
	Host    string        `json:"host"`
	IP      string        `json:"resolved_ip"`
	Latency time.Duration `json:"latency"`
	Cached  bool          `json:"cached"`
	Error   string        `json:"error,omitempty"`
}

func (d DNSResult) MarshalJSON() ([]byte, error) {
	type Alias DNSResult
	return json.Marshal(&struct {
		Alias
		Latency float64 `json:"latency"`
	}{
		Alias:   (Alias)(d),
		Latency: marshalDurationAsMs(d.Latency),
	})
}

func (d *DNSResult) UnmarshalJSON(data []byte) error {
	type Alias DNSResult
	aux := &struct {
		*Alias
		Latency float64 `json:"latency"`
	}{Alias: (*Alias)(d)}
	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal DNS result: %v", err) //nolint:errorlint // project convention: %v not %w
	}
	d.Latency = unmarshalMsAsDuration(aux.Latency)
	return nil
}

type QualityScore struct {
	Score int    `json:"score"`
	Grade grade  `json:"grade"`
	Label string `json:"label"`
}

type Result struct {
	Target    string       `json:"target"`
	Method    string       `json:"method"`
	Hops      []Hop        `json:"hops"`
	DNS       *DNSResult   `json:"dns"`
	Quality   QualityScore `json:"quality"`
	Timestamp time.Time    `json:"timestamp"`
}

type Config struct {
	MaxHops    int
	Timeout    int
	MaxEntries int
	Path       string
}

// DefaultConfig returns sensible defaults for diagnostics configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxHops:    defaultMaxHops,
		Timeout:    defaultTimeoutSec,
		MaxEntries: defaultMaxEntries,
	}
}

// NewConfig creates a Config by overlaying non-zero overrides onto defaults.
func NewConfig(overrides Config) *Config {
	cfg := DefaultConfig()
	if overrides.MaxHops > 0 {
		cfg.MaxHops = overrides.MaxHops
	}
	if overrides.Timeout > 0 {
		cfg.Timeout = overrides.Timeout
	}
	if overrides.MaxEntries > 0 {
		cfg.MaxEntries = overrides.MaxEntries
	}
	if overrides.Path != "" {
		cfg.Path = overrides.Path
	}
	return cfg
}

func Run(ctx context.Context, backend Backend, target string, cfg *Config) (*Result, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("target must not be empty")
	}

	result := &Result{
		Target:    target,
		Timestamp: time.Now(),
	}

	isIPTarget := net.ParseIP(target) != nil

	// Phase 1: DNS resolution (skip if target is an IP)
	if !isIPTarget {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		coldIP, coldLatency, err := backend.ResolveDNS(ctx, target)
		if err != nil {
			// DNS failure is non-fatal — we record the error in the result and proceed
			// to traceroute using the raw target string. The user sees DNS status in the
			// output and the quality score adjusts by redistributing DNS weight.
			result.DNS = &DNSResult{
				Host:  target,
				Error: fmt.Sprintf("dns resolution failed: %v", err),
			}
		} else {
			_, warmLatency, warmErr := backend.ResolveDNS(ctx, target)
			cached := warmErr == nil && warmLatency < coldLatency/dnsCacheThresholdDivisor
			result.DNS = &DNSResult{
				Host:    target,
				IP:      coldIP,
				Latency: coldLatency,
				Cached:  cached,
			}
		}
	}

	// Phase 2: Traceroute — pass the already-resolved IP to avoid duplicate resolution
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	resolvedIP := ""
	if result.DNS != nil && result.DNS.IP != "" {
		resolvedIP = result.DNS.IP
	}
	hops, method, err := backend.Traceroute(ctx, target, cfg.MaxHops, resolvedIP)
	if err != nil {
		return nil, fmt.Errorf("failed to run traceroute: %v", err) //nolint:errorlint // project convention: %v not %w
	}
	result.Hops = hops
	result.Method = method

	// Phase 3: Quality score
	result.Quality = ComputeScore(result)

	return result, nil
}

// marshalDurationAsMs converts a time.Duration to fractional milliseconds for JSON output.
func marshalDurationAsMs(d time.Duration) float64 {
	return timeutil.DurationMs(d)
}

// unmarshalMsAsDuration converts a float64 millisecond value to a time.Duration.
func unmarshalMsAsDuration(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}
