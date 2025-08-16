package main

import (
	"fmt"
	"runtime/debug"
	"time"
)

// Returns a formatted version string using embedded build info
func GetVersionInfo() string {
	version := "dev"
	buildDate := ""
	commitHash := ""

	// Read embedded build information
	if info, ok := debug.ReadBuildInfo(); ok {
		// 1. Check for the main module version
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}

		// 2. Extract VCS information (used when building locally with 'go build' in a git repo)
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.time":
				t, err := time.Parse(time.RFC3339, setting.Value)
				if err == nil {
					buildDate = t.Format("2006-01-02")
				}
			case "vcs.revision":
				// Use short commit hash (first 7 characters)
				if len(setting.Value) > 7 {
					commitHash = setting.Value[:7]
				} else {
					commitHash = setting.Value
				}
			}
		}

		// If we are in dev mode but found a commit hash, use it for clarity
		if version == "dev" && commitHash != "" {
			version = fmt.Sprintf("dev (%s)", commitHash)
		}
	}

	infoStr := fmt.Sprintf("lazyspeed version %s", version)

	if buildDate != "" {
		infoStr += fmt.Sprintf(" (built: %s)", buildDate)
	}
	return infoStr
}
