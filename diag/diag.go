package diag

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const (
	MethodICMP = "icmp"
	MethodUDP  = "udp"

	// dnsCacheThresholdDivisor is the factor by which warm DNS latency must be lower
	// than cold latency to be considered a cached result.
	dnsCacheThresholdDivisor = 2
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
		Latency: DurationMs(h.Latency),
	})
}

func (h *Hop) UnmarshalJSON(data []byte) error {
	type Alias Hop
	aux := &struct {
		*Alias
		Latency float64 `json:"latency"`
	}{Alias: (*Alias)(h)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	h.Latency = time.Duration(aux.Latency * float64(time.Millisecond))
	return nil
}

type DNSResult struct {
	Host    string        `json:"host"`
	IP      string        `json:"resolved_ip"`
	Latency time.Duration `json:"latency"`
	Cached  bool          `json:"cached"`
}

func (d DNSResult) MarshalJSON() ([]byte, error) {
	type Alias DNSResult
	return json.Marshal(&struct {
		Alias
		Latency float64 `json:"latency"`
	}{
		Alias:   (Alias)(d),
		Latency: DurationMs(d.Latency),
	})
}

func (d *DNSResult) UnmarshalJSON(data []byte) error {
	type Alias DNSResult
	aux := &struct {
		*Alias
		Latency float64 `json:"latency"`
	}{Alias: (*Alias)(d)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	d.Latency = time.Duration(aux.Latency * float64(time.Millisecond))
	return nil
}

type QualityScore struct {
	Score int    `json:"score"`
	Grade string `json:"grade"`
	Label string `json:"label"`
}

type DiagResult struct {
	Target    string       `json:"target"`
	Method    string       `json:"method"`
	Hops      []Hop        `json:"hops"`
	DNS       *DNSResult   `json:"dns"`
	Quality   QualityScore `json:"quality"`
	Timestamp time.Time    `json:"timestamp"`
}

type DiagConfig struct {
	MaxHops    int
	Timeout    int
	MaxEntries int
	Path       string
}

func DefaultDiagConfig() *DiagConfig {
	return &DiagConfig{
		MaxHops:    30,
		Timeout:    60,
		MaxEntries: 20,
	}
}

// DurationMs converts a time.Duration to fractional milliseconds.
func DurationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func Run(ctx context.Context, backend DiagBackend, target string, cfg *DiagConfig) (*DiagResult, error) {
	if cfg == nil {
		cfg = DefaultDiagConfig()
	}

	result := &DiagResult{
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
			result.DNS = &DNSResult{
				Host:    target,
				Latency: time.Duration(dnsTerrible) * time.Millisecond,
				Cached:  false,
			}
		} else {
			_, warmLatency, _ := backend.ResolveDNS(ctx, target)
			cached := warmLatency < coldLatency/dnsCacheThresholdDivisor
			result.DNS = &DNSResult{
				Host:    target,
				IP:      coldIP,
				Latency: coldLatency,
				Cached:  cached,
			}
		}
	}

	// Phase 2: Traceroute
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	hops, method, err := backend.Traceroute(ctx, target, cfg.MaxHops)
	if err != nil {
		return nil, fmt.Errorf("failed to run traceroute: %v", err)
	}
	result.Hops = hops
	result.Method = method

	// Phase 3: Quality score
	result.Quality = ComputeScore(result)

	return result, nil
}
