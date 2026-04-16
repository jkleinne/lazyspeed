package notify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

type mockSender struct {
	doFn func(req *http.Request) (*http.Response, error)
}

func (m *mockSender) Do(req *http.Request) (*http.Response, error) {
	if m.doFn != nil {
		return m.doFn(req)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestDeliver_Success(t *testing.T) {
	var capturedReq *http.Request
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		},
	}

	endpoints := []model.WebhookEndpoint{{URL: "http://example.com/hook"}}
	payload := NewPayload(&model.SpeedTestResult{DownloadSpeed: 100}, nil, "1.0.0", time.Now())

	errs := deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 1})

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if capturedReq == nil {
		t.Fatal("expected request to be sent")
	}
	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST, got %s", capturedReq.Method)
	}
	if ct := capturedReq.Header.Get("Content-Type"); ct != contentTypeJSON {
		t.Errorf("expected Content-Type %q, got %q", contentTypeJSON, ct)
	}
}

func TestDeliver_CustomHeaders(t *testing.T) {
	var capturedReq *http.Request
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		},
	}

	endpoints := []model.WebhookEndpoint{{
		URL:     "http://example.com/hook",
		Headers: map[string]string{"X-Secret": "abc123", "X-Tenant": "acme"},
	}}
	payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())

	errs := deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 1})

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if capturedReq.Header.Get("X-Secret") != "abc123" {
		t.Errorf("expected X-Secret header 'abc123', got %q", capturedReq.Header.Get("X-Secret"))
	}
	if capturedReq.Header.Get("X-Tenant") != "acme" {
		t.Errorf("expected X-Tenant header 'acme', got %q", capturedReq.Header.Get("X-Tenant"))
	}
}

func TestDeliver_4xxNoRetry(t *testing.T) {
	attempts := 0
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: http.StatusForbidden, Body: http.NoBody}, nil
		},
	}

	endpoints := []model.WebhookEndpoint{{URL: "http://example.com/hook"}}
	payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())

	errs := deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 3})

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt for 4xx, got %d", attempts)
	}
}

func TestDeliver_5xxRetries(t *testing.T) {
	attempts := 0
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: http.NoBody}, nil
		},
	}

	endpoints := []model.WebhookEndpoint{{URL: "http://example.com/hook"}}
	payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())

	// maxRetries=3 means 3 total attempts with exponential backoff between them.
	// Backoff: 1s after attempt 1, 2s after attempt 2. This test takes ~3 seconds.
	errs := deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 3})

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts for 5xx with maxRetries=3, got %d", attempts)
	}
}

func TestDeliver_NetworkErrorRetries(t *testing.T) {
	attempts := 0
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			attempts++
			return nil, errors.New("connection refused")
		},
	}

	endpoints := []model.WebhookEndpoint{{URL: "http://example.com/hook"}}
	payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())

	// maxRetries=2 means 2 total attempts with 1s backoff between them.
	// This test takes ~1 second.
	errs := deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 2})

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts for network error with maxRetries=2, got %d", attempts)
	}
}

func TestDeliver_MultipleEndpointsPartialFailure(t *testing.T) {
	const failURL = "http://fail.example.com/hook"
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == failURL {
				return &http.Response{StatusCode: http.StatusBadGateway, Body: http.NoBody}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		},
	}

	endpoints := []model.WebhookEndpoint{
		{URL: "http://ok1.example.com/hook"},
		{URL: failURL},
		{URL: "http://ok2.example.com/hook"},
	}
	payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())

	errs := deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 1})

	if len(errs) != 1 {
		t.Fatalf("expected 1 error for 1 failing endpoint, got %d", len(errs))
	}
	if errs[0].URL != failURL {
		t.Errorf("expected error URL %q, got %q", failURL, errs[0].URL)
	}
}

func TestDeliver_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	sender := &mockSender{}
	endpoints := []model.WebhookEndpoint{{URL: "http://example.com/hook"}}
	payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())

	errs := deliver(ctx, sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 1})

	if len(errs) != 1 {
		t.Fatalf("expected 1 error for cancelled context, got %d", len(errs))
	}
}

func TestDispatch_NoEndpoints(t *testing.T) {
	cfg := model.WebhookConfig{
		Endpoints:  []model.WebhookEndpoint{},
		Timeout:    10,
		MaxRetries: 1,
	}
	result := &model.SpeedTestResult{DownloadSpeed: 100}

	errs := Dispatch(context.Background(), &mockSender{}, cfg, result, "1.0.0")

	if errs != nil {
		t.Errorf("expected nil for no endpoints, got %v", errs)
	}
}

