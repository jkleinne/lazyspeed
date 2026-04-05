package main

import (
	"testing"
)

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		name   string
		isJSON bool
		isCSV  bool
		want   outputFormat
	}{
		{"neither flag", false, false, outputTable},
		{"json flag", true, false, outputJSON},
		{"csv flag", false, true, outputCSV},
		{"both flags prefers json", true, true, outputJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFormat(tt.isJSON, tt.isCSV)
			if got != tt.want {
				t.Errorf("resolveFormat(%v, %v) = %d, want %d", tt.isJSON, tt.isCSV, got, tt.want)
			}
		})
	}
}

func TestResolveFormatString(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   outputFormat
	}{
		{"json string", "json", outputJSON},
		{"csv string", "csv", outputCSV},
		{"empty string", "", outputTable},
		{"unknown string", "xml", outputTable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFormatString(tt.format)
			if got != tt.want {
				t.Errorf("resolveFormatString(%q) = %d, want %d", tt.format, got, tt.want)
			}
		})
	}
}

func TestTailSlice(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		n     int
		want  int // expected length
		first int // expected first element (if non-empty)
	}{
		{"last 2 of 5", []int{1, 2, 3, 4, 5}, 2, 2, 4},
		{"n equals len", []int{1, 2, 3}, 3, 3, 1},
		{"n exceeds len", []int{1, 2}, 5, 2, 1},
		{"n is zero", []int{1, 2, 3}, 0, 3, 1},
		{"n is negative", []int{1, 2, 3}, -1, 3, 1},
		{"empty slice", []int{}, 3, 0, 0},
		{"last 1", []int{10, 20, 30}, 1, 1, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tailSlice(tt.slice, tt.n)
			if len(got) != tt.want {
				t.Errorf("tailSlice(len=%d, %d) returned len %d, want %d", len(tt.slice), tt.n, len(got), tt.want)
			}
			if len(got) > 0 && got[0] != tt.first {
				t.Errorf("tailSlice first element = %d, want %d", got[0], tt.first)
			}
		})
	}
}
