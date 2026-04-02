package model

import (
	"fmt"
	"time"
)

// TrendDirection indicates whether a metric is trending up, down, or stable.
type TrendDirection int

const (
	TrendStable TrendDirection = iota
	TrendUp
	TrendDown
)

var trendLabels = [...]string{TrendStable: "stable", TrendUp: "up", TrendDown: "down"}

// MarshalText implements encoding.TextMarshaler for JSON string serialization.
func (d TrendDirection) MarshalText() ([]byte, error) {
	if int(d) < len(trendLabels) {
		return []byte(trendLabels[d]), nil
	}
	return nil, fmt.Errorf("unknown TrendDirection %d", d)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *TrendDirection) UnmarshalText(text []byte) error {
	s := string(text)
	for i, label := range trendLabels {
		if label == s {
			*d = TrendDirection(i)
			return nil
		}
	}
	return fmt.Errorf("unknown TrendDirection %q", s)
}

// MetricSummary holds computed analytics for a single metric.
type MetricSummary struct {
	Average   float64        `json:"average"`
	Min       float64        `json:"min"`
	Max       float64        `json:"max"`
	Latest    float64        `json:"latest"`
	Trend     TrendDirection `json:"trend"`
	TrendPct  float64        `json:"trend_pct"`
	Sparkline string         `json:"sparkline"`
}

// PeakComparison holds peak vs off-peak averages for a metric.
type PeakComparison struct {
	PeakAvg      float64 `json:"peak_avg"`
	OffPeakAvg   float64 `json:"off_peak_avg"`
	PeakCount    int     `json:"peak_count"`
	OffPeakCount int     `json:"off_peak_count"`
}

// Summary is the complete analytics result computed from history entries.
type Summary struct {
	Download     MetricSummary  `json:"download"`
	Upload       MetricSummary  `json:"upload"`
	Ping         MetricSummary  `json:"ping"`
	PeakDownload PeakComparison `json:"peak_download"`
	PeakUpload   PeakComparison `json:"peak_upload"`
	TotalTests   int            `json:"total_tests"`
	DateRange    [2]time.Time   `json:"date_range"`
}
