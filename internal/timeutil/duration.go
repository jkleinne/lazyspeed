package timeutil

import "time"

// DurationMs converts a time.Duration to fractional milliseconds.
func DurationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}
