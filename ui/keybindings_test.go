package ui

import (
	"strings"
	"testing"
)

func TestBindingsForContext(t *testing.T) {
	tests := []struct {
		name    string
		context BindingContext
		minKeys int // minimum expected bindings
	}{
		{"Home", ContextHome, 5},
		{"Server Selection", ContextServerSelection, 3},
		{"Export", ContextExport, 3},
		{"Diagnostics compact", ContextDiagCompact, 4},
		{"Diagnostics expanded", ContextDiagExpanded, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := BindingsForContext(tt.context)
			if len(bindings) < tt.minKeys {
				t.Errorf("BindingsForContext(%q) returned %d bindings, want at least %d",
					tt.context, len(bindings), tt.minKeys)
			}
			for _, b := range bindings {
				if b.Key == "" || b.Description == "" {
					t.Errorf("binding has empty Key or Description: %+v", b)
				}
				if b.Context != tt.context {
					t.Errorf("binding context = %q, want %q", b.Context, tt.context)
				}
			}
		})
	}
}

func TestBindingsForContextUnknown(t *testing.T) {
	bindings := BindingsForContext("nonexistent")
	if len(bindings) != 0 {
		t.Errorf("expected 0 bindings for unknown context, got %d", len(bindings))
	}
}

func TestFormatHint(t *testing.T) {
	tests := []struct {
		name     string
		context  BindingContext
		wantSep  string
		wantSubs []string // substrings that must appear (lowercased descriptions)
	}{
		{
			name:     "DiagCompact contains all bindings pipe-separated",
			context:  ContextDiagCompact,
			wantSep:  " | ",
			wantSubs: []string{"Enter: expand trace", "Esc: back", "d: new diagnostic", "n: speed test", "q: quit"},
		},
		{
			name:     "DiagExpanded contains all bindings pipe-separated",
			context:  ContextDiagExpanded,
			wantSep:  " | ",
			wantSubs: []string{"Up/Down: scroll", "Esc: compact view", "d: new diagnostic", "q: quit"},
		},
		{
			name:    "Unknown context returns empty string",
			context: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHint(tt.context)

			if len(tt.wantSubs) == 0 {
				if got != "" {
					t.Errorf("formatHint(%q) = %q, want empty string", tt.context, got)
				}
				return
			}

			for _, sub := range tt.wantSubs {
				if !strings.Contains(got, sub) {
					t.Errorf("formatHint(%q) = %q, missing %q", tt.context, got, sub)
				}
			}

			// Verify pipe separator count matches binding count - 1
			bindings := BindingsForContext(tt.context)
			expectedPipes := len(bindings) - 1
			actualPipes := strings.Count(got, tt.wantSep)
			if actualPipes != expectedPipes {
				t.Errorf("formatHint(%q) has %d pipes, want %d", tt.context, actualPipes, expectedPipes)
			}
		})
	}
}
