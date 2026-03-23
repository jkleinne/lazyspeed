package diag

import (
	"math"
	"time"
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
)

func ComputeScore(result *DiagResult) QualityScore {
	latencyMs := finalHopLatency(result.Hops)
	jitterMs := hopJitter(result.Hops)
	packetLossPct := hopPacketLoss(result.Hops)

	latencyScore := normalizeMetric(latencyMs, latencyExcellent, latencyTerrible)
	jitterScore := normalizeMetric(jitterMs, jitterExcellent, jitterTerrible)
	packetLossScore := normalizeMetric(packetLossPct, packetLossExcellent, packetLossTerrible)

	var composite float64
	if result.DNS != nil {
		dnsMs := float64(result.DNS.Latency) / float64(time.Millisecond)
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

	score := int(math.Round(composite * 100))
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

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

func finalHopLatency(hops []Hop) float64 {
	for i := len(hops) - 1; i >= 0; i-- {
		if !hops[i].Timeout {
			return float64(hops[i].Latency) / float64(time.Millisecond)
		}
	}
	return latencyTerrible
}

func hopJitter(hops []Hop) float64 {
	var latencies []float64
	for _, h := range hops {
		if !h.Timeout {
			latencies = append(latencies, float64(h.Latency)/float64(time.Millisecond))
		}
	}
	if len(latencies) < 2 {
		return 0
	}
	var mean float64
	for _, l := range latencies {
		mean += l
	}
	mean /= float64(len(latencies))

	var variance float64
	for _, l := range latencies {
		diff := l - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(latencies)))
}

func hopPacketLoss(hops []Hop) float64 {
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
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 50:
		return "C"
	case score >= 25:
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
