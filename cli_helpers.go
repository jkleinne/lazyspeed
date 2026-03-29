package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jkleinne/lazyspeed/model"
)

// fetchServersOrExit fetches the server list with timeout, exiting on failure.
func fetchServersOrExit(m *model.Model) {
	ctx, cancel := context.WithTimeout(context.Background(), m.FetchTimeoutDuration())
	defer cancel()
	if err := m.FetchServerList(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching servers: %v\n", err)
		os.Exit(1)
	}
}

// printJSON marshals v as indented JSON to stdout, exiting on failure.
func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serialising JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
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
		fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
		os.Exit(1)
	}
}
