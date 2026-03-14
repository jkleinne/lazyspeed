package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		width    int
		wantIn   string
		wantZero bool
	}{
		{
			name:     "nil error",
			err:      nil,
			width:    100,
			wantZero: true,
		},
		{
			name:     "actual error",
			err:      errors.New("connection timeout"),
			width:    100,
			wantIn:   "❌ Error: connection timeout",
			wantZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderError(tt.err, tt.width)
			if tt.wantZero {
				if got != "" {
					t.Errorf("RenderError() = %q, want empty string", got)
				}
			} else {
				if got == "" {
					t.Errorf("RenderError() = empty string, want non-empty")
				}
				if !strings.Contains(got, tt.wantIn) {
					t.Errorf("RenderError() = %q, want string containing %q", got, tt.wantIn)
				}
			}
		})
	}
}
