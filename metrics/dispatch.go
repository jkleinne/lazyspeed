package metrics

import (
	"context"
	"os"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

// Dispatch is the top-level entry point for InfluxDB metrics export.
// It encodes the result once, then writes sequentially to each configured
// endpoint. Returns nil if no endpoints are configured; otherwise returns
// a slice of WriteError for every endpoint that failed. Per-endpoint
// failures do not abort delivery to subsequent endpoints.
//
// A cancelled context aborts at the next delivery attempt.
func Dispatch(ctx context.Context, sender Sender, cfg model.MetricsConfig, result *model.SpeedTestResult) []WriteError {
	if len(cfg.Endpoints) == 0 {
		return nil
	}

	host := resolveHost(cfg)
	body := EncodePoint(result, host)
	timeout := time.Duration(cfg.Timeout) * time.Second

	var errs []WriteError
	for _, ep := range cfg.Endpoints {
		if err := writeOne(ctx, sender, ep, body, timeout, cfg.MaxRetries); err != nil {
			errs = append(errs, WriteError{URL: ep.URL, Err: err})
		}
	}
	return errs
}

// resolveHost returns the host tag value based on the config. OmitHostTag
// or an os.Hostname() failure returns the empty string, which the encoder
// treats as "omit the host tag entirely". Users who want a literal
// fallback label (e.g., "unknown") can set HostTag explicitly.
func resolveHost(cfg model.MetricsConfig) string {
	if cfg.OmitHostTag {
		return ""
	}
	if cfg.HostTag != "" {
		return cfg.HostTag
	}
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return host
}
