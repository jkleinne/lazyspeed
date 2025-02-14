package main

import "fmt"

// Version information
const (
	Version   = "0.1.2"
	BuildDate = "2024-02-14" // This should ideally be set during build time
)

// GetVersionInfo returns a formatted string containing version and build date
func GetVersionInfo() string {
	return fmt.Sprintf("lazyspeed version %s (built: %s)", Version, BuildDate)
}
