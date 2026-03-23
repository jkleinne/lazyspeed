package main

import (
	"testing"
)

func TestDiagCmdFlagDefaults(t *testing.T) {
	tests := []struct {
		name string
		flag string
		want string
	}{
		{"json default", "json", "false"},
		{"csv default", "csv", "false"},
		{"simple default", "simple", "false"},
		{"history default", "history", "false"},
		{"server default", "server", ""},
		{"last default", "last", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := diagCmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag %q not found", tt.flag)
			}
			if f.DefValue != tt.want {
				t.Errorf("flag %q default = %q, want %q", tt.flag, f.DefValue, tt.want)
			}
		})
	}
}

func TestDiagCmdExists(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "diag [target]" {
			found = true
			break
		}
	}
	if !found {
		t.Error("diag command not registered on rootCmd")
	}
}
