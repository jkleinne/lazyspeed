package diag

import (
	"math"
)

const (
	weightLatency    = 0.35
	weightJitter     = 0.20
	weightPacketLoss = 0.30
	weightDNS        = 0.15

	latencyExcellent = 20.0
	latencyTerrible  = 200.0

	jitterExcellent = 2.0
	jitterTerrible  = 50.0

	packetLossExcellent = 0.0
	packetLossTerrible  = 40.0

	dnsExcellent = 10.0
	dnsTerrible  = 500.0

	gradeA = 90
	gradeB = 75
	gradeC = 50
	gradeD = 25
)

func ComputeScore(result *DiagResult) QualityScore {
	latencyMs := FinalHopLatencyMs(result.Hops)
	if latencyMs == 0 && len(result.Hops) > 0 {
		latencyMs = latencyTerrible
	}
	jitterMs := hopJitter(result.Hops)
	packetLossPct := HopPacketLoss(result.Hops)

	latencyScore := normalizeMetric(latencyMs, latencyExcellent, latencyTerrible)
	jitterScore := normalizeMetric(jitterMs, jitterExcellent, jitterTerrible)
	packetLossScore := normalizeMetric(packetLossPct, packetLossExcellent, packetLossTerrible)

	var composite float64
	if result.DNS != nil {
		dnsMs := DurationMs(result.DNS.Latency)
		dnsScore := normalizeMetric(dnsMs, dnsExcellent, dnsTerrible)
		composite = latencyScore*weightLatency +
			jitterScore*weightJitter +
			packetLossScore*weightPacketLoss +
			dnsScore*weightDNS
	} else {
		total := weightLatency + weightJitter + weightPacketLoss
		composite = latencyScore*(weightLatency/total) +
			jitterScore*(weightJitter/total) +
			packetLossScore*(weightPacketLoss/total)
	}

	score := max(0, min(100, int(math.Round(composite*100))))

	grade := gradeFromScore(score)
	return QualityScore{
		Score: score,
		Grade: grade,
		Label: labelFromGrade(grade),
	}
}

func normalizeMetric(value, excellent, terrible float64) float64 {
	if value <= excellent {
		return 1.0
	}
	if value >= terrible {
		return 0.0
	}
	return 1.0 - (value-excellent)/(terrible-excellent)
}

// FinalHopLatencyMs returns the latency of the last non-timeout hop in milliseconds.
// Returns 0 if all hops timed out or the slice is empty.
func FinalHopLatencyMs(hops []Hop) float64 {
	for i := len(hops) - 1; i >= 0; i-- {
		if !hops[i].Timeout {
			return DurationMs(hops[i].Latency)
		}
	}
	return 0
}

func hopJitter(hops []Hop) float64 {
	var latencies []float64
	for _, h := range hops {
		if !h.Timeout {
			latencies = append(latencies, DurationMs(h.Latency))
		}
	}
	if len(latencies) < 2 {
		return 0
	}
	var mean float64
	for _, latency := range latencies {
		mean += latency
	}
	mean /= float64(len(latencies))

	var variance float64
	for _, latency := range latencies {
		diff := latency - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(latencies)))
}

// HopPacketLoss returns the packet loss percentage across hops.
func HopPacketLoss(hops []Hop) float64 {
	if len(hops) == 0 {
		return 0
	}
	var timeouts int
	for _, h := range hops {
		if h.Timeout {
			timeouts++
		}
	}
	return float64(timeouts) / float64(len(hops)) * 100
}

func gradeFromScore(score int) string {
	switch {
	case score >= gradeA:
		return "A"
	case score >= gradeB:
		return "B"
	case score >= gradeC:
		return "C"
	case score >= gradeD:
		return "D"
	default:
		return "F"
	}
}

func labelFromGrade(grade string) string {
	switch grade {
	case "A":
		return "Great for streaming and video calls"
	case "B":
		return "Good for most activities"
	case "C":
		return "Adequate for browsing, poor for real-time"
	case "D":
		return "Unstable — expect interruptions"
	default:
		return "Severe connectivity issues"
	}
}
