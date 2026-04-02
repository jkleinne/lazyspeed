package model

import (
	"encoding"
	"testing"
)

func TestTrendDirectionMarshalText(t *testing.T) {
	tests := []struct {
		name string
		td   TrendDirection
		want string
	}{
		{"stable", TrendStable, "stable"},
		{"up", TrendUp, "up"},
		{"down", TrendDown, "down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.td.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("MarshalText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrendDirectionUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    TrendDirection
		wantErr bool
	}{
		{"stable", "stable", TrendStable, false},
		{"up", "up", TrendUp, false},
		{"down", "down", TrendDown, false},
		{"invalid", "sideways", TrendStable, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var td TrendDirection
			err := td.UnmarshalText([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && td != tt.want {
				t.Errorf("UnmarshalText() = %v, want %v", td, tt.want)
			}
		})
	}
}

// Verify interface compliance at compile time.
var (
	_ encoding.TextMarshaler   = TrendDirection(0)
	_ encoding.TextUnmarshaler = (*TrendDirection)(nil)
)
