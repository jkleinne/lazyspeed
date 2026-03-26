package ui

// BindingContext identifies the TUI screen where a keybinding is active.
type BindingContext string

const (
	ContextHome            BindingContext = "Home"
	ContextServerSelection BindingContext = "Server Selection"
	ContextExport          BindingContext = "Export"
	ContextDiagCompact     BindingContext = "Diagnostics"
	ContextDiagExpanded    BindingContext = "Diagnostics (Expanded)"
)

// Binding describes a single keybinding shown in help text.
type Binding struct {
	Key         string
	Description string
	Context     BindingContext
}

// Bindings is the single source of truth for all user-facing keybinding labels.
// The Update handler switch cases remain the authoritative dispatch logic;
// this slice drives only the help/hint renderers.
var Bindings = []Binding{
	// Home
	{"n", "New Test", ContextHome},
	{"d", "Diagnostics", ContextHome},
	{"e", "Export Result", ContextHome},
	{"↑/↓, j/k", "Scroll History", ContextHome},
	{"h", "Toggle Help", ContextHome},
	{"q", "Quit", ContextHome},

	// Server Selection
	{"↑/↓, j/k", "Navigate", ContextServerSelection},
	{"Enter", "Select Server", ContextServerSelection},
	{"Esc", "Back to Home", ContextServerSelection},

	// Export
	{"j", "JSON", ContextExport},
	{"c", "CSV", ContextExport},
	{"Esc", "Cancel", ContextExport},

	// Diagnostics (compact)
	{"Enter", "Expand Trace", ContextDiagCompact},
	{"Esc", "Back", ContextDiagCompact},
	{"d", "New Diagnostic", ContextDiagCompact},
	{"n", "Speed Test", ContextDiagCompact},
	{"q", "Quit", ContextDiagCompact},

	// Diagnostics (expanded)
	{"Up/Down", "Scroll", ContextDiagExpanded},
	{"Esc", "Compact View", ContextDiagExpanded},
	{"d", "New Diagnostic", ContextDiagExpanded},
	{"q", "Quit", ContextDiagExpanded},
}

// BindingsForContext returns all bindings matching the given context.
func BindingsForContext(ctx BindingContext) []Binding {
	var result []Binding
	for _, b := range Bindings {
		if b.Context == ctx {
			result = append(result, b)
		}
	}
	return result
}
