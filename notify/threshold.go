package notify

import "github.com/jkleinne/lazyspeed/model"

// Breach records a single threshold violation, capturing the metric name,
// the measured value, and the configured threshold that was crossed.
type Breach struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

// EvaluateThresholds checks result against the configured thresholds and
// returns a description of every violated threshold.
//
// Return value semantics:
//   - nil: no thresholds configured (all pointers nil). The caller should
//     treat this as "always fire" since filtering is not active.
//   - non-nil empty slice: thresholds are active and all passed. The caller
//     should skip firing.
//   - non-nil non-empty slice: one or more thresholds were violated. The
//     caller should fire and may include the breach list in the payload.
//
// Boundary values (result exactly equal to the threshold) are not breaches;
// strict inequality is used for both min and max checks.
func EvaluateThresholds(result *model.SpeedTestResult, thresholds model.ThresholdConfig) []Breach {
	if thresholds.MinDownload == nil && thresholds.MinUpload == nil &&
		thresholds.MaxPing == nil && thresholds.MaxJitter == nil {
		return nil
	}

	var breaches []Breach

	if thresholds.MinDownload != nil && result.DownloadSpeed < *thresholds.MinDownload {
		breaches = append(breaches, Breach{
			Metric:    "download",
			Value:     result.DownloadSpeed,
			Threshold: *thresholds.MinDownload,
		})
	}
	if thresholds.MinUpload != nil && result.UploadSpeed < *thresholds.MinUpload {
		breaches = append(breaches, Breach{
			Metric:    "upload",
			Value:     result.UploadSpeed,
			Threshold: *thresholds.MinUpload,
		})
	}
	if thresholds.MaxPing != nil && result.Ping > *thresholds.MaxPing {
		breaches = append(breaches, Breach{
			Metric:    "ping",
			Value:     result.Ping,
			Threshold: *thresholds.MaxPing,
		})
	}
	if thresholds.MaxJitter != nil && result.Jitter > *thresholds.MaxJitter {
		breaches = append(breaches, Breach{
			Metric:    "jitter",
			Value:     result.Jitter,
			Threshold: *thresholds.MaxJitter,
		})
	}

	if breaches == nil {
		breaches = []Breach{}
	}
	return breaches
}