func TestDispatch_ThresholdsSuppressFiring(t *testing.T) {
	// Thresholds configured but result passes all of them — webhook must not fire.
	cfg := model.WebhookConfig{
		Endpoints: []model.WebhookEndpoint{{URL: "http://example.com/hook"}},
		Thresholds: model.ThresholdConfig{
			MinDownload: ptrFloat64(50),
		},
		Timeout:    10,
		MaxRetries: 1,
	}
	// Result satisfies the threshold (100 >= 50), so no breach.
	result := &model.SpeedTestResult{DownloadSpeed: 100}

	fired := false
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			fired = true
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		},
	}

	errs := Dispatch(context.Background(), sender, cfg, result, "1.0.0")

	if errs != nil {
		t.Errorf("expected nil errors when thresholds suppress firing, got %v", errs)
	}
	if fired {
		t.Error("webhook should not have fired when thresholds pass")
	}
}

func TestDispatch_ThresholdBreachFires(t *testing.T) {
	// Result breaches the threshold — webhook must fire and body must be valid JSON.
	cfg := model.WebhookConfig{
		Endpoints: []model.WebhookEndpoint{{URL: "http://example.com/hook"}},
		Thresholds: model.ThresholdConfig{
			MinDownload: ptrFloat64(50),
		},
		Timeout:    10,
		MaxRetries: 1,
	}
	// Result violates the threshold (10 < 50).
	result := &model.SpeedTestResult{DownloadSpeed: 10}

	var capturedBody []byte
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			var err error
			capturedBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		},
	}

	errs := Dispatch(context.Background(), sender, cfg, result, "1.0.0")

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if capturedBody == nil {
		t.Fatal("expected webhook to fire and send a body")
	}

	var p Payload
	if err := json.Unmarshal(capturedBody, &p); err != nil {
		t.Fatalf("expected valid JSON body, got error: %v", err)
	}
	if p.Event != EventThresholdBreach {
		t.Errorf("expected event %q, got %q", EventThresholdBreach, p.Event)
	}
	if len(p.Breaches) != 1 {
		t.Errorf("expected 1 breach in payload, got %d", len(p.Breaches))
	}
}

func TestDispatch_NoThresholdsAlwaysFires(t *testing.T) {
	// No thresholds configured — webhook must always fire.
	cfg := model.WebhookConfig{
		Endpoints:  []model.WebhookEndpoint{{URL: "http://example.com/hook"}},
		Thresholds: model.ThresholdConfig{
			// All nil — no filtering active.
		},
		Timeout:    10,
		MaxRetries: 1,
	}
	result := &model.SpeedTestResult{DownloadSpeed: 100}

	fired := false
	var capturedBody []byte
	sender := &mockSender{
		doFn: func(req *http.Request) (*http.Response, error) {
			fired = true
			var err error
			capturedBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		},
	}

	errs := Dispatch(context.Background(), sender, cfg, result, "1.0.0")

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if !fired {
		t.Error("webhook should fire when no thresholds configured")
	}

	var p Payload
	if err := json.Unmarshal(capturedBody, &p); err != nil {
		t.Fatalf("expected valid JSON body, got error: %v", err)
	}
	if p.Event != EventTestComplete {
		t.Errorf("expected event %q for no thresholds, got %q", EventTestComplete, p.Event)
	}
}

// trackingReader wraps an io.Reader and calls onEOF exactly once when Read returns io.EOF.
// Used in tests to verify the response body is fully consumed before Close is called.
type trackingReader struct {
	io.Reader
	onEOF  func()
	hitEOF bool
}

func (r *trackingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF && !r.hitEOF {
		r.hitEOF = true
		if r.onEOF != nil {
			r.onEOF()
		}
	}
	return n, err
}

func TestDeliver_DrainsResponseBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"2xx drains body", http.StatusOK},
		{"5xx drains body", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyContent := "response payload"
			drained := false

			sender := &mockSender{
				doFn: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tt.status,
						Body: io.NopCloser(&trackingReader{
							Reader: strings.NewReader(bodyContent),
							onEOF:  func() { drained = true },
						}),
					}, nil
				},
			}

			endpoints := []model.WebhookEndpoint{{URL: "http://example.com/hook"}}
			payload := NewPayload(&model.SpeedTestResult{}, nil, "1.0.0", time.Now())
			deliver(context.Background(), sender, endpoints, payload, deliverOpts{timeout: 5 * time.Second, maxRetries: 1})

			if !drained {
				t.Errorf("response body was not drained for status %d", tt.status)
			}
		})
	}
}
