package diag

import (
	"math"

	"github.com/jkleinne/lazyspeed/internal/timeutil"
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

	gradeScoreA = 90
	gradeScoreB = 75
	gradeScoreC = 50
	gradeScoreD = 25

	maxScore          = 100
	percentMultiplier = 100.0
)

// grade is the letter-grade component of a QualityScore.
// Using a named string type keeps grade values self-documenting and lets the
// exhaustive linter enforce complete switch coverage.
type grade string

const (
	gradeA grade = "A"
	gradeB grade = "B"
	gradeC grade = "C"
	gradeD grade = "D"
	gradeF grade = "F"
)

func ComputeScore(result *Result) QualityScore {
	latencyMs := FinalHopLatencyMs(result.Hops)
	if latencyMs == 0 && len(result.Hops) > 0 {
		latencyMs = latencyTerrible
	}
	jitterMs := hopLatencyStdDev(result.Hops)
	packetLossPct := HopPacketLoss(result.Hops)

	latencyScore := normalizeMetric(latencyMs, latencyExcellent, latencyTerrible)
	jitterScore := normalizeMetric(jitterMs, jitterExcellent, jitterTerrible)
	packetLossScore := normalizeMetric(packetLossPct, packetLossExcellent, packetLossTerrible)

	var composite float64
	if result.DNS != nil && result.DNS.Error == "" {
		dnsMs := timeutil.DurationMs(result.DNS.Latency)
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

	score := max(0, min(maxScore, int(math.Round(composite*percentMultiplier))))

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
			return timeutil.DurationMs(hops[i].Latency)
		}
	}
	return 0
}

func hopLatencyStdDev(hops []Hop) float64 {
	var latencies []float64
	for _, h := range hops {
		if !h.Timeout {
			latencies = append(latencies, timeutil.DurationMs(h.Latency))
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
	return float64(timeouts) / float64(len(hops)) * percentMultiplier
}

func gradeFromScore(score int) grade {
	switch {
	case score >= gradeScoreA:
		return gradeA
	case score >= gradeScoreB:
		return gradeB
	case score >= gradeScoreC:
		return gradeC
	case score >= gradeScoreD:
		return gradeD
	default:
		return gradeF
	}
}

func labelFromGrade(g grade) string {
	switch g {
	case gradeA:
		return "Great for streaming and video calls"
	case gradeB:
		return "Good for most activities"
	case gradeC:
		return "Adequate for browsing, poor for real-time"
	case gradeD:
		return "Unstable — expect interruptions"
	case gradeF:
		return "Severe connectivity issues"
	}
	// Unreachable: all grade values are covered above.
	return ""
}
