package model

import "testing"

func TestFindBestMetrics(t *testing.T) {
	tests := []struct {
		name    string
		results []*SpeedTestResult
		want    BestMetrics
	}{
		{
			name: "distinct winners",
			results: []*SpeedTestResult{
				{DownloadSpeed: 50, UploadSpeed: 10, Ping: 20, Jitter: 5},
				{DownloadSpeed: 100, UploadSpeed: 5, Ping: 10, Jitter: 8},
				{DownloadSpeed: 75, UploadSpeed: 20, Ping: 30, Jitter: 2},
			},
			want: BestMetrics{DownloadIdx: 1, UploadIdx: 2, PingIdx: 1, JitterIdx: 2},
		},
		{
			name: "all equal returns -1",
			results: []*SpeedTestResult{
				{DownloadSpeed: 50, UploadSpeed: 10, Ping: 20, Jitter: 5},
				{DownloadSpeed: 50, UploadSpeed: 10, Ping: 20, Jitter: 5},
			},
			want: BestMetrics{DownloadIdx: -1, UploadIdx: -1, PingIdx: -1, JitterIdx: -1},
		},
		{
			name: "single result returns -1",
			results: []*SpeedTestResult{
				{DownloadSpeed: 100, UploadSpeed: 50, Ping: 10, Jitter: 2},
			},
			want: BestMetrics{DownloadIdx: -1, UploadIdx: -1, PingIdx: -1, JitterIdx: -1},
		},
		{
			name: "partial equality",
			results: []*SpeedTestResult{
				{DownloadSpeed: 50, UploadSpeed: 10, Ping: 20, Jitter: 5},
				{DownloadSpeed: 50, UploadSpeed: 20, Ping: 10, Jitter: 5},
			},
			want: BestMetrics{DownloadIdx: -1, UploadIdx: 1, PingIdx: 1, JitterIdx: -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindBestMetrics(tt.results)
			if got != tt.want {
				t.Errorf("FindBestMetrics() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
