package metrics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

// mockSender is a function-field test double for the Sender interface.
type mockSender struct {
	DoFn    func(*http.Request) (*http.Response, error)
	Calls   int32
	LastReq *http.Request
}

func (m *mockSender) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&m.Calls, 1)
	m.LastReq = req
	if m.DoFn != nil {
		return m.DoFn(req)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func okResponse() *http.Response {
	return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader(""))}
}

func v2Endpoint() model.MetricsEndpoint {
	return model.MetricsEndpoint{
		URL: "https://example.com",
		V2:  &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"},
	}
}

func TestWriteOne_Success2xx(t *testing.T) {
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		return okResponse(), nil
	}}
	err := writeOne(context.Background(), sender, v2Endpoint(), []byte("line\n"), time.Second, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.Calls != 1 {
		t.Errorf("expected 1 call, got %d", sender.Calls)
	}
}

func TestWriteOne_4xxPermanent(t *testing.T) {
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(""))}, nil
	}}
	err := writeOne(context.Background(), sender, v2Endpoint(), []byte("line\n"), time.Second, 3)
	if err == nil {
		t.Fatal("expected error")
	}
	if sender.Calls != 1 {
		t.Errorf("expected exactly 1 attempt for 4xx, got %d", sender.Calls)
	}
}

func TestWriteOne_5xxRetriedThenFails(t *testing.T) {
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))}, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := writeOne(ctx, sender, v2Endpoint(), []byte("line\n"), time.Second, 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if sender.Calls != 2 {
		t.Errorf("expected 2 attempts, got %d", sender.Calls)
	}
}

func TestWriteOne_NetworkErrorRetried(t *testing.T) {
	var calls int32
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			return nil, errors.New("dial refused")
		}
		return okResponse(), nil
	}}
	err := writeOne(context.Background(), sender, v2Endpoint(), []byte("line\n"), time.Second, 3)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
}

func TestWriteOne_ContextCancelledDuringBackoff(t *testing.T) {
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))}, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err := writeOne(ctx, sender, v2Endpoint(), []byte("line\n"), time.Second, 5)
	if err == nil {
		t.Fatal("expected cancelled error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected cancelled error, got %v", err)
	}
}

func TestBuildWriteURL_V2(t *testing.T) {
	ep := model.MetricsEndpoint{
		URL: "https://influx.example.com:8086",
		V2:  &model.InfluxV2{Token: "t", Org: "my-org", Bucket: "speedtest"},
	}
	got, err := buildWriteURL(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://influx.example.com:8086/api/v2/write?bucket=speedtest&org=my-org&precision=ns"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestBuildWriteURL_V1(t *testing.T) {
	ep := model.MetricsEndpoint{
		URL: "http://localhost:8086",
		V1:  &model.InfluxV1{Database: "lazyspeed"},
	}
	got, err := buildWriteURL(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://localhost:8086/write?db=lazyspeed"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestBuildWriteURL_V2WithPathPrefix(t *testing.T) {
	ep := model.MetricsEndpoint{
		URL: "https://proxy.example.com/influx",
		V2:  &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"},
	}
	got, err := buildWriteURL(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "https://proxy.example.com/influx/api/v2/write") {
		t.Errorf("path prefix lost: %q", got)
	}
}

func TestBuildWriteURL_V2WithTrailingSlash(t *testing.T) {
	ep := model.MetricsEndpoint{
		URL: "https://proxy.example.com/influx/",
		V2:  &model.InfluxV2{Token: "t", Org: "o", Bucket: "b"},
	}
	got, err := buildWriteURL(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "/influx/api/v2/write") {
		t.Errorf("double slash handling wrong: %q", got)
	}
	if strings.Contains(got, "//api") {
		t.Errorf("double slash in suffix: %q", got)
	}
}

func TestWriteOne_V2AuthHeader(t *testing.T) {
	var seenAuth string
	sender := &mockSender{DoFn: func(req *http.Request) (*http.Response, error) {
		seenAuth = req.Header.Get("Authorization")
		return okResponse(), nil
	}}
	ep := model.MetricsEndpoint{
		URL: "https://example.com",
		V2:  &model.InfluxV2{Token: "secret-token", Org: "o", Bucket: "b"},
	}
	if err := writeOne(context.Background(), sender, ep, []byte("line\n"), time.Second, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenAuth != "Token secret-token" {
		t.Errorf("got auth header %q, want %q", seenAuth, "Token secret-token")
	}
}

func TestWriteOne_V1BasicAuth(t *testing.T) {
	var seenUser, seenPass string
	var hadAuth bool
	sender := &mockSender{DoFn: func(req *http.Request) (*http.Response, error) {
		seenUser, seenPass, hadAuth = req.BasicAuth()
		return okResponse(), nil
	}}
	ep := model.MetricsEndpoint{
		URL: "http://localhost:8086",
		V1:  &model.InfluxV1{Database: "db", Username: "admin", Password: "pw"},
	}
	if err := writeOne(context.Background(), sender, ep, []byte("line\n"), time.Second, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hadAuth {
		t.Fatal("expected basic auth header")
	}
	if seenUser != "admin" || seenPass != "pw" {
		t.Errorf("got %q/%q, want admin/pw", seenUser, seenPass)
	}
}

func TestWriteOne_V1NoAuthWhenUsernameEmpty(t *testing.T) {
	var hadAuth bool
	sender := &mockSender{DoFn: func(req *http.Request) (*http.Response, error) {
		_, _, hadAuth = req.BasicAuth()
		return okResponse(), nil
	}}
	ep := model.MetricsEndpoint{
		URL: "http://localhost:8086",
		V1:  &model.InfluxV1{Database: "db"},
	}
	if err := writeOne(context.Background(), sender, ep, []byte("line\n"), time.Second, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hadAuth {
		t.Error("expected no basic auth when username is empty")
	}
}

func TestWriteOne_ZeroMaxRetriesRejected(t *testing.T) {
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		t.Fatal("sender should not be called when maxRetries < 1")
		return nil, nil
	}}
	err := writeOne(context.Background(), sender, v2Endpoint(), []byte("line\n"), time.Second, 0)
	if err == nil {
		t.Fatal("expected error for maxRetries=0, got nil")
	}
	if !strings.Contains(err.Error(), "maxRetries") {
		t.Errorf("expected maxRetries in error, got %q", err.Error())
	}
}

// recordingBody tracks whether Read and Close were called. Used to verify
// writeOne drains the response body before closing it.
type recordingBody struct {
	read   bool
	closed bool
}

func (r *recordingBody) Read(_ []byte) (int, error) {
	r.read = true
	return 0, io.EOF
}

func (r *recordingBody) Close() error {
	r.closed = true
	return nil
}

func TestWriteOne_DrainsBodyBeforeClose(t *testing.T) {
	rb := &recordingBody{}
	sender := &mockSender{DoFn: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: rb}, nil
	}}
	if err := writeOne(context.Background(), sender, v2Endpoint(), []byte("line\n"), time.Second, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rb.read {
		t.Error("expected body to be read (drained)")
	}
	if !rb.closed {
		t.Error("expected body to be closed")
	}
}
