package ui

import (
	"strings"
)

// bindingContext identifies the TUI screen where a keybinding is active.
type bindingContext string

const (
	contextHome            bindingContext = "Home"
	contextServerSelection bindingContext = "Server Selection"
	contextExport          bindingContext = "Export"
	contextDiagCompact     bindingContext = "Diagnostics"
	contextDiagExpanded    bindingContext = "Diagnostics (Expanded)"
	contextDiagInput       bindingContext = "Diagnostics (Target)"
	contextAnalytics       bindingContext = "Analytics"
	contextComparison      bindingContext = "Comparison"
)

// binding describes a single keybinding shown in help text.
type binding struct {
	Key         string
	Description string
	Context     bindingContext
	ResultOnly  bool // only shown when a test result exists
}

// bindings is the single source of truth for all user-facing keybinding labels.
// The Update handler switch cases remain the authoritative dispatch logic;
// this slice drives only the help/hint renderers.
var bindings = []binding{
	// Home
	{Key: "n", Description: "New Test", Context: contextHome},
	{Key: "d", Description: "Diagnostics", Context: contextHome},
	{Key: "a", Description: "Analytics", Context: contextHome, ResultOnly: true},
	{Key: "e", Description: "Export Result", Context: contextHome, ResultOnly: true},
	{Key: "↑/↓", Description: "Scroll History", Context: contextHome, ResultOnly: true},
	{Key: "h", Description: "Toggle Help", Context: contextHome},
	{Key: "q", Description: "Quit", Context: contextHome},

	// Server Selection
	{Key: "↑/↓", Description: "Navigate", Context: contextServerSelection},
	{Key: "Space", Description: "Toggle Select", Context: contextServerSelection},
	{Key: "Enter", Description: "Select Server", Context: contextServerSelection},
	{Key: "f", Description: "Toggle Favorite", Context: contextServerSelection},
	{Key: "Esc", Description: "Back to Home", Context: contextServerSelection},

	// Export
	{Key: "j", Description: "JSON", Context: contextExport},
	{Key: "c", Description: "CSV", Context: contextExport},
	{Key: "Esc", Description: "Cancel", Context: contextExport},

	// Diagnostics (compact)
	{Key: "Enter", Description: "Expand Trace", Context: contextDiagCompact},
	{Key: "Esc", Description: "Back", Context: contextDiagCompact},
	{Key: "d", Description: "New Diagnostic", Context: contextDiagCompact},
	{Key: "n", Description: "Speed Test", Context: contextDiagCompact},
	{Key: "q", Description: "Quit", Context: contextDiagCompact},

	// Diagnostics (expanded)
	{Key: "↑/↓", Description: "Scroll", Context: contextDiagExpanded},
	{Key: "Esc", Description: "Compact View", Context: contextDiagExpanded},
	{Key: "d", Description: "New Diagnostic", Context: contextDiagExpanded},
	{Key: "q", Description: "Quit", Context: contextDiagExpanded},

	// Diagnostics (target input)
	// No "q" binding — the text input must accept all typed characters including "q".
	// Ctrl+C (handled in main.go) remains available to quit.
	{Key: "Enter", Description: "Run Diagnostic", Context: contextDiagInput},
	{Key: "Esc", Description: "Cancel", Context: contextDiagInput},

	// Analytics
	{Key: "Esc", Description: "Back", Context: contextAnalytics},
	{Key: "n", Description: "New Test", Context: contextAnalytics},
	{Key: "q", Description: "Quit", Context: contextAnalytics},

	// Comparison
	{Key: "Esc", Description: "Back", Context: contextComparison},
	{Key: "n", Description: "New Test", Context: contextComparison},
	{Key: "q", Description: "Quit", Context: contextComparison},
}

// bindingsForContext returns all bindings matching the given context.
func bindingsForContext(ctx bindingContext) []binding {
	var result []binding
	for _, b := range bindings {
		if b.Context == ctx {
			result = append(result, b)
		}
	}
	return result
}

// formatHint builds a " | "-separated hint line from all bindings in a context.
// Keys are rendered in purple (hintKeyStyle) and descriptions in secondary gray (hintDescStyle).
func formatHint(ctx bindingContext) string {
	bindings := bindingsForContext(ctx)
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		key := hintKeyStyle.Render(b.Key)
		desc := hintDescStyle.Render(strings.ToLower(b.Description))
		parts = append(parts, key+hintDescStyle.Render(": ")+desc)
	}
	return strings.Join(parts, hintDescStyle.Render(" | "))
}
