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

// TrendLabel returns a plain-text trend indicator (e.g. "↑12.3%", "↓3.1%", "stable").
func (ms MetricSummary) TrendLabel() string {
	switch ms.Trend {
	case TrendUp:
		return fmt.Sprintf("↑%.1f%%", ms.TrendPct)
	case TrendDown:
		return fmt.Sprintf("↓%.1f%%", -ms.TrendPct)
	case TrendStable:
		return "stable"
	default:
		return "stable"
	}
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

const sparkBlocks = "▁▂▃▄▅▆▇█"

var sparkRunes = []rune(sparkBlocks)

const (
	// PeakStartHour is the start of peak hours (inclusive). Exported for UI display.
	PeakStartHour = 9
	// PeakEndHour is the end of peak hours (exclusive). Exported for UI display.
	PeakEndHour       = 21
	trendRecentWindow = 5
	trendThresholdPct = 5.0
)

// peakComparison splits entries into peak (09:00–20:59) and off-peak buckets
// and computes the average for each using the extract function.
func peakComparison(entries []*SpeedTestResult, extract func(*SpeedTestResult) float64) PeakComparison {
	var peakSum, offSum float64
	var peakN, offN int

	for _, e := range entries {
		h := e.Timestamp.Local().Hour()
		v := extract(e)
		if h >= PeakStartHour && h < PeakEndHour {
			peakSum += v
			peakN++
		} else {
			offSum += v
			offN++
		}
	}

	var pc PeakComparison
	pc.PeakCount = peakN
	pc.OffPeakCount = offN
	if peakN > 0 {
		pc.PeakAvg = peakSum / float64(peakN)
	}
	if offN > 0 {
		pc.OffPeakAvg = offSum / float64(offN)
	}
	return pc
}

// detectTrend compares the recent window average to the overall average
// and returns the trend direction and percentage change.
func detectTrend(values []float64, avg float64) (TrendDirection, float64) {
	if len(values) < trendRecentWindow || avg == 0 {
		return TrendStable, 0
	}

	var recentSum float64
	for _, v := range values[len(values)-trendRecentWindow:] {
		recentSum += v
	}
	recentAvg := recentSum / float64(trendRecentWindow)
	pct := (recentAvg - avg) / avg * 100

	if pct > trendThresholdPct {
		return TrendUp, pct
	}
	if pct < -trendThresholdPct {
		return TrendDown, pct
	}
	return TrendStable, pct
}

// metricSummary computes a MetricSummary for a single metric extracted from entries.
func metricSummary(entries []*SpeedTestResult, extract func(*SpeedTestResult) float64) MetricSummary {
	values := make([]float64, len(entries))
	var sum, lo, hi float64
	for i, e := range entries {
		v := extract(e)
		values[i] = v
		sum += v
		if i == 0 || v < lo {
			lo = v
		}
		if i == 0 || v > hi {
			hi = v
		}
	}

	n := float64(len(entries))
	avg := sum / n

	ms := MetricSummary{
		Average:   avg,
		Min:       lo,
		Max:       hi,
		Latest:    values[len(values)-1],
		Sparkline: sparkline(values),
	}

	ms.Trend, ms.TrendPct = detectTrend(values, avg)

	return ms
}

// ComputeSummary computes analytics from history entries.
// Returns nil if entries is empty.
func ComputeSummary(entries []*SpeedTestResult) *Summary {
	if len(entries) == 0 {
		return nil
	}

	extractDL := func(r *SpeedTestResult) float64 { return r.DownloadSpeed }
	extractUL := func(r *SpeedTestResult) float64 { return r.UploadSpeed }
	extractPing := func(r *SpeedTestResult) float64 { return r.Ping }

	return &Summary{
		Download:     metricSummary(entries, extractDL),
		Upload:       metricSummary(entries, extractUL),
		Ping:         metricSummary(entries, extractPing),
		PeakDownload: peakComparison(entries, extractDL),
		PeakUpload:   peakComparison(entries, extractUL),
		TotalTests:   len(entries),
		DateRange:    [2]time.Time{entries[0].Timestamp, entries[len(entries)-1].Timestamp},
	}
}

// sparkline maps values to a Unicode block-element sparkline string.
// Each value becomes one character. All-equal values render as mid-level (▄).
func sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}

	lo, hi := values[0], values[0]
	for _, v := range values[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}

	levels := len(sparkRunes)
	midLevel := (levels - 1) / 2
	rng := hi - lo

	runes := make([]rune, len(values))
	for i, v := range values {
		if rng == 0 {
			runes[i] = sparkRunes[midLevel]
		} else {
			idx := int((v - lo) / rng * float64(levels-1))
			runes[i] = sparkRunes[idx]
		}
	}
	return string(runes)
}
