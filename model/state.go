package model

// ModelState represents the primary screen state driven by the model layer.
type ModelState int

const (
	StateIdle             ModelState = iota // Results/history screen (default)
	StateAwaitingServers                    // Fetching servers + will show selection when done
	StateSelectingServer                    // User picking a server
	StateTesting                            // Speed test in progress
	StateExporting                          // Export format prompt shown
)
