package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

func TestAnalyticsCommandRegistered(t *testing.T) {
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "analytics" {
			found = true
			break
		}
	}
	if !found {
		t.Error("analytics command not registered")
	}
}

func TestAnalyticsLastFlagValidation(t *testing.T) {
	origLast := analyticsLast
	t.Cleanup(func() { analyticsLast = origLast })

	analyticsLast = -1
	err := analyticsCmd.RunE(analyticsCmd, nil)
	if err == nil {
		t.Error("expected error for negative --last")
	}
}

func TestAnalyticsSimpleLine(t *testing.T) {
	s := &model.Summary{
		Download: model.MetricSummary{Average: 94.2, Trend: model.TrendUp, TrendPct: 12.3},
		Upload:   model.MetricSummary{Average: 38.7, Trend: model.TrendDown, TrendPct: -3.1},
		Ping:     model.MetricSummary{Average: 12.4, Trend: model.TrendStable},
	}
	got := analyticsSimpleLine(s)
	if !strings.Contains(got, "94.2") {
		t.Errorf("simple line missing download avg, got %q", got)
	}
	if !strings.Contains(got, "38.7") {
		t.Errorf("simple line missing upload avg, got %q", got)
	}
	if !strings.Contains(got, "stable") {
		t.Errorf("simple line missing stable indicator, got %q", got)
	}
}

func TestAnalyticsDefaultOutput(t *testing.T) {
	s := &model.Summary{
		Download:     model.MetricSummary{Average: 94.2, Sparkline: "▃▄▅▆▇", Trend: model.TrendUp, TrendPct: 12.3},
		Upload:       model.MetricSummary{Average: 38.7, Sparkline: "▂▃▃▄▅", Trend: model.TrendDown, TrendPct: -3.1},
		Ping:         model.MetricSummary{Average: 12.4, Sparkline: "▆▅▄▃▂", Trend: model.TrendStable},
		PeakDownload: model.PeakComparison{PeakAvg: 82.3, OffPeakAvg: 103.1, PeakCount: 14, OffPeakCount: 10},
		PeakUpload:   model.PeakComparison{PeakAvg: 35.1, OffPeakAvg: 41.8, PeakCount: 14, OffPeakCount: 10},
		TotalTests:   24,
		DateRange: [2]time.Time{
			time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	got := analyticsDefaultOutput(s)
	checks := []string{"Analytics", "24 tests", "▃▄▅▆▇", "94.2", "Peak", "Off-Peak"}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("default output missing %q", want)
		}
	}
}

func TestAnalyticsJSONOutput(t *testing.T) {
	s := &model.Summary{
		Download:   model.MetricSummary{Average: 100, Trend: model.TrendUp, TrendPct: 10},
		Upload:     model.MetricSummary{Average: 50, Trend: model.TrendDown, TrendPct: -5},
		Ping:       model.MetricSummary{Average: 10, Trend: model.TrendStable},
		TotalTests: 5,
		DateRange: [2]time.Time{
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// TrendDirection should serialize as string, not int
	raw := string(data)
	if !strings.Contains(raw, `"up"`) {
		t.Errorf("expected TrendDirection 'up' as string in JSON, got %s", raw)
	}
	if !strings.Contains(raw, `"down"`) {
		t.Errorf("expected TrendDirection 'down' as string in JSON, got %s", raw)
	}
	if !strings.Contains(raw, `"stable"`) {
		t.Errorf("expected TrendDirection 'stable' as string in JSON, got %s", raw)
	}
}

func TestRunAnalyticsEmptyHistory(t *testing.T) {
	// Save and restore package-level flags
	origJSON, origSimple, origLast := analyticsJSON, analyticsSimple, analyticsLast
	t.Cleanup(func() {
		analyticsJSON = origJSON
		analyticsSimple = origSimple
		analyticsLast = origLast
	})

	// Redirect stdout to capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	// Set XDG_DATA_HOME to a temp dir so history is empty
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	analyticsJSON = false
	analyticsSimple = false
	analyticsLast = 0
	runAnalytics()

	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe: %v", err)
	}
	out, _ := io.ReadAll(r)
	got := string(out)

	if !strings.Contains(got, "No test data yet") {
		t.Errorf("expected 'No test data yet' for empty history, got %q", got)
	}
}
