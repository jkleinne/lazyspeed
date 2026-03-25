package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/spf13/cobra"
)

var (
	diagJSON    bool
	diagCSV     bool
	diagSimple  bool
	diagHistory bool
	diagServer  string
	diagLast    int
)

var diagCmd = &cobra.Command{
	Use:   "diag [target]",
	Short: "Run network diagnostics against a target",
	Long: `Run network diagnostics (traceroute + DNS) against a target host or IP.
If no target is given and --server is not set, the closest speedtest server is used.
Use --history to view past diagnostics.`,
	RunE: func(_ *cobra.Command, args []string) error {
		if diagLast < 0 {
			return fmt.Errorf("--last must be >= 0, got %d", diagLast)
		}
		if diagHistory {
			runDiagHistory()
			return nil
		}
		runDiag(args)
		return nil
	},
}

func init() {
	diagCmd.Flags().BoolVar(&diagJSON, "json", false, "Output result as JSON")
	diagCmd.Flags().BoolVar(&diagCSV, "csv", false, "Output result as CSV")
	diagCmd.Flags().BoolVar(&diagSimple, "simple", false, "Minimal one-line output")
	diagCmd.Flags().BoolVar(&diagHistory, "history", false, "Show past diagnostics instead of running a new one")
	diagCmd.Flags().StringVar(&diagServer, "server", "", "Use a specific speedtest server ID as the target")
	diagCmd.Flags().IntVar(&diagLast, "last", 0, "Limit history output to the last N results (0 = all)")

	rootCmd.AddCommand(diagCmd)
}

func stripPort(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}

// diagConfigFromModel maps model.Config.Diagnostics to diag.DiagConfig.
func diagConfigFromModel(m *model.Model) *diag.DiagConfig {
	cfg := diag.DefaultDiagConfig()
	if m.Config != nil {
		if m.Config.Diagnostics.MaxHops > 0 {
			cfg.MaxHops = m.Config.Diagnostics.MaxHops
		}
		if m.Config.Diagnostics.Timeout > 0 {
			cfg.Timeout = m.Config.Diagnostics.Timeout
		}
		if m.Config.Diagnostics.MaxEntries > 0 {
			cfg.MaxEntries = m.Config.Diagnostics.MaxEntries
		}
		if m.Config.Diagnostics.Path != "" {
			cfg.Path = m.Config.Diagnostics.Path
		}
	}
	return cfg
}

func runDiag(args []string) {
	m := model.NewDefaultModel()
	cfg := diagConfigFromModel(m)

	var target string

	if len(args) > 0 {
		target = args[0]
	} else if diagServer != "" {
		// Use the specified server ID — fetch server list to resolve host
		if !diagJSON && !diagCSV && !diagSimple {
			fmt.Fprintln(os.Stderr, "Fetching server list...")
		}
		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), m.FetchTimeoutDuration())
		defer fetchCancel()
		if err := m.FetchServerList(fetchCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching servers: %v\n", err)
			os.Exit(1)
		}
		found := false
		for _, s := range m.ServerList {
			if s.ID == diagServer {
				target = stripPort(s.Host)
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "Error: server %s not found\n", diagServer)
			os.Exit(1)
		}
	} else {
		// Auto-select closest speedtest server
		if !diagJSON && !diagCSV && !diagSimple {
			fmt.Fprintln(os.Stderr, "Fetching server list...")
		}
		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), m.FetchTimeoutDuration())
		defer fetchCancel()
		if err := m.FetchServerList(fetchCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching servers: %v\n", err)
			os.Exit(1)
		}
		if len(m.ServerList) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no servers found\n")
			os.Exit(1)
		}
		target = stripPort(m.ServerList[0].Host)
		if !diagJSON && !diagCSV && !diagSimple {
			fmt.Fprintf(os.Stderr, "Selected server: %s (%s)\n", m.ServerList[0].Name, m.ServerList[0].Country)
		}
	}

	if !diagJSON && !diagCSV && !diagSimple {
		fmt.Fprintf(os.Stderr, "Running diagnostics against %s...\n", target)
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	diagCtx, diagCancel := context.WithTimeout(context.Background(), timeout)
	defer diagCancel()

	backend := &diag.RealDiagBackend{}
	result, err := diag.Run(diagCtx, backend, target, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running diagnostics: %v\n", err)
		os.Exit(1)
	}

	// Persist to history
	histPath := cfg.Path
	history, _ := diag.LoadHistory(histPath)
	history = append(history, result)
	_ = diag.SaveHistory(histPath, history, cfg.MaxEntries)

	// Output
	if diagJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serialising result: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	if diagCSV {
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{
			"timestamp", "target", "method", "score", "grade",
			"dns_ms", "dns_cached", "hops", "packet_loss_pct", "final_hop_latency_ms",
		})
		_ = w.Write(diagCSVRow(result))
		flushCSV(w)
		return
	}

	if diagSimple {
		fmt.Println(diagSimpleLine(result))
		return
	}

	// Default: human-readable with full hop table
	fmt.Println(diagDefaultOutput(result))
}

