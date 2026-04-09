package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

const (
	contentTypeJSON = "application/json"
	initialBackoff  = 1 * time.Second
	backoffFactor   = 2
	maxBackoff      = 8 * time.Second
)

// Sender abstracts HTTP POST for testability. *http.Client satisfies this interface.
type Sender interface {
	Do(req *http.Request) (*http.Response, error)
}

// DeliveryError records a webhook delivery failure for a specific endpoint.
type DeliveryError struct {
	URL string
	Err error
}

func (e DeliveryError) Error() string {
	return fmt.Sprintf("webhook %s: %v", e.URL, e.Err)
}

// Deliver sends the payload to all configured endpoints sequentially.
// Returns a slice of DeliveryError for every endpoint that failed; nil means all succeeded.
// A cancelled context aborts at the next delivery attempt.
func Deliver(ctx context.Context, sender Sender, endpoints []model.WebhookEndpoint, payload Payload, timeout time.Duration, maxRetries int) []DeliveryError {
	body, err := json.Marshal(payload)
	if err != nil {
		return []DeliveryError{{URL: "(marshal)", Err: fmt.Errorf("failed to marshal payload: %v", err)}}
	}

	var errs []DeliveryError
	for _, ep := range endpoints {
		if err := deliverOne(ctx, sender, ep, body, timeout, maxRetries); err != nil {
			errs = append(errs, DeliveryError{URL: ep.URL, Err: err})
		}
	}
	return errs
}

// deliverOne sends the pre-serialized body to a single endpoint with retry/backoff.
// Returns nil on success, a descriptive error on permanent failure.
func deliverOne(ctx context.Context, sender Sender, ep model.WebhookEndpoint, body []byte, timeout time.Duration, maxRetries int) error {
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
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, ep.URL, bytes.NewReader(body))
		if err != nil {
			reqCancel()
			return fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", contentTypeJSON)
		for k, v := range ep.Headers {
			req.Header.Set(k, v)
		}

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
		// 4xx errors are client-side faults that retrying cannot fix.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("client error: status %d", resp.StatusCode)
		}
		// 5xx and other non-2xx codes are retryable.
		lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	return lastErr
}

// Dispatch is the top-level entry point for webhook delivery.
// Evaluates thresholds, builds the payload, and delivers to all configured endpoints.
// Returns nil if no endpoints are configured or if thresholds are active and all pass.
func Dispatch(ctx context.Context, sender Sender, cfg model.WebhookConfig, result *model.SpeedTestResult, version string) []DeliveryError {
	if len(cfg.Endpoints) == 0 {
		return nil
	}

	breaches := EvaluateThresholds(result, cfg.Thresholds)
	// A non-nil empty slice means thresholds are configured and all passed — suppress firing.
	if breaches != nil && len(breaches) == 0 {
		return nil
	}

	payload := NewPayload(result, breaches, version, time.Now())
	timeout := time.Duration(cfg.Timeout) * time.Second
	return Deliver(ctx, sender, cfg.Endpoints, payload, timeout, cfg.MaxRetries)
}
