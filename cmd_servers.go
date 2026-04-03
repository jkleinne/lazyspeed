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

var (
	serversFavorites bool
	serversPin       string
	serversUnpin     string
)

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
		if serversPin != "" && serversUnpin != "" {
			return fmt.Errorf("--pin and --unpin are mutually exclusive")
		}
		if (serversPin != "" || serversUnpin != "") && serversFavorites {
			return fmt.Errorf("--pin/--unpin and --favorites are mutually exclusive")
		}

		if serversPin != "" {
			return runPinServer(serversPin)
		}
		if serversUnpin != "" {
			return runUnpinServer(serversUnpin)
		}

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

	if serversFavorites {
		favIDs := m.Config.Servers.FavoriteIDs
		if len(favIDs) == 0 {
			exitWithError("no favorites configured; use 'lazyspeed servers --pin <id>' to add favorites")
		}
		favSet := make(map[string]bool, len(favIDs))
		for _, id := range favIDs {
			favSet[id] = true
		}
		filtered := make([]model.Server, 0, len(favIDs))
		for _, s := range servers {
			if favSet[s.ID] {
				filtered = append(filtered, s)
			}
		}
		servers = filtered
		if len(servers) == 0 {
			fmt.Println("No favorited servers found in the current server list.")
			return
		}
	}

	if len(servers) == 0 {
		fmt.Println("No servers found.")
		return
	}

	format := resolveFormatString(serversFormat)

	jsonEntries := make([]serverEntry, len(servers))
	for i, s := range servers {
		jsonEntries[i] = serverEntry{
			ID:       s.ID,
			Name:     s.Name,
			Sponsor:  s.Sponsor,
			Country:  s.Country,
			Latency:  diag.DurationMs(s.Latency),
			Distance: s.Distance,
		}
	}

	csvHeader := []string{"id", "name", "sponsor", "country", "latency_ms", "distance_km"}
	csvRows := make([][]string, len(servers))
	for i, s := range servers {
		csvRows[i] = []string{
			s.ID, s.Name, s.Sponsor, s.Country,
			fmt.Sprintf("%.2f", diag.DurationMs(s.Latency)),
			fmt.Sprintf("%.1f", s.Distance),
		}
	}

	formatOutput(format, jsonEntries, csvHeader, csvRows, func() {
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
	})
}

// runPinServer adds a server to favorites by ID, fetching the server list
// first to validate that the ID exists.
func runPinServer(id string) error {
	m := model.NewDefaultModel()

	// Check if already favorited.
	for _, fav := range m.Config.Servers.FavoriteIDs {
		if fav == id {
			fmt.Printf("Server %s is already in favorites.\n", id)
			return nil
		}
	}

	// Fetch to validate the server exists.
	fmt.Println("Fetching server list...")
	fetchServersOrExit(m)

	idx, found := m.Servers.FindIndex(id)
	if !found {
		exitWithError("server %s not found", id)
	}

	server := m.Servers.Raw()[idx]
	m.Config.Servers.FavoriteIDs = append(m.Config.Servers.FavoriteIDs, id)

	if err := model.SaveConfig(m.Config); err != nil {
		exitWithError("saving config: %v", err)
	}

	fmt.Printf("Pinned server: %s (%s)\n", server.Name, server.Country)
	return nil
}

// runUnpinServer removes a server from favorites by ID.
// Does not fetch the server list — operates on config only.
func runUnpinServer(id string) error {
	m := model.NewDefaultModel()

	found := false
	favs := m.Config.Servers.FavoriteIDs
	for i, fav := range favs {
		if fav == id {
			m.Config.Servers.FavoriteIDs = append(favs[:i], favs[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("Server %s is not in favorites.\n", id)
		return nil
	}

	if err := model.SaveConfig(m.Config); err != nil {
		exitWithError("saving config: %v", err)
	}

	fmt.Printf("Unpinned server %s.\n", id)
	return nil
}

func init() {
	serversCmd.Flags().StringVar(&serversFormat, "format", "", "Output format: json or csv (default: table)")
	serversCmd.Flags().BoolVar(&serversFavorites, "favorites", false, "Show only favorited servers")
	serversCmd.Flags().StringVar(&serversPin, "pin", "", "Add a server to favorites by ID")
	serversCmd.Flags().StringVar(&serversUnpin, "unpin", "", "Remove a server from favorites by ID")
	rootCmd.AddCommand(serversCmd)
}
