package notify

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

func TestNewPayload_TestComplete(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 142.5,
		UploadSpeed:   45.3,
		Ping:          12.4,
		Jitter:        1.8,
		ServerName:    "Stockholm",
		Timestamp:     time.Date(2026, 4, 6, 14, 30, 0, 0, time.UTC),
	}

	now := time.Now()
	p := NewPayload(result, nil, "1.3.0", now)
	if p.Event != EventTestComplete {
		t.Errorf("expected event %q, got %q", EventTestComplete, p.Event)
	}
	if p.Breaches != nil {
		t.Errorf("expected nil breaches, got %v", p.Breaches)
	}
	if p.Result != result {
		t.Error("expected result pointer to match")
	}
	if p.Version != "1.3.0" {
		t.Errorf("expected version '1.3.0', got %q", p.Version)
	}
	if p.Timestamp != now {
		t.Errorf("expected timestamp %v, got %v", now, p.Timestamp)
	}
}

func TestNewPayload_ThresholdBreach(t *testing.T) {
	result := &model.SpeedTestResult{DownloadSpeed: 42.1}
	breaches := []Breach{{Metric: "download", Value: 42.1, Threshold: 50.0}}

	p := NewPayload(result, breaches, "1.3.0", time.Now())
	if p.Event != EventThresholdBreach {
		t.Errorf("expected event %q, got %q", EventThresholdBreach, p.Event)
	}
	if len(p.Breaches) != 1 {
		t.Fatalf("expected 1 breach, got %d", len(p.Breaches))
	}
}

func TestPayload_JSONStructure(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 94.52,
		UploadSpeed:   38.71,
		Ping:          12.4,
		Jitter:        1.8,
		ServerName:    "Tokyo",
		ServerSponsor: "ACME Corp",
		Country:       "Japan",
		Distance:      42.3,
		Timestamp:     time.Date(2026, 4, 6, 14, 30, 0, 0, time.UTC),
		UserIP:        "203.0.113.42",
		UserISP:       "Example ISP",
	}

	p := NewPayload(result, nil, "1.3.0", time.Now())
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	requiredKeys := []string{"event", "breaches", "result", "version", "timestamp"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in payload JSON", key)
		}
	}

	var resultMap map[string]json.RawMessage
	if err := json.Unmarshal(raw["result"], &resultMap); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	resultKeys := []string{"download_speed", "upload_speed", "ping", "jitter", "server_name", "server_sponsor", "server_country", "distance", "timestamp", "user_ip", "user_isp"}
	for _, key := range resultKeys {
		if _, ok := resultMap[key]; !ok {
			t.Errorf("missing key %q in result JSON", key)
		}
	}
}
