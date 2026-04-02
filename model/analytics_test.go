package model

import (
	"encoding"
	"testing"
	"time"
)

func TestTrendDirectionMarshalText(t *testing.T) {
	tests := []struct {
		name string
		td   TrendDirection
		want string
	}{
		{"stable", TrendStable, "stable"},
		{"up", TrendUp, "up"},
		{"down", TrendDown, "down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.td.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("MarshalText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrendDirectionUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    TrendDirection
		wantErr bool
	}{
		{"stable", "stable", TrendStable, false},
		{"up", "up", TrendUp, false},
		{"down", "down", TrendDown, false},
		{"invalid", "sideways", TrendStable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var td TrendDirection
			err := td.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && td != tt.want {
				t.Errorf("UnmarshalText() = %v, want %v", td, tt.want)
			}
		})
	}
}

// Verify interface compliance at compile time.
var (
	_ encoding.TextMarshaler   = TrendDirection(0)
	_ encoding.TextUnmarshaler = (*TrendDirection)(nil)
)

func TestSparkline(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{
			name:   "ascending 8 values",
			values: []float64{1, 2, 3, 4, 5, 6, 7, 8},
			want:   "▁▂▃▄▅▆▇█",
		},
		{
			name:   "all equal",
			values: []float64{5, 5, 5, 5},
			want:   "▄▄▄▄",
		},
		{
			name:   "single value",
			values: []float64{42},
			want:   "▄",
		},
		{
			name:   "two values min max",
			values: []float64{0, 100},
			want:   "▁█",
		},
		{
			name:   "descending",
			values: []float64{8, 7, 6, 5, 4, 3, 2, 1},
			want:   "█▇▆▅▄▃▂▁",
		},
		{
			name:   "empty returns empty",
			values: nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sparkline(tt.values)
			if got != tt.want {
				t.Errorf("sparkline() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPeakComparison(t *testing.T) {
	makePeakEntry := func(hour int, dl float64) *SpeedTestResult {
		return &SpeedTestResult{
			DownloadSpeed: dl,
			Timestamp:     time.Date(2026, 3, 15, hour, 0, 0, 0, time.Local),
		}
	}

	extractDL := func(r *SpeedTestResult) float64 { return r.DownloadSpeed }

	tests := []struct {
		name        string
		entries     []*SpeedTestResult
		wantPeakAvg float64
		wantOffAvg  float64
		wantPeakN   int
		wantOffN    int
	}{
		{
			name: "mixed peak and off-peak",
			entries: []*SpeedTestResult{
				makePeakEntry(10, 100), // peak
				makePeakEntry(14, 80),  // peak
				makePeakEntry(22, 120), // off-peak
			},
			wantPeakAvg: 90,
			wantOffAvg:  120,
			wantPeakN:   2,
			wantOffN:    1,
		},
		{
			name: "boundary hours: 9 is peak, 21 is off-peak",
			entries: []*SpeedTestResult{
				makePeakEntry(9, 50),  // peak (exactly 09:00)
				makePeakEntry(20, 70), // peak (20:59 range)
				makePeakEntry(21, 90), // off-peak (exactly 21:00)
				makePeakEntry(8, 110), // off-peak (08:xx)
			},
			wantPeakAvg: 60,
			wantOffAvg:  100,
			wantPeakN:   2,
			wantOffN:    2,
		},
		{
			name: "all peak",
			entries: []*SpeedTestResult{
				makePeakEntry(12, 100),
				makePeakEntry(15, 100),
			},
			wantPeakAvg: 100,
			wantOffAvg:  0,
			wantPeakN:   2,
			wantOffN:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peakComparison(tt.entries, extractDL)
			if got.PeakAvg != tt.wantPeakAvg {
				t.Errorf("PeakAvg = %v, want %v", got.PeakAvg, tt.wantPeakAvg)
			}
			if got.OffPeakAvg != tt.wantOffAvg {
				t.Errorf("OffPeakAvg = %v, want %v", got.OffPeakAvg, tt.wantOffAvg)
			}
			if got.PeakCount != tt.wantPeakN {
				t.Errorf("PeakCount = %v, want %v", got.PeakCount, tt.wantPeakN)
			}
			if got.OffPeakCount != tt.wantOffN {
				t.Errorf("OffPeakCount = %v, want %v", got.OffPeakCount, tt.wantOffN)
			}
		})
	}
}

func TestComputeSummary(t *testing.T) {
	t.Run("nil entries returns nil", func(t *testing.T) {
		if got := ComputeSummary(nil); got != nil {
			t.Errorf("ComputeSummary(nil) = %v, want nil", got)
		}
	})

	t.Run("empty entries returns nil", func(t *testing.T) {
		if got := ComputeSummary([]*SpeedTestResult{}); got != nil {
			t.Errorf("ComputeSummary([]) = %v, want nil", got)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		entry := &SpeedTestResult{
			DownloadSpeed: 100,
			UploadSpeed:   50,
			Ping:          10,
			Timestamp:     time.Date(2026, 3, 15, 14, 0, 0, 0, time.Local),
		}
		got := ComputeSummary([]*SpeedTestResult{entry})
		if got == nil {
			t.Fatal("ComputeSummary returned nil for single entry")
		}
		if got.Download.Average != 100 {
			t.Errorf("Download.Average = %v, want 100", got.Download.Average)
		}
		if got.Download.Trend != TrendStable {
			t.Errorf("Download.Trend = %v, want TrendStable", got.Download.Trend)
		}
		if got.TotalTests != 1 {
			t.Errorf("TotalTests = %v, want 1", got.TotalTests)
		}
		if len([]rune(got.Download.Sparkline)) != 1 {
			t.Errorf("Sparkline length = %d, want 1", len([]rune(got.Download.Sparkline)))
		}
	})

	t.Run("trend up when recent results higher", func(t *testing.T) {
		entries := make([]*SpeedTestResult, 10)
		for i := range entries {
			dl := 100.0
			if i >= 5 {
				dl = 120.0 // last 5 are 20% higher
			}
			entries[i] = &SpeedTestResult{
				DownloadSpeed: dl,
				UploadSpeed:   50,
				Ping:          10,
				Timestamp:     time.Date(2026, 3, 1+i, 14, 0, 0, 0, time.Local),
			}
		}
		got := ComputeSummary(entries)
		if got.Download.Trend != TrendUp {
			t.Errorf("Download.Trend = %v, want TrendUp", got.Download.Trend)
		}
	})

	t.Run("trend down when recent results lower", func(t *testing.T) {
		entries := make([]*SpeedTestResult, 10)
		for i := range entries {
			dl := 100.0
			if i >= 5 {
				dl = 80.0 // last 5 are 20% lower
			}
			entries[i] = &SpeedTestResult{
				DownloadSpeed: dl,
				UploadSpeed:   50,
				Ping:          10,
				Timestamp:     time.Date(2026, 3, 1+i, 14, 0, 0, 0, time.Local),
			}
		}
		got := ComputeSummary(entries)
		if got.Download.Trend != TrendDown {
			t.Errorf("Download.Trend = %v, want TrendDown", got.Download.Trend)
		}
	})

	t.Run("trend stable when recent results similar", func(t *testing.T) {
		entries := make([]*SpeedTestResult, 10)
		for i := range entries {
			entries[i] = &SpeedTestResult{
				DownloadSpeed: 100 + float64(i%3), // tiny variance
				UploadSpeed:   50,
				Ping:          10,
				Timestamp:     time.Date(2026, 3, 1+i, 14, 0, 0, 0, time.Local),
			}
		}
		got := ComputeSummary(entries)
		if got.Download.Trend != TrendStable {
			t.Errorf("Download.Trend = %v, want TrendStable", got.Download.Trend)
		}
	})
}
