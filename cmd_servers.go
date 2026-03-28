package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/spf13/cobra"
)

const (
	serversFormatJSON    = "json"
	serversFormatCSV     = "csv"
	serversNameMaxLen    = 30
	serversSponsorMaxLen = 20
)

var serversFormat string

// serverEntry is a clean serialization struct for JSON/CSV output,
// decoupled from speedtest-go internals.
type serverEntry struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Sponsor  string  `json:"sponsor"`
	Country  string  `json:"country"`
	Latency  float64 `json:"latency_ms"`
	Distance float64 `json:"distance_km"`
}

var serversCmd = &cobra.Command{
	Use:   "servers",
	Short: "List available speed test servers",
	RunE: func(_ *cobra.Command, _ []string) error {
		if serversFormat != "" && serversFormat != serversFormatJSON && serversFormat != serversFormatCSV {
			return fmt.Errorf("invalid --format %q: must be %q or %q", serversFormat, serversFormatJSON, serversFormatCSV)
		}
		runServers()
		return nil
	},
}

func runServers() {
	m := model.NewDefaultModel()

	fmt.Println("Fetching server list...")

	fetchCtx, fetchCancel := context.WithTimeout(context.Background(), m.FetchTimeoutDuration())
	defer fetchCancel()

	if err := m.FetchServerList(fetchCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching servers: %v\n", err)
		os.Exit(1)
	}

	if len(m.ServerList) == 0 {
		fmt.Println("No servers found.")
		return
	}

	switch serversFormat {
	case serversFormatJSON:
		entries := make([]serverEntry, len(m.ServerList))
		for i, s := range m.ServerList {
			entries[i] = serverEntry{
				ID:       s.ID,
				Name:     s.Name,
				Sponsor:  s.Sponsor,
				Country:  s.Country,
				Latency:  s.Latency.Seconds() * 1000,
				Distance: s.Distance,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serialising servers: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))

	case serversFormatCSV:
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"id", "name", "sponsor", "country", "latency_ms", "distance_km"})
		for _, s := range m.ServerList {
			_ = w.Write([]string{
				s.ID,
				s.Name,
				s.Sponsor,
				s.Country,
				fmt.Sprintf("%.2f", s.Latency.Seconds()*1000),
				fmt.Sprintf("%.1f", s.Distance),
			})
		}
		flushCSV(w)

	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tNAME\tSPONSOR\tCOUNTRY\tLATENCY (ms)\tDISTANCE (km)")
		for _, s := range m.ServerList {
			name := ui.Truncate(s.Name, serversNameMaxLen)
			sponsor := ui.Truncate(s.Sponsor, serversSponsorMaxLen)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.2f\t%.1f\n",
				s.ID, name, sponsor, s.Country,
				s.Latency.Seconds()*1000, s.Distance)
		}
		_ = w.Flush()
	}
}

func init() {
	serversCmd.Flags().StringVar(&serversFormat, "format", "", "Output format: json or csv (default: table)")
	rootCmd.AddCommand(serversCmd)
}
