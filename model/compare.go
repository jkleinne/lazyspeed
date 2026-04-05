package model

// BestMetrics identifies the result indices with the best value for each metric.
// An index of -1 means all values were equal (no winner to highlight).
type BestMetrics struct {
	DownloadIdx int
	UploadIdx   int
	PingIdx     int
	JitterIdx   int
}

// bestIndex returns the index of the best value extracted from results.
// "Best" is defined by the better comparator (e.g. a > b for highest).
// Returns -1 when all extracted values are equal or results has fewer than 2 elements.
func bestIndex(results []*SpeedTestResult, extract func(*SpeedTestResult) float64, better func(a, b float64) bool) int {
	if len(results) < 2 {
		return -1
	}

	allEqual := true
	bestIdx := 0
	first := extract(results[0])
	bestVal := first

	for i := 1; i < len(results); i++ {
		v := extract(results[i])
		if v != first {
			allEqual = false
		}
		if better(v, bestVal) {
			bestIdx = i
			bestVal = v
		}
	}

	if allEqual {
		return -1
	}
	return bestIdx
}

func higher(a, b float64) bool { return a > b }
func lower(a, b float64) bool  { return a < b }

// FindBestMetrics scans results and returns the index of the best value for
// each metric: highest download/upload, lowest ping/jitter. Returns -1 for a
// metric when all values are identical.
func FindBestMetrics(results []*SpeedTestResult) BestMetrics {
	return BestMetrics{
		DownloadIdx: bestIndex(results, func(r *SpeedTestResult) float64 { return r.DownloadSpeed }, higher),
		UploadIdx:   bestIndex(results, func(r *SpeedTestResult) float64 { return r.UploadSpeed }, higher),
		PingIdx:     bestIndex(results, func(r *SpeedTestResult) float64 { return r.Ping }, lower),
		JitterIdx:   bestIndex(results, func(r *SpeedTestResult) float64 { return r.Jitter }, lower),
	}
}
