package notify

import (
	"testing"

	"github.com/jkleinne/lazyspeed/model"
)

// ptrFloat64 returns a pointer to v, used to build threshold configs inline.
func ptrFloat64(v float64) *float64 { return &v }

func TestEvaluateThresholds(t *testing.T) {
	tests := []struct {
		name       string
		result     model.SpeedTestResult
		thresholds model.ThresholdConfig
		// wantNil true means the return must be nil (no thresholds configured).
		// wantNil false means a non-nil slice is expected; wantLen gives its length.
		wantNil bool
		wantLen int
	}{
		{
			name: "no thresholds configured returns nil",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   50,
				Ping:          10,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{},
			wantNil:    true,
		},
		{
			name: "all thresholds pass returns empty non-nil slice",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   50,
				Ping:          10,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MinDownload: ptrFloat64(50),
				MinUpload:   ptrFloat64(25),
				MaxPing:     ptrFloat64(100),
				MaxJitter:   ptrFloat64(10),
			},
			wantNil: false,
			wantLen: 0,
		},
		{
			name: "download below minimum breaches",
			result: model.SpeedTestResult{
				DownloadSpeed: 30,
				UploadSpeed:   50,
				Ping:          10,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MinDownload: ptrFloat64(50),
			},
			wantNil: false,
			wantLen: 1,
		},
		{
			name: "upload below minimum breaches",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   10,
				Ping:          10,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MinUpload: ptrFloat64(25),
			},
			wantNil: false,
			wantLen: 1,
		},
		{
			name: "ping above maximum breaches",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   50,
				Ping:          150,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MaxPing: ptrFloat64(100),
			},
			wantNil: false,
			wantLen: 1,
		},
		{
			name: "jitter above maximum breaches",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   50,
				Ping:          10,
				Jitter:        20,
			},
			thresholds: model.ThresholdConfig{
				MaxJitter: ptrFloat64(10),
			},
			wantNil: false,
			wantLen: 1,
		},
		{
			name: "multiple breaches all reported",
			result: model.SpeedTestResult{
				DownloadSpeed: 10,
				UploadSpeed:   5,
				Ping:          200,
				Jitter:        50,
			},
			thresholds: model.ThresholdConfig{
				MinDownload: ptrFloat64(50),
				MinUpload:   ptrFloat64(25),
				MaxPing:     ptrFloat64(100),
				MaxJitter:   ptrFloat64(10),
			},
			wantNil: false,
			wantLen: 4,
		},
		{
			name: "download exactly at minimum is not a breach",
			result: model.SpeedTestResult{
				DownloadSpeed: 50,
				UploadSpeed:   50,
				Ping:          10,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MinDownload: ptrFloat64(50),
			},
			wantNil: false,
			wantLen: 0,
		},
		{
			name: "upload exactly at minimum is not a breach",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   25,
				Ping:          10,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MinUpload: ptrFloat64(25),
			},
			wantNil: false,
			wantLen: 0,
		},
		{
			name: "ping exactly at maximum is not a breach",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   50,
				Ping:          100,
				Jitter:        2,
			},
			thresholds: model.ThresholdConfig{
				MaxPing: ptrFloat64(100),
			},
			wantNil: false,
			wantLen: 0,
		},
		{
			name: "jitter exactly at maximum is not a breach",
			result: model.SpeedTestResult{
				DownloadSpeed: 100,
				UploadSpeed:   50,
				Ping:          10,
				Jitter:        10,
			},
			thresholds: model.ThresholdConfig{
				MaxJitter: ptrFloat64(10),
			},
			wantNil: false,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateThresholds(&tt.result, tt.thresholds)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil slice, got nil")
			}
			if len(got) != tt.wantLen {
				t.Errorf("expected %d breach(es), got %d: %v", tt.wantLen, len(got), got)
			}
		})
	}
}

func TestEvaluateThresholds_BreachFields(t *testing.T) {
	result := &model.SpeedTestResult{
		DownloadSpeed: 30,
		UploadSpeed:   10,
		Ping:          200,
		Jitter:        25,
	}
	thresholds := model.ThresholdConfig{
		MinDownload: ptrFloat64(50),
		MinUpload:   ptrFloat64(25),
		MaxPing:     ptrFloat64(100),
		MaxJitter:   ptrFloat64(10),
	}

	breaches := EvaluateThresholds(result, thresholds)
	if len(breaches) != 4 {
		t.Fatalf("expected 4 breaches, got %d", len(breaches))
	}

	// Verify each breach has the correct metric name, measured value, and threshold.
	type want struct {
		metric    string
		value     float64
		threshold float64
	}
	expected := []want{
		{MetricDownload, 30, 50},
		{MetricUpload, 10, 25},
		{MetricPing, 200, 100},
		{MetricJitter, 25, 10},
	}
	for i, b := range breaches {
		w := expected[i]
		if b.Metric != w.metric {
			t.Errorf("breach[%d].Metric: got %q, want %q", i, b.Metric, w.metric)
		}
		if b.Value != w.value {
			t.Errorf("breach[%d].Value: got %f, want %f", i, b.Value, w.value)
		}
		if b.Threshold != w.threshold {
			t.Errorf("breach[%d].Threshold: got %f, want %f", i, b.Threshold, w.threshold)
		}
	}
}
