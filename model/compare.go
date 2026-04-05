package model

// BestMetrics identifies the result indices with the best value for each metric.
// An index of -1 means all values were equal (no winner to highlight).
type BestMetrics struct {
	DownloadIdx int
	UploadIdx   int
	PingIdx     int
	JitterIdx   int
}

// FindBestMetrics scans results and returns the index of the best value for
// each metric: highest download/upload, lowest ping/jitter. Returns -1 for a
// metric when all values are identical.
func FindBestMetrics(results []*SpeedTestResult) BestMetrics {
	bm := BestMetrics{}
	allDLEqual := true
	allULEqual := true
	allPingEqual := true
	allJitterEqual := true

	for i, res := range results {
		if i == 0 {
			continue
		}
		if res.DownloadSpeed != results[0].DownloadSpeed {
			allDLEqual = false
		}
		if res.UploadSpeed != results[0].UploadSpeed {
			allULEqual = false
		}
		if res.Ping != results[0].Ping {
			allPingEqual = false
		}
		if res.Jitter != results[0].Jitter {
			allJitterEqual = false
		}

		if res.DownloadSpeed > results[bm.DownloadIdx].DownloadSpeed {
			bm.DownloadIdx = i
		}
		if res.UploadSpeed > results[bm.UploadIdx].UploadSpeed {
			bm.UploadIdx = i
		}
		if res.Ping < results[bm.PingIdx].Ping {
			bm.PingIdx = i
		}
		if res.Jitter < results[bm.JitterIdx].Jitter {
			bm.JitterIdx = i
		}
	}

	if allDLEqual {
		bm.DownloadIdx = -1
	}
	if allULEqual {
		bm.UploadIdx = -1
	}
	if allPingEqual {
		bm.PingIdx = -1
	}
	if allJitterEqual {
		bm.JitterIdx = -1
	}

	return bm
}
