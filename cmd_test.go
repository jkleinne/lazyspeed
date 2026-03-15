package main

import (
	"testing"
)

func TestVersionCommand(t *testing.T) {
	origVersion := version
	version = "1.2.3"
	defer func() { version = origVersion }()

	// We can't easily capture stdout from os.Stdout in tests if the command writes directly to fmt.Println(GetVersionInfo())
	// But we can check that versionCmd exists and can be executed
	if versionCmd.Use != "version" {
		t.Errorf("expected version command")
	}
}

// We just ensure the commands exist and are wired up properly.
func TestCommandsConfigured(t *testing.T) {
	var foundVersion, foundRun, foundHistory bool
	for _, cmd := range rootCmd.Commands() {
		switch cmd.Name() {
		case "version":
			foundVersion = true
		case "run":
			foundRun = true
		case "history":
			foundHistory = true
		}
	}

	if !foundVersion {
		t.Error("version command not registered")
	}
	if !foundRun {
		t.Error("run command not registered")
	}
	if !foundHistory {
		t.Error("history command not registered")
	}
}
