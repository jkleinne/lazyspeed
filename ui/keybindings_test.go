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
		{"Comparison", ContextComparison, 3},
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

func TestBindingsForContextComparison(t *testing.T) {
	bindings := BindingsForContext(ContextComparison)
	if len(bindings) != 3 {
		t.Errorf("BindingsForContext(ContextComparison) returned %d bindings, want 3", len(bindings))
	}

	keys := make(map[string]bool)
	for _, b := range bindings {
		keys[b.Key] = true
		if b.Context != ContextComparison {
			t.Errorf("binding context = %q, want %q", b.Context, ContextComparison)
		}
	}

	expectedKeys := []string{"Esc", "n", "q"}
	for _, key := range expectedKeys {
		if !keys[key] {
			t.Errorf("BindingsForContext(ContextComparison) missing key %q", key)
		}
	}
}

func TestFormatHint(t *testing.T) {
	tests := []struct {
		name     string
		context  BindingContext
		wantKeys []string
		wantDesc []string
	}{
		{
			name:     "DiagCompact contains all bindings",
			context:  ContextDiagCompact,
			wantKeys: []string{"Enter", "Esc", "d", "n", "q"},
			wantDesc: []string{"expand trace", "back", "new diagnostic", "speed test", "quit"},
		},
		{
			name:     "DiagExpanded contains all bindings",
			context:  ContextDiagExpanded,
			wantKeys: []string{"↑/↓", "Esc", "d", "q"},
			wantDesc: []string{"scroll", "compact view", "new diagnostic", "quit"},
		},
		{
			name:    "Unknown context returns empty string",
			context: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHint(tt.context)

			if len(tt.wantKeys) == 0 {
				if got != "" {
					t.Errorf("formatHint(%q) = %q, want empty string", tt.context, got)
				}
				return
			}

			for _, key := range tt.wantKeys {
				if !strings.Contains(got, key) {
					t.Errorf("formatHint(%q) missing key %q", tt.context, key)
				}
			}
			for _, desc := range tt.wantDesc {
				if !strings.Contains(got, desc) {
					t.Errorf("formatHint(%q) missing description %q", tt.context, desc)
				}
			}
		})
	}
}
