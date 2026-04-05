package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jkleinne/lazyspeed/internal/timeutil"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/spf13/cobra"
)

const (
	serversNameMaxLen    = 30
	serversSponsorMaxLen = 20
)

type serversFlags struct {
	format    string
	favorites bool
	pin       string
	unpin     string
}

var serversF serversFlags

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
		if serversF.pin != "" && serversF.unpin != "" {
			return fmt.Errorf("--pin and --unpin are mutually exclusive")
		}
		if (serversF.pin != "" || serversF.unpin != "") && serversF.favorites {
			return fmt.Errorf("--pin/--unpin and --favorites are mutually exclusive")
		}

		if serversF.pin != "" {
			return runPinServer(serversF.pin)
		}
		if serversF.unpin != "" {
			return runUnpinServer(serversF.unpin)
		}

		if err := validateFormat(serversF.format); err != nil {
			return err
		}
		runServers()
		return nil
	},
}

// filterFavoriteServers returns only servers whose IDs are in the config's
// favorites list. Returns an error if no favorites are configured. Returns an
// empty slice (without error) if favorites are configured but none match.
func filterFavoriteServers(servers []model.Server, cfg *model.Config) ([]model.Server, error) {
	favIDs := cfg.Servers.FavoriteIDs
	if len(favIDs) == 0 {
		return nil, fmt.Errorf("no favorites configured; use 'lazyspeed servers --pin <id>' to add favorites")
	}
	favSet := cfg.FavoriteIDSet()
	filtered := make([]model.Server, 0, len(favIDs))
	for _, s := range servers {
		if favSet[s.ID] {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

// toServerEntries converts model servers to serialization-friendly entries.
func toServerEntries(servers []model.Server) []serverEntry {
	entries := make([]serverEntry, len(servers))
	for i, s := range servers {
		entries[i] = serverEntry{
			ID:       s.ID,
			Name:     s.Name,
			Sponsor:  s.Sponsor,
			Country:  s.Country,
			Latency:  timeutil.DurationMs(s.Latency),
			Distance: s.Distance,
		}
	}
	return entries
}

func runServers() {
	m := model.NewDefaultModel()

	fmt.Println("Fetching server list...")
	fetchServersOrExit(m)

	servers := m.Servers.List()

	if serversF.favorites {
		var err error
		servers, err = filterFavoriteServers(servers, m.Config)
		if err != nil {
			exitWithError(err.Error())
		}
		if len(servers) == 0 {
			fmt.Println("No favorited servers found in the current server list.")
			return
		}
	}

	if len(servers) == 0 {
		fmt.Println("No servers found.")
		return
	}

	format := resolveFormatString(serversF.format)
	jsonEntries := toServerEntries(servers)

	csvHeader := []string{"id", "name", "sponsor", "country", "latency_ms", "distance_km"}
	csvRows := make([][]string, len(servers))
	for i, s := range servers {
		csvRows[i] = []string{
			s.ID, s.Name, s.Sponsor, s.Country,
			fmt.Sprintf("%.2f", timeutil.DurationMs(s.Latency)),
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
				timeutil.DurationMs(s.Latency), s.Distance)
		}
		_ = tw.Flush()
	})
}

// runPinServer adds a server to favorites by ID, fetching the server list
// first to validate that the ID exists.
func runPinServer(id string) error {
	m := model.NewDefaultModel()

	// Check if already favorited.
	if favoriteIndex(m.Config.Servers.FavoriteIDs, id) >= 0 {
		fmt.Printf("Server %s is already in favorites.\n", id)
		return nil
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

	favs := m.Config.Servers.FavoriteIDs
	idx := favoriteIndex(favs, id)
	if idx < 0 {
		fmt.Printf("Server %s is not in favorites.\n", id)
		return nil
	}
	m.Config.Servers.FavoriteIDs = append(favs[:idx], favs[idx+1:]...)

	if err := model.SaveConfig(m.Config); err != nil {
		exitWithError("saving config: %v", err)
	}

	fmt.Printf("Unpinned server %s.\n", id)
	return nil
}

func init() {
	serversCmd.Flags().StringVar(&serversF.format, "format", "", "Output format: json or csv (default: table)")
	serversCmd.Flags().BoolVar(&serversF.favorites, "favorites", false, "Show only favorited servers")
	serversCmd.Flags().StringVar(&serversF.pin, "pin", "", "Add a server to favorites by ID")
	serversCmd.Flags().StringVar(&serversF.unpin, "unpin", "", "Remove a server from favorites by ID")
	rootCmd.AddCommand(serversCmd)
}
