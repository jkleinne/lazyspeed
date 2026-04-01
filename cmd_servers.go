package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/spf13/cobra"
)

const (
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
		if err := validateFormat(serversFormat); err != nil {
			return err
		}
		runServers()
		return nil
	},
}

func runServers() {
	m := model.NewDefaultModel()

	fmt.Println("Fetching server list...")
	fetchServersOrExit(m)

	servers := m.Servers.List()
	if len(servers) == 0 {
		fmt.Println("No servers found.")
		return
	}

	switch serversFormat {
	case formatJSON:
		entries := make([]serverEntry, len(servers))
		for i, s := range servers {
			entries[i] = serverEntry{
				ID:       s.ID,
				Name:     s.Name,
				Sponsor:  s.Sponsor,
				Country:  s.Country,
				Latency:  diag.DurationMs(s.Latency),
				Distance: s.Distance,
			}
		}
		printJSON(entries)

	case formatCSV:
		header := []string{"id", "name", "sponsor", "country", "latency_ms", "distance_km"}
		rows := make([][]string, len(servers))
		for i, s := range servers {
			rows[i] = []string{
				s.ID,
				s.Name,
				s.Sponsor,
				s.Country,
				fmt.Sprintf("%.2f", diag.DurationMs(s.Latency)),
				fmt.Sprintf("%.1f", s.Distance),
			}
		}
		writeCSVRows(header, rows)

	default:
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tNAME\tSPONSOR\tCOUNTRY\tLATENCY (ms)\tDISTANCE (km)")
		for _, s := range servers {
			name := ui.Truncate(s.Name, serversNameMaxLen)
			sponsor := ui.Truncate(s.Sponsor, serversSponsorMaxLen)
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.2f\t%.1f\n",
				s.ID, name, sponsor, s.Country,
				diag.DurationMs(s.Latency), s.Distance)
		}
		_ = tw.Flush()
	}
}

func init() {
	serversCmd.Flags().StringVar(&serversFormat, "format", "", "Output format: json or csv (default: table)")
	rootCmd.AddCommand(serversCmd)
}
