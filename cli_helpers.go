package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
)

const (
	formatJSON = "json"
	formatCSV  = "csv"
)

type outputFormat int

const (
	outputTable outputFormat = iota
	outputJSON
	outputCSV
)

// resolveFormat maps bool flags (--json, --csv) to an outputFormat.
func resolveFormat(isJSON, isCSV bool) outputFormat {
	switch {
	case isJSON:
		return outputJSON
	case isCSV:
		return outputCSV
	default:
		return outputTable
	}
}

// resolveFormatString maps a --format string flag to an outputFormat.
func resolveFormatString(format string) outputFormat {
	switch format {
	case formatJSON:
		return outputJSON
	case formatCSV:
		return outputCSV
	default:
		return outputTable
	}
}

// formatOutput dispatches structured output (JSON, CSV, or default table) based on format flags.
// tableRender is called for the human-readable table case.
func formatOutput(format outputFormat, jsonData any, csvHeader []string, csvRows [][]string, tableRender func()) {
	switch format {
	case outputJSON:
		printJSON(jsonData)
	case outputCSV:
		writeCSVRows(csvHeader, csvRows)
	case outputTable:
		tableRender()
	}
}

// exitWithError prints a formatted error to stderr and exits with code 1.
func exitWithError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// validateFormat returns an error if format is non-empty and not "json" or "csv".
func validateFormat(format string) error {
	if format != "" && format != formatJSON && format != formatCSV {
		return fmt.Errorf("invalid --format %q: must be %q or %q", format, formatJSON, formatCSV)
	}
	return nil
}

const clearLine = "\r\033[K"

// interactiveProgressFn returns a progress callback that overwrites the current
// stderr line with the phase string, adding a newline after speed results.
func interactiveProgressFn() func(string) {
	return func(phase string) {
		fmt.Fprintf(os.Stderr, "%s  %s", clearLine, phase)
		// Speed result phases end with phaseResultSuffix; print a newline to preserve them.
		if strings.HasSuffix(phase, phaseResultSuffix) {
			fmt.Fprint(os.Stderr, "\n")
		}
	}
}

// fetchServersOrExit fetches the server list with timeout, exiting on failure.
func fetchServersOrExit(m *model.Model) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Config.FetchTimeoutDuration())
	defer cancel()
	if err := m.FetchServers(ctx); err != nil {
		exitWithError("fetching servers: %v", err)
	}
}

// printJSON marshals v as indented JSON to stdout, exiting on failure.
func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		exitWithError("serialising JSON: %v", err)
	}
	fmt.Println(string(data))
}

// diagConfig maps model diagnostics settings to a diag.Config.
func diagConfig(d model.DiagnosticsConfig) *diag.Config {
	return diag.NewConfig(diag.Config{
		MaxHops:    d.MaxHops,
		Timeout:    d.Timeout,
		MaxEntries: d.MaxEntries,
		Path:       d.Path,
	})
}

// favoriteIndex returns the index of serverID in favs, or -1 if not found.
func favoriteIndex(favs []string, serverID string) int {
	for i, id := range favs {
		if id == serverID {
			return i
		}
	}
	return -1
}

// tailSlice returns the last n elements of s. Returns s unchanged if n <= 0
// or n >= len(s).
func tailSlice[T any](s []T, n int) []T {
	if n > 0 && n < len(s) {
		return s[len(s)-n:]
	}
	return s
}

// writeCSVRows writes a header and rows as CSV to stdout, exiting on flush error.
// header may be nil to skip the header row (used by --watch for incremental output).
func writeCSVRows(header []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	if header != nil {
		_ = w.Write(header)
	}
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		exitWithError("writing CSV: %v", err)
	}
}
