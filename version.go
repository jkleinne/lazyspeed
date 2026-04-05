package main

import (
	"fmt"
	"runtime/debug"
	"time"
)

// Set via -ldflags by GoReleaser at build time.
// When building locally with `go build`, these remain empty
// and the function falls back to debug.ReadBuildInfo().
var (
	version = ""
	date    = ""
)

const shortHashLen = 7

// GetVersionInfo returns a formatted version string using embedded build info.
// It prefers values injected at build time via GoReleaser ldflags and falls
// back to debug.ReadBuildInfo() for local `go build` invocations.
func GetVersionInfo() string {
	// Prefer values injected by GoReleaser via ldflags
	if version != "" {
		versionLine := fmt.Sprintf("lazyspeed version %s", version)
		if date != "" {
			versionLine += fmt.Sprintf(" (built: %s)", date)
		}
		return versionLine
	}

	// Fallback: read embedded build information (local `go build`)
	fallbackVersion := "dev"
	buildDate := ""
	commitHash := ""

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			fallbackVersion = info.Main.Version
		}

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.time":
				t, err := time.Parse(time.RFC3339, setting.Value)
				if err == nil {
					buildDate = t.Format("2006-01-02")
				}
			case "vcs.revision":
				if len(setting.Value) > shortHashLen {
					commitHash = setting.Value[:shortHashLen]
				} else {
					commitHash = setting.Value
				}
			}
		}

		if fallbackVersion == "dev" && commitHash != "" {
			fallbackVersion = fmt.Sprintf("dev (%s)", commitHash)
		}
	}

	versionLine := fmt.Sprintf("lazyspeed version %s", fallbackVersion)

	if buildDate != "" {
		versionLine += fmt.Sprintf(" (built: %s)", buildDate)
	}
	return versionLine
}
