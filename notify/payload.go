package notify

import (
	"time"

	"github.com/jkleinne/lazyspeed/model"
)

const (
	// EventTestComplete indicates the webhook fired because no thresholds were configured.
	EventTestComplete = "test_complete"

	// EventThresholdBreach indicates the webhook fired because a threshold was violated.
	EventThresholdBreach = "threshold_breach"
)

// Payload wraps a speed test result with event metadata for webhook delivery.
type Payload struct {
	Event     string                 `json:"event"`
	Breaches  []Breach               `json:"breaches"`
	Result    *model.SpeedTestResult `json:"result"`
	Version   string                 `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewPayload builds a webhook payload from a speed test result and optional breaches.
// A nil breaches slice produces a "test_complete" event; a non-empty slice produces
// a "threshold_breach" event. The timestamp is caller-supplied (typically time.Now()).
func NewPayload(result *model.SpeedTestResult, breaches []Breach, version string, ts time.Time) Payload {
	event := EventTestComplete
	if len(breaches) > 0 {
		event = EventThresholdBreach
	}
	return Payload{
		Event:     event,
		Breaches:  breaches,
		Result:    result,
		Version:   version,
		Timestamp: ts,
	}
}
