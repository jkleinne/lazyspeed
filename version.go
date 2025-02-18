package main

import "fmt"

const (
	Version   = "0.1.2"
	BuildDate = "2025-02-19" // TODO: This should ideally be set during build time
)

func GetVersionInfo() string {
	return fmt.Sprintf("lazyspeed version %s (built: %s)", Version, BuildDate)
}
