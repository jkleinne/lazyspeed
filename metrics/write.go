package metrics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

// Retry/backoff constants for writeOne. These match the shapes defined in
// notify/deliver.go:15-20 for webhook delivery. They are kept package-local
// rather than shared because the two packages have independent lifecycles;
// a future reader looking at either set should find both at once.
const (
	initialBackoff  = 1 * time.Second
	backoffFactor   = 2
	maxBackoff      = 8 * time.Second
	contentTypeLine = "text/plain; charset=utf-8"
	precisionQuery  = "ns"
	suffixV1Write   = "/write"
	suffixV2Write   = "/api/v2/write"
)

// Sender abstracts HTTP POST for testability. *http.Client satisfies this
// interface. Mirrors notify.Sender for the same reason: keeps tests free
// of real network I/O without taking a dependency on notify.
type Sender interface {
	Do(req *http.Request) (*http.Response, error)
}

// WriteError records a write failure for a specific endpoint.
type WriteError struct {
	URL string
	Err error
}

func (e WriteError) Error() string {
	return fmt.Sprintf("metrics %s: %v", e.URL, e.Err)
}

// writeOne sends the pre-encoded body to a single InfluxDB endpoint with
// retry/backoff. Returns nil on success, a descriptive error on permanent
// failure. Network errors and 5xx are retried up to maxRetries times with
// exponential backoff; 4xx fails permanently on the first attempt.
func writeOne(ctx context.Context, sender Sender, ep model.MetricsEndpoint, body []byte, timeout time.Duration, maxRetries int) error {
	writeURL, err := buildWriteURL(ep)
	if err != nil {
		return fmt.Errorf("failed to build write URL: %v", err)
	}

	var lastErr error
	backoff := initialBackoff

	for attempt := range maxRetries {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("cancelled: %v", err)
		}

		if attempt > 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("cancelled during backoff: %v", ctx.Err())
			case <-time.After(backoff):
				backoff *= backoffFactor
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}

		reqCtx, reqCancel := context.WithTimeout(ctx, timeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, writeURL, bytes.NewReader(body))
		if err != nil {
			reqCancel()
			return fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", contentTypeLine)
		applyAuth(req, ep)

		resp, err := sender.Do(req)
		if err != nil {
			reqCancel()
			lastErr = fmt.Errorf("request failed: %v", err)
			continue
		}

		// Drain body before closing to allow HTTP connection reuse.
		// Drain errors are safe to ignore: failure just means the
		// connection won't be reused, which is a performance detail.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		reqCancel()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("client error: status %d", resp.StatusCode)
		}
		lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	return lastErr
}

// buildWriteURL constructs the full write endpoint URL for an InfluxDB
// endpoint. For v2, the suffix is /api/v2/write with org, bucket, and
// precision query parameters. For v1, the suffix is /write with the db
// query parameter. If the base URL already has a path prefix (e.g., a
// proxied deployment), the suffix is appended to it.
func buildWriteURL(ep model.MetricsEndpoint) (string, error) {
	base, err := url.Parse(ep.URL)
	if err != nil {
		return "", fmt.Errorf("parsing base URL %q: %v", ep.URL, err)
	}
	q := url.Values{}
	var suffix string
	switch {
	case ep.V2 != nil:
		suffix = suffixV2Write
		q.Set("org", ep.V2.Org)
		q.Set("bucket", ep.V2.Bucket)
		q.Set("precision", precisionQuery)
	case ep.V1 != nil:
		suffix = suffixV1Write
		q.Set("db", ep.V1.Database)
	default:
		return "", fmt.Errorf("endpoint %q has neither v1 nor v2 auth set", ep.URL)
	}
	base.Path = strings.TrimRight(base.Path, "/") + suffix
	base.RawQuery = q.Encode()
	return base.String(), nil
}

// applyAuth sets the appropriate auth header on the request based on
// which auth block is populated. v1 with no username sends no auth
// header at all (passwordless deployments).
func applyAuth(req *http.Request, ep model.MetricsEndpoint) {
	switch {
	case ep.V2 != nil:
		req.Header.Set("Authorization", "Token "+ep.V2.Token)
	case ep.V1 != nil:
		if ep.V1.Username != "" {
			req.SetBasicAuth(ep.V1.Username, ep.V1.Password)
		}
	}
}
