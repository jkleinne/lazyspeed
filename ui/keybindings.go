package ui

import (
	"strings"
)

// BindingContext identifies the TUI screen where a keybinding is active.
type BindingContext string

const (
	ContextHome            BindingContext = "Home"
	ContextServerSelection BindingContext = "Server Selection"
	ContextExport          BindingContext = "Export"
	ContextDiagCompact     BindingContext = "Diagnostics"
	ContextDiagExpanded    BindingContext = "Diagnostics (Expanded)"
	ContextAnalytics       BindingContext = "Analytics"
	ContextComparison      BindingContext = "Comparison"
)

// Binding describes a single keybinding shown in help text.
type Binding struct {
	Key         string
	Description string
	Context     BindingContext
	ResultOnly  bool // only shown when a test result exists
}

// bindings is the single source of truth for all user-facing keybinding labels.
// The Update handler switch cases remain the authoritative dispatch logic;
// this slice drives only the help/hint renderers.
var bindings = []Binding{
	// Home
	{Key: "n", Description: "New Test", Context: ContextHome},
	{Key: "d", Description: "Diagnostics", Context: ContextHome},
	{Key: "a", Description: "Analytics", Context: ContextHome, ResultOnly: true},
	{Key: "e", Description: "Export Result", Context: ContextHome, ResultOnly: true},
	{Key: "↑/↓", Description: "Scroll History", Context: ContextHome, ResultOnly: true},
	{Key: "h", Description: "Toggle Help", Context: ContextHome},
	{Key: "q", Description: "Quit", Context: ContextHome},

	// Server Selection
	{Key: "↑/↓", Description: "Navigate", Context: ContextServerSelection},
	{Key: "Space", Description: "Toggle Select", Context: ContextServerSelection},
	{Key: "Enter", Description: "Select Server", Context: ContextServerSelection},
	{Key: "f", Description: "Toggle Favorite", Context: ContextServerSelection},
	{Key: "Esc", Description: "Back to Home", Context: ContextServerSelection},

	// Export
	{Key: "j", Description: "JSON", Context: ContextExport},
	{Key: "c", Description: "CSV", Context: ContextExport},
	{Key: "Esc", Description: "Cancel", Context: ContextExport},

	// Diagnostics (compact)
	{Key: "Enter", Description: "Expand Trace", Context: ContextDiagCompact},
	{Key: "Esc", Description: "Back", Context: ContextDiagCompact},
	{Key: "d", Description: "New Diagnostic", Context: ContextDiagCompact},
	{Key: "n", Description: "Speed Test", Context: ContextDiagCompact},
	{Key: "q", Description: "Quit", Context: ContextDiagCompact},

	// Diagnostics (expanded)
	{Key: "↑/↓", Description: "Scroll", Context: ContextDiagExpanded},
	{Key: "Esc", Description: "Compact View", Context: ContextDiagExpanded},
	{Key: "d", Description: "New Diagnostic", Context: ContextDiagExpanded},
	{Key: "q", Description: "Quit", Context: ContextDiagExpanded},

	// Analytics
	{Key: "Esc", Description: "Back", Context: ContextAnalytics},
	{Key: "n", Description: "New Test", Context: ContextAnalytics},
	{Key: "q", Description: "Quit", Context: ContextAnalytics},

	// Comparison
	{Key: "Esc", Description: "Back", Context: ContextComparison},
	{Key: "n", Description: "New Test", Context: ContextComparison},
	{Key: "q", Description: "Quit", Context: ContextComparison},
}

// BindingsForContext returns all bindings matching the given context.
func BindingsForContext(ctx BindingContext) []Binding {
	var result []Binding
	for _, b := range bindings {
		if b.Context == ctx {
			result = append(result, b)
		}
	}
	return result
}

// formatHint builds a " | "-separated hint line from all bindings in a context.
// Keys are rendered in purple (hintKeyStyle) and descriptions in secondary gray (hintDescStyle).
func formatHint(ctx BindingContext) string {
	bindings := BindingsForContext(ctx)
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		key := hintKeyStyle.Render(b.Key)
		desc := hintDescStyle.Render(strings.ToLower(b.Description))
		parts = append(parts, key+hintDescStyle.Render(": ")+desc)
	}
	return strings.Join(parts, hintDescStyle.Render(" | "))
}
