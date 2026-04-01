package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
)

const (
	formatJSON = "json"
	formatCSV  = "csv"
)

// exitWithError prints a formatted error to stderr and exits with code 1.
func exitWithError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// validateFormat returns an error if format is non-empty and not "json" or "csv".
func validateFormat(format string) error {
	if format != "" && format != formatJSON && format != formatCSV {
		return fmt.Errorf("invalid --format %q: must be %q or %q", format, formatJSON, formatCSV)
	}
	return nil
}

// fetchServersOrExit fetches the server list with timeout, exiting on failure.
func fetchServersOrExit(m *model.Model) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Config.FetchTimeoutDuration())
	defer cancel()
	if err := m.FetchServers(ctx); err != nil {
		exitWithError("Error fetching servers: %v", err)
	}
}

// printJSON marshals v as indented JSON to stdout, exiting on failure.
func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		exitWithError("Error serialising JSON: %v", err)
	}
	fmt.Println(string(data))
}

// diagConfig maps model diagnostics settings to a diag.DiagConfig.
func diagConfig(d model.DiagnosticsConfig) *diag.DiagConfig {
	return diag.NewDiagConfig(diag.DiagConfig{
		MaxHops:    d.MaxHops,
		Timeout:    d.Timeout,
		MaxEntries: d.MaxEntries,
		Path:       d.Path,
	})
}

// writeCSVRows writes a header and rows as CSV to stdout, exiting on flush error.
func writeCSVRows(header []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	_ = w.Write(header)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		exitWithError("Error writing CSV: %v", err)
	}
}
