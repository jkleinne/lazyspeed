package metrics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

func TestDispatch_NoEndpoints(t *testing.T) {
	errs := Dispatch(context.Background(), &mockSender{}, model.MetricsConfig{}, &model.SpeedTestResult{})
	if errs != nil {
		t.Errorf("expected nil for no endpoints, got %v", errs)
	}
}

func TestDispatch_SingleSuccess(t *testing.T) {
	var capturedBody []byte
	sender := &mockSender{DoFn: func(req *http.Request) (*http.Response, error) {
		capturedBody, _ = io.ReadAll(req.Body)
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}, nil
	}}
	cfg := model.MetricsConfig{
		Endpoints: []model.MetricsEndpoint{
			{URL: "https://example.com", V2: &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"}},
		},
		Timeout:    10,
		MaxRetries: 1,
		HostTag:    "h1",
	}
	result := &model.SpeedTestResult{
		DownloadSpeed: 100,
		ServerName:    "srv",
		Timestamp:     time.Unix(1712761200, 0),
	}
	errs := Dispatch(context.Background(), sender, cfg, result)
	if errs != nil {
		t.Errorf("expected no errors, got %v", errs)
	}
	if !strings.Contains(string(capturedBody), "host=h1") {
		t.Errorf("expected host tag in body, got %q", capturedBody)
	}
	if !strings.Contains(string(capturedBody), "download_mbps=100") {
		t.Errorf("expected download field, got %q", capturedBody)
	}
}

func TestDispatch_OneSucceedsOneFails(t *testing.T) {
	call := 0
	sender := &mockSender{DoFn: func(req *http.Request) (*http.Response, error) {
		call++
		if call == 1 {
			return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return nil, errors.New("boom")
	}}
	cfg := model.MetricsConfig{
		Endpoints: []model.MetricsEndpoint{
			{URL: "https://ok.example.com", V2: &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"}},
			{URL: "https://bad.example.com", V2: &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"}},
		},
		Timeout:    1,
		MaxRetries: 1,
	}
	errs := Dispatch(context.Background(), sender, cfg, &model.SpeedTestResult{Timestamp: time.Unix(1, 0)})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].URL != "https://bad.example.com" {
		t.Errorf("wrong URL on error: %q", errs[0].URL)
	}
}

func TestDispatch_OmitHostTag(t *testing.T) {
	var body []byte
	sender := &mockSender{DoFn: func(req *http.Request) (*http.Response, error) {
		body, _ = io.ReadAll(req.Body)
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}, nil
	}}
	cfg := model.MetricsConfig{
		Endpoints: []model.MetricsEndpoint{
			{URL: "https://example.com", V2: &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"}},
		},
		Timeout:     1,
		MaxRetries:  1,
		OmitHostTag: true,
	}
	Dispatch(context.Background(), sender, cfg, &model.SpeedTestResult{Timestamp: time.Unix(1, 0)})
	if strings.Contains(string(body), "host=") {
		t.Errorf("expected no host tag, got %q", body)
	}
}

func TestResolveHost(t *testing.T) {
	if h := resolveHost(model.MetricsConfig{OmitHostTag: true, HostTag: "override"}); h != "" {
		t.Errorf("OmitHostTag should win, got %q", h)
	}
	if h := resolveHost(model.MetricsConfig{HostTag: "custom"}); h != "custom" {
		t.Errorf("custom override: got %q, want %q", h, "custom")
	}
	// Default path calls os.Hostname(); just assert it doesn't panic.
	// The return value is either the real hostname or "" on error, both acceptable.
	_ = resolveHost(model.MetricsConfig{})
}
