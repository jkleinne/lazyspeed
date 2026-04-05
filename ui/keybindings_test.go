package ui

import (
	"strings"
	"testing"
)

func Test_bindingsForContext(t *testing.T) {
	tests := []struct {
		name    string
		context bindingContext
		minKeys int // minimum expected bindings
	}{
		{"Home", contextHome, 5},
		{"Server Selection", contextServerSelection, 3},
		{"Export", contextExport, 3},
		{"Diagnostics compact", contextDiagCompact, 4},
		{"Diagnostics expanded", contextDiagExpanded, 3},
		{"Diagnostics input", contextDiagInput, 2},
		{"Comparison", contextComparison, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bindings := bindingsForContext(tt.context)
			if len(bindings) < tt.minKeys {
				t.Errorf("bindingsForContext(%q) returned %d bindings, want at least %d",
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

func Test_bindingsForContext_Unknown(t *testing.T) {
	bindings := bindingsForContext("nonexistent")
	if len(bindings) != 0 {
		t.Errorf("expected 0 bindings for unknown context, got %d", len(bindings))
	}
}

func Test_bindingsForContext_Comparison(t *testing.T) {
	bindings := bindingsForContext(contextComparison)
	if len(bindings) != 3 {
		t.Errorf("bindingsForContext(contextComparison) returned %d bindings, want 3", len(bindings))
	}

	keys := make(map[string]bool)
	for _, b := range bindings {
		keys[b.Key] = true
		if b.Context != contextComparison {
			t.Errorf("binding context = %q, want %q", b.Context, contextComparison)
		}
	}

	expectedKeys := []string{"Esc", "n", "q"}
	for _, key := range expectedKeys {
		if !keys[key] {
			t.Errorf("bindingsForContext(contextComparison) missing key %q", key)
		}
	}
}

func Test_bindingsForContext_DiagInput(t *testing.T) {
	bindings := bindingsForContext(contextDiagInput)
	if len(bindings) != 2 {
		t.Errorf("bindingsForContext(contextDiagInput) returned %d bindings, want 2", len(bindings))
	}

	keys := make(map[string]bool)
	for _, b := range bindings {
		keys[b.Key] = true
	}
	for _, key := range []string{"Enter", "Esc"} {
		if !keys[key] {
			t.Errorf("bindingsForContext(contextDiagInput) missing key %q", key)
		}
	}
}

func TestFormatHint(t *testing.T) {
	tests := []struct {
		name     string
		context  bindingContext
		wantKeys []string
		wantDesc []string
	}{
		{
			name:     "DiagCompact contains all bindings",
			context:  contextDiagCompact,
			wantKeys: []string{"Enter", "Esc", "d", "n", "q"},
			wantDesc: []string{"expand trace", "back", "new diagnostic", "speed test", "quit"},
		},
		{
			name:     "DiagExpanded contains all bindings",
			context:  contextDiagExpanded,
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
