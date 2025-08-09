package main

import "fmt"

var (
	// Version holds the current version of the application. It can be overridden at
	// build time using ldflags.
	Version = "dev"
	// BuildDate stores the date the binary was built. It can be set at build time
	// using ldflags.
	BuildDate string
)

// GetVersionInfo returns a formatted version string. If BuildDate is empty, the
// build date is omitted. If Version is empty, it defaults to "dev".
func GetVersionInfo() string {
	v := Version
	if v == "" {
		v = "dev"
	}

	if BuildDate == "" {
		return fmt.Sprintf("lazyspeed version %s", v)
	}
	return fmt.Sprintf("lazyspeed version %s (built: %s)", v, BuildDate)
}