func runDiagHistory() {
	m := model.NewDefaultModel()
	cfg := diagConfigFromModel(m)

	history, err := diag.LoadHistory(cfg.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading diagnostics history: %v\n", err)
		os.Exit(1)
	}

	if len(history) == 0 {
		fmt.Println("No diagnostics history found.")
		return
	}

	// Apply --last slice
	entries := history
	if diagLast > 0 && diagLast < len(entries) {
		entries = entries[len(entries)-diagLast:]
	}

	if diagJSON {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serialising history: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	if diagCSV {
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{
			"timestamp", "target", "method", "score", "grade",
			"dns_ms", "dns_cached", "hops", "packet_loss_pct", "final_hop_latency_ms",
		})
		for _, r := range entries {
			_ = w.Write(diagCSVRow(r))
		}
		flushCSV(w)
		return
	}

	// Default: table view
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tTARGET\tSCORE\tGRADE\tHOPS\tDNS (ms)")
	for _, r := range entries {
		dateStr := r.Timestamp.Format("2006-01-02 15:04")
		targetStr := r.Target
		if len(targetStr) > 30 {
			targetStr = targetStr[:27] + "..."
		}
		dnsMs := "-"
		if r.DNS != nil {
			dnsMs = fmt.Sprintf("%.1f", float64(r.DNS.Latency.Microseconds())/1000.0)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%d\t%s\n",
			dateStr, targetStr, r.Quality.Score, r.Quality.Grade, len(r.Hops), dnsMs)
	}
	_ = tw.Flush()
}

// diagCSVRow returns a CSV row for a DiagResult.
func diagCSVRow(r *diag.DiagResult) []string {
	dnsMs := ""
	dnsCached := ""
	if r.DNS != nil {
		dnsMs = fmt.Sprintf("%.3f", float64(r.DNS.Latency.Microseconds())/1000.0)
		dnsCached = fmt.Sprintf("%v", r.DNS.Cached)
	}

	packetLossPct := diagPacketLossPct(r.Hops)
	finalLatencyMs := diagFinalHopLatencyMs(r.Hops)

	return []string{
		r.Timestamp.Format(time.RFC3339),
		r.Target,
		r.Method,
		fmt.Sprintf("%d", r.Quality.Score),
		r.Quality.Grade,
		dnsMs,
		dnsCached,
		fmt.Sprintf("%d", len(r.Hops)),
		fmt.Sprintf("%.2f", packetLossPct),
		fmt.Sprintf("%.3f", finalLatencyMs),
	}
}

// diagSimpleLine returns a one-line summary of a DiagResult.
func diagSimpleLine(r *diag.DiagResult) string {
	dnsStr := "-"
	if r.DNS != nil {
		dnsStr = fmt.Sprintf("%.0fms", float64(r.DNS.Latency.Microseconds())/1000.0)
	}
	return fmt.Sprintf("Score: %d/%s | DNS: %s | Hops: %d",
		r.Quality.Score, r.Quality.Grade, dnsStr, len(r.Hops))
}

// diagDefaultOutput formats a DiagResult as a human-readable report with a hop table.
func diagDefaultOutput(r *diag.DiagResult) string {
	var out string
	out += fmt.Sprintf("\nDiagnostics: %s\n", r.Target)
	out += fmt.Sprintf("Timestamp:   %s\n", r.Timestamp.Format("2006-01-02 15:04:05"))
	out += fmt.Sprintf("Method:      %s\n", r.Method)
	out += fmt.Sprintf("Score:       %d / %s  (%s)\n",
		r.Quality.Score, r.Quality.Grade, r.Quality.Label)

	if r.DNS != nil {
		cachedStr := "no"
		if r.DNS.Cached {
			cachedStr = "yes"
		}
		out += fmt.Sprintf("DNS:         %.1f ms (cached: %s)\n",
			float64(r.DNS.Latency.Microseconds())/1000.0, cachedStr)
	}

	out += fmt.Sprintf("\nHops (%d):\n", len(r.Hops))

	var sb string
	for _, h := range r.Hops {
		if h.Timeout {
			sb += fmt.Sprintf("  %2d  *\n", h.Number)
		} else {
			latencyMs := float64(h.Latency.Microseconds()) / 1000.0
			host := h.Host
			if host == "" || host == h.IP {
				host = h.IP
			} else {
				host = fmt.Sprintf("%s (%s)", h.Host, h.IP)
			}
			sb += fmt.Sprintf("  %2d  %-50s  %.3f ms\n", h.Number, host, latencyMs)
		}
	}
	out += sb
	return out
}

// diagPacketLossPct computes the packet loss percentage across hops.
func diagPacketLossPct(hops []diag.Hop) float64 {
	if len(hops) == 0 {
		return 0
	}
	var timeouts int
	for _, h := range hops {
		if h.Timeout {
			timeouts++
		}
	}
	return float64(timeouts) / float64(len(hops)) * 100
}

// diagFinalHopLatencyMs returns the latency of the last non-timeout hop in ms.
func diagFinalHopLatencyMs(hops []diag.Hop) float64 {
	for i := len(hops) - 1; i >= 0; i-- {
		if !hops[i].Timeout {
			return float64(hops[i].Latency.Microseconds()) / 1000.0
		}
	}
	return 0
}
