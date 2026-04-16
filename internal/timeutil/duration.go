package timeutil

import "time"

const microsecondsPerMs = 1000.0

// DurationMs converts a time.Duration to fractional milliseconds.
func DurationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / microsecondsPerMs
}
