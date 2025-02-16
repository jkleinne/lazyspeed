package main

import "fmt"

const (
	Version   = "0.1.1"
	BuildDate = "2025-02-16" // TODO: This should ideally be set during build time
)

func GetVersionInfo() string {
	return fmt.Sprintf("lazyspeed version %s (built: %s)", Version, BuildDate)
}
