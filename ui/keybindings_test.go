package ui

import "testing"

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
