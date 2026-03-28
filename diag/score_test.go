package diag

import (
	"testing"
	"time"
)

func TestComputeScore(t *testing.T) {
	tests := []struct {
		name          string
		result        *DiagResult
		expectedGrade string
		minScore      int
		maxScore      int
	}{
		{
			name: "perfect connection",
			result: &DiagResult{
				Hops: []Hop{
					{Number: 1, Latency: 1 * time.Millisecond},
					{Number: 2, Latency: 2 * time.Millisecond},
					{Number: 3, Latency: 5 * time.Millisecond},
				},
				DNS: &DNSResult{Latency: 5 * time.Millisecond},
			},
			expectedGrade: "A",
			minScore:      90,
			maxScore:      100,
		},
		{
			name: "terrible connection",
			result: &DiagResult{
				Hops: []Hop{
					{Number: 1, Timeout: true},
					{Number: 2, Timeout: true},
					{Number: 3, Latency: 250 * time.Millisecond},
				},
				DNS: &DNSResult{Latency: 600 * time.Millisecond},
			},
			expectedGrade: "F",
			minScore:      0,
			maxScore:      24,
		},
		{
			name: "nil DNS redistributes weight",
			result: &DiagResult{
				Hops: []Hop{
					{Number: 1, Latency: 10 * time.Millisecond},
					{Number: 2, Latency: 15 * time.Millisecond},
				},
				DNS: nil,
			},
			expectedGrade: "A",
			minScore:      90,
			maxScore:      100,
		},
		{
			name: "mediocre connection",
			result: &DiagResult{
				Hops: []Hop{
					{Number: 1, Latency: 10 * time.Millisecond},
					{Number: 2, Latency: 50 * time.Millisecond},
					{Number: 3, Timeout: true},
					{Number: 4, Latency: 80 * time.Millisecond},
					{Number: 5, Latency: 100 * time.Millisecond},
				},
				DNS: &DNSResult{Latency: 100 * time.Millisecond},
			},
			expectedGrade: "C",
			minScore:      50,
			maxScore:      74,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ComputeScore(tt.result)
			if score.Score < tt.minScore || score.Score > tt.maxScore {
				t.Errorf("score %d not in range [%d, %d]", score.Score, tt.minScore, tt.maxScore)
			}
			if score.Grade != tt.expectedGrade {
				t.Errorf("grade = %q, want %q", score.Grade, tt.expectedGrade)
			}
			if score.Label == "" {
				t.Error("label should not be empty")
			}
		})
	}
}

func TestGradeBoundaries(t *testing.T) {
	tests := []struct {
		score int
		grade string
	}{
		{100, "A"},
		{90, "A"},
		{89, "B"},
		{75, "B"},
		{74, "C"},
		{50, "C"},
		{49, "D"},
		{25, "D"},
		{24, "F"},
		{0, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.grade, func(t *testing.T) {
			got := gradeFromScore(tt.score)
			if got != tt.grade {
				t.Errorf("gradeFromScore(%d) = %q, want %q", tt.score, got, tt.grade)
			}
		})
	}
}

func TestLabelFromGrade(t *testing.T) {
	grades := []string{"A", "B", "C", "D", "F"}
	for _, g := range grades {
		label := labelFromGrade(g)
		if label == "" {
			t.Errorf("labelFromGrade(%q) returned empty string", g)
		}
	}
}

func TestHopPacketLoss(t *testing.T) {
	tests := []struct {
		name string
		hops []Hop
		want float64
	}{
		{"nil hops", nil, 0},
		{"no timeouts", []Hop{
			{Number: 1, Timeout: false},
			{Number: 2, Timeout: false},
		}, 0},
		{"all timeout", []Hop{
			{Number: 1, Timeout: true},
			{Number: 2, Timeout: true},
		}, 100},
		{"one of three timeout", []Hop{
			{Number: 1, Timeout: false},
			{Number: 2, Timeout: true},
			{Number: 3, Timeout: false},
		}, 100.0 / 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HopPacketLoss(tt.hops)
			if got < tt.want-0.0001 || got > tt.want+0.0001 {
				t.Errorf("HopPacketLoss() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestFinalHopLatencyMs(t *testing.T) {
	tests := []struct {
		name string
		hops []Hop
		want float64
	}{
		{"nil hops", nil, 0},
		{"all timeout", []Hop{
			{Number: 1, Timeout: true},
			{Number: 2, Timeout: true},
		}, 0},
		{"last hop valid", []Hop{
			{Number: 1, Latency: 5 * time.Millisecond},
			{Number: 2, Latency: 10 * time.Millisecond},
		}, 10},
		{"last timeout skips to previous", []Hop{
			{Number: 1, Latency: 5 * time.Millisecond},
			{Number: 2, Latency: 12 * time.Millisecond},
			{Number: 3, Timeout: true},
		}, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FinalHopLatencyMs(tt.hops)
			if got < tt.want-0.0001 || got > tt.want+0.0001 {
				t.Errorf("FinalHopLatencyMs() = %f, want %f", got, tt.want)
			}
		})
	}
}
