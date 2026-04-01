package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
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

var diagCSVHeader = []string{
	"timestamp", "target", "method", "score", "grade",
	"dns_ms", "dns_cached", "hops", "packet_loss_pct", "final_hop_latency_ms",
}

func diagIsInteractive() bool {
	return !diagJSON && !diagCSV && !diagSimple
}

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

// fetchDiagServers fetches the server list, printing status if interactive.
func fetchDiagServers(m *model.Model) {
	if diagIsInteractive() {
		fmt.Fprintln(os.Stderr, "Fetching server list...")
	}
	fetchServersOrExit(m)
}

// resolveDiagTarget determines the diagnostics target from CLI args, --server flag,
// or the closest speedtest server. Calls exitWithError on fatal errors.
func resolveDiagTarget(m *model.Model, args []string) string {
	if len(args) > 0 {
		return args[0]
	}

	fetchDiagServers(m)

	if diagServer != "" {
		idx, found := m.Servers.FindIndex(diagServer)
		if !found {
			exitWithError("server %s not found", diagServer)
		}
		return stripPort(m.Servers.Raw()[idx].Host)
	}

	if m.Servers.Len() == 0 {
		exitWithError("no servers found")
	}
	if diagIsInteractive() {
		fmt.Fprintf(os.Stderr, "Selected server: %s (%s)\n", m.Servers.Raw()[0].Name, m.Servers.Raw()[0].Country)
	}
	return stripPort(m.Servers.Raw()[0].Host)
}

func runDiag(args []string) {
	m := model.NewDefaultModel()
	cfg := diagConfig(m.Config.Diagnostics)
	target := resolveDiagTarget(m, args)

	if diagIsInteractive() {
		fmt.Fprintf(os.Stderr, "Running diagnostics against %s...\n", target)
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	diagCtx, diagCancel := context.WithTimeout(context.Background(), timeout)
	defer diagCancel()

	backend := &diag.RealDiagBackend{}
	result, err := diag.Run(diagCtx, backend, target, cfg)
	if err != nil {
		exitWithError("running diagnostics: %v", err)
	}

	if cfg.MaxEntries > 0 {
		if err := diag.AppendHistory(cfg.Path, result, cfg.MaxEntries); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to persist diagnostics history: %v\n", err)
		}
	}

	// Output
	if diagJSON {
		printJSON(result)
		return
	}

	if diagCSV {
		writeCSVRows(diagCSVHeader, [][]string{diagCSVRow(result)})
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
	cfg := diagConfig(m.Config.Diagnostics)

	history, err := diag.LoadHistory(cfg.Path)
	if err != nil {
		exitWithError("loading diagnostics history: %v", err)
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
		printJSON(entries)
		return
	}

	if diagCSV {
		rows := make([][]string, len(entries))
		for i, r := range entries {
			rows[i] = diagCSVRow(r)
		}
		writeCSVRows(diagCSVHeader, rows)
		return
	}

	// Default: table view
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATE\tTARGET\tSCORE\tGRADE\tHOPS\tDNS (ms)")
	for _, r := range entries {
		dateStr := r.Timestamp.Format("2006-01-02 15:04")
		targetStr := ui.Truncate(r.Target, diagTargetMaxLen)
		dnsMs := "-"
		if r.DNS != nil {
			dnsMs = fmt.Sprintf("%.1f", diag.DurationMs(r.DNS.Latency))
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%d\t%s\n",
			dateStr, targetStr, r.Quality.Score, r.Quality.Grade, len(r.Hops), dnsMs)
	}
	_ = tw.Flush()
}

const diagTargetMaxLen = 30

// diagCSVRow returns a CSV row for a DiagResult.
func diagCSVRow(r *diag.DiagResult) []string {
	dnsMs := ""
	dnsCached := ""
	if r.DNS != nil {
		dnsMs = fmt.Sprintf("%.3f", diag.DurationMs(r.DNS.Latency))
		dnsCached = strconv.FormatBool(r.DNS.Cached)
	}

	packetLossPct := diag.HopPacketLoss(r.Hops)
	finalLatencyMs := diag.FinalHopLatencyMs(r.Hops)

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
		dnsStr = fmt.Sprintf("%.0fms", diag.DurationMs(r.DNS.Latency))
	}
	return fmt.Sprintf("Score: %d/%s | DNS: %s | Hops: %d",
		r.Quality.Score, r.Quality.Grade, dnsStr, len(r.Hops))
}

// diagDefaultOutput formats a DiagResult as a human-readable report with a hop table.
func diagDefaultOutput(r *diag.DiagResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\nDiagnostics: %s\n", r.Target)
	fmt.Fprintf(&b, "Timestamp:   %s\n", r.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Method:      %s\n", r.Method)
	fmt.Fprintf(&b, "Score:       %d / %s  (%s)\n",
		r.Quality.Score, r.Quality.Grade, r.Quality.Label)

	if r.DNS != nil {
		cachedStr := "cold"
		if r.DNS.Cached {
			cachedStr = "cached"
		}
		fmt.Fprintf(&b, "DNS:         %.1f ms (cached: %s)\n",
			diag.DurationMs(r.DNS.Latency), cachedStr)
	}

	fmt.Fprintf(&b, "\nHops (%d):\n", len(r.Hops))

	for _, h := range r.Hops {
		if h.Timeout {
			fmt.Fprintf(&b, "  %2d  *\n", h.Number)
		} else {
			latencyMs := diag.DurationMs(h.Latency)
			host := h.Host
			if host == "" || host == h.IP {
				host = h.IP
			} else {
				host = fmt.Sprintf("%s (%s)", h.Host, h.IP)
			}
			fmt.Fprintf(&b, "  %2d  %-50s  %.3f ms\n", h.Number, host, latencyMs)
		}
	}
	return b.String()
}
