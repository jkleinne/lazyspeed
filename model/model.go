package model

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

const (
	bytesToMbps             = 125_000
	pingIterations          = 10
	progressInterval        = 200 * time.Millisecond
	pingDelay               = 100 * time.Millisecond
	estimatedTestDurationMs = 15_000
)

const (
	progressInit          = 0.0
	progressFetchNet      = 0.1
	progressServer        = 0.2
	progressPingStart     = 0.3
	progressPingIncrement = 0.02
	progressDownloadStart = 0.5
	progressDownloadSpan  = 0.25
	progressDownloadMax   = 0.7
	progressDownloadDone  = 0.75
	progressUploadStart   = 0.8
	progressUploadSpan    = 0.15
	progressUploadMax     = 0.95
	progressUploadDone    = 0.9 // intentionally < progressUploadMax: progress steps down when phase finishes
	progressComplete      = 1.0
)

type ProgressUpdate struct {
	Progress float64
	Phase    string
}

type SpeedTestResult struct {
	DownloadSpeed float64   `json:"download_speed"`
	UploadSpeed   float64   `json:"upload_speed"`
	Ping          float64   `json:"ping"`
	Jitter        float64   `json:"jitter"`
	ServerName    string    `json:"server_name"`
	ServerSponsor string    `json:"server_sponsor"`
	ServerCountry string    `json:"server_country"`
	Distance      float64   `json:"distance"`
	Timestamp     time.Time `json:"timestamp"`
	UserIP        string    `json:"user_ip"`
	UserISP       string    `json:"user_isp"`
}

// UnmarshalJSON supports reading both the current "server_country" key and
// the legacy "server_loc" key so that existing history files are loaded
// without data loss.
func (r *SpeedTestResult) UnmarshalJSON(data []byte) error {
	// Alias avoids infinite recursion through the custom UnmarshalJSON.
	type Alias SpeedTestResult
	aux := &struct {
		*Alias
		ServerLoc string `json:"server_loc"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	// Prefer the canonical key; fall back to the legacy key.
	if r.ServerCountry == "" && aux.ServerLoc != "" {
		r.ServerCountry = aux.ServerLoc
	}
	return nil
}

// CSVRow returns the result as a string slice suitable for csv.Writer.Write.
func (r *SpeedTestResult) CSVRow() []string {
	return []string{
		r.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		r.ServerName,
		r.ServerCountry,
		fmt.Sprintf("%.2f", r.DownloadSpeed),
		fmt.Sprintf("%.2f", r.UploadSpeed),
		fmt.Sprintf("%.2f", r.Ping),
		fmt.Sprintf("%.2f", r.Jitter),
		r.UserIP,
		r.UserISP,
	}
}

// RunOptions configures a headless speed test run.
type RunOptions struct {
	SkipDownload bool
	SkipUpload   bool
	ProgressFn   func(phase string)
}

// Model holds all application state for the TUI and speed test orchestration.
type Model struct {
	Results                *SpeedTestResult
	TestHistory            []*SpeedTestResult
	Testing                bool
	FetchingServers        bool
	Progress               float64
	CurrentPhase           string
	Error                  error
	Warning                string
	ShowHelp               bool
	Width, Height          int
	PingResults            []float64 // Used for jitter calculation
	ServerList             speedtest.Servers
	Backend                Backend
	Config                 *Config
	SelectingServer        bool
	PendingServerSelection bool
	Cursor                 int
	ServerListOffset       int
	HistoryOffset          int
	User                   *speedtest.User
	// Exporting is true when the TUI is showing the inline export format prompt.
	Exporting bool
	// ExportMessage is set after an export attempt (success path or error) and
	// shown briefly in the TUI view.
	ExportMessage string
}

func NewModel(backend Backend, cfg *Config) *Model {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Model{
		Results:         nil,
		TestHistory:     make([]*SpeedTestResult, 0),
		Testing:         false,
		Progress:        0,
		CurrentPhase:    "",
		ShowHelp:        true,
		SelectingServer: false,
		Cursor:          0,
		Backend:         backend,
		Config:          cfg,
	}
}

func NewDefaultModel() *Model {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}
	return NewModel(&realBackend{}, cfg)
}

// FetchTimeoutDuration returns the configured fetch timeout as a time.Duration.
func (m *Model) FetchTimeoutDuration() time.Duration {
	secs := defaultFetchTimeout
	if m.Config != nil && m.Config.Test.FetchTimeout > 0 {
		secs = m.Config.Test.FetchTimeout
	}
	return time.Duration(secs) * time.Second
}

// TestTimeoutDuration returns the configured test timeout as a time.Duration.
func (m *Model) TestTimeoutDuration() time.Duration {
	secs := defaultTestTimeout
	if m.Config != nil && m.Config.Test.TestTimeout > 0 {
		secs = m.Config.Test.TestTimeout
	}
	return time.Duration(secs) * time.Second
}

// ExportDir returns the configured export directory, falling back to the
// current working directory if none is configured.
func (m *Model) ExportDir() (string, error) {
	if m.Config != nil && m.Config.Export.Directory != "" {
		dir := m.Config.Export.Directory
		if dir == "~" || strings.HasPrefix(dir, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to expand home directory: %v", err)
			}
			if dir == "~" {
				dir = home
			} else {
				dir = filepath.Join(home, dir[2:])
			}
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create export directory: %v", err)
		}
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not determine working directory: %v", err)
	}
	return cwd, nil
}

// pingResult holds the aggregated outcome of a ping measurement.
type pingResult struct {
	Pings   []float64
	AvgPing float64
	Jitter  float64
}

// measurePing runs count ping iterations against server and computes avg/jitter.
// Callers should check len(result.Pings) == 0 to detect total ping failure.
func measurePing(ctx context.Context, backend Backend, server *speedtest.Server, count int) (*pingResult, error) {
	var pings []float64
	var sumPing float64

	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		err := backend.PingTest(server, func(latency time.Duration) {
			ping := float64(latency.Milliseconds())
			pings = append(pings, ping)
			sumPing += ping
		})
		if err != nil {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pingDelay):
		}
	}

	result := &pingResult{Pings: pings}

	if len(pings) > 0 {
		result.AvgPing = sumPing / float64(len(pings))
	}
	if len(pings) > 1 {
		var sum float64
		for i := 1; i < len(pings); i++ {
			sum += math.Abs(pings[i] - pings[i-1])
		}
		result.Jitter = sum / float64(len(pings)-1)
	}

	return result, nil
}

func sendUpdate(progress float64, phase string, updateChan chan<- ProgressUpdate) {
	if updateChan != nil {
		updateChan <- ProgressUpdate{
			Progress: progress,
			Phase:    phase,
		}
	}
}

func callProgressFn(fn func(string), phase string) {
	if fn != nil {
		fn(phase)
	}
}

func (m *Model) historyPath() (string, error) {
	if m.Config != nil && m.Config.History.Path != "" {
		return m.Config.History.Path, nil
	}
	return defaultHistoryPath(), nil
}

func (m *Model) LoadHistory() error {
	historyPath, err := m.historyPath()
	if err != nil {
		return err
	}
	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(historyPath), 0700); err != nil {
		return fmt.Errorf("failed to create history directory: %v", err)
	}

	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history yet, that's fine
		}
		return fmt.Errorf("failed to read history file: %v", err)
	}

	if err := json.Unmarshal(data, &m.TestHistory); err != nil {
		return fmt.Errorf("failed to parse history file: %v", err)
	}

	if len(m.TestHistory) > 0 {
		m.Results = m.TestHistory[len(m.TestHistory)-1]
	}

	return nil
}

func (m *Model) SaveHistory() error {
	historyPath, err := m.historyPath()
	if err != nil {
		return err
	}

	// Ensure the parent directory exists
	if err := os.MkdirAll(filepath.Dir(historyPath), 0700); err != nil {
		return fmt.Errorf("failed to create history directory: %v", err)
	}

	maxEntries := defaultMaxEntries
	if m.Config != nil && m.Config.History.MaxEntries > 0 {
		maxEntries = m.Config.History.MaxEntries
	}
	if len(m.TestHistory) > maxEntries {
		m.TestHistory = m.TestHistory[len(m.TestHistory)-maxEntries:]
	}

	data, err := json.MarshalIndent(m.TestHistory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %v", err)
	}

	// 0600: history contains the user's IP address (PII)
	if err := os.WriteFile(historyPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %v", err)
	}

	return nil
}

func (m *Model) FetchServerList(ctx context.Context) error {
	serverList, err := m.Backend.FetchServers()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("failed to fetch servers: %v", err)
	}
	sort.Slice(serverList, func(i, j int) bool {
		return serverList[i].Latency < serverList[j].Latency
	})
	m.ServerList = serverList
	return nil
}

// transferPhase defines the progress parameters for a download or upload phase.
type transferPhase struct {
	start   float64
	span    float64
	maxProg float64
	label   string
	rateFn  func() float64
}

// monitorTransferProgress runs a ticker goroutine that reports progress during
// a download or upload phase. Close the returned channel to stop monitoring,
// then receive from doneAck to wait for cleanup.
func monitorTransferProgress(
	ctx context.Context,
	phase transferPhase,
	updateChan chan<- ProgressUpdate,
) (done chan struct{}, doneAck chan struct{}) {
	done = make(chan struct{})
	doneAck = make(chan struct{})
	go func() {
		defer close(doneAck)
		start := time.Now()
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(start)
				progress := phase.start + (float64(elapsed.Milliseconds())/estimatedTestDurationMs)*phase.span
				progress = min(progress, phase.maxProg)
				mbps := phase.rateFn() / bytesToMbps
				sendUpdate(progress, fmt.Sprintf("Testing %s: %.2f Mbps...", phase.label, mbps), updateChan)
			}
		}
	}()
	return done, doneAck
}

func (m *Model) PerformSpeedTest(ctx context.Context, server *speedtest.Server, updateChan chan<- ProgressUpdate) error {
	var err error
	m.Testing = true
	m.Progress = 0
	m.Error = nil
	m.Warning = ""
	m.Results = nil
	m.PingResults = make([]float64, 0)

	sendUpdate(progressInit, "Initializing speed test...", updateChan)

	sendUpdate(progressFetchNet, "Fetching network information...", updateChan)
	user, userErr := m.Backend.FetchUserInfo()
	if userErr == nil {
		m.User = user
	} else {
		m.Warning = fmt.Sprintf("could not fetch network info: %v", userErr)
	}

	if ctx.Err() != nil {
		m.Testing = false
		return ctx.Err()
	}

	sendUpdate(progressServer, fmt.Sprintf("Testing with server: %s", server.Name), updateChan)

	pingCount := pingIterations
	if m.Config != nil && m.Config.Test.PingCount > 0 {
		pingCount = m.Config.Test.PingCount
	}

	sendUpdate(progressPingStart, "Measuring ping and jitter...", updateChan)
	pr, err := measurePing(ctx, m.Backend, server, pingCount)
	if err != nil {
		m.Testing = false
		return err
	}
	m.PingResults = pr.Pings

	sendUpdate(progressDownloadStart, "Starting download test...", updateChan)
	dlDone, dlAck := monitorTransferProgress(ctx, transferPhase{
		start:   progressDownloadStart,
		span:    progressDownloadSpan,
		maxProg: progressDownloadMax,
		label:   "download",
		rateFn:  func() float64 { return float64(server.Context.GetEWMADownloadRate()) },
	}, updateChan)
	err = m.Backend.DownloadTest(server)
	close(dlDone)
	<-dlAck
	if ctx.Err() != nil {
		m.Testing = false
		return ctx.Err()
	}
	if err != nil {
		m.Testing = false
		return fmt.Errorf("download test failed: %v", err)
	}
	dlSpeed := float64(server.DLSpeed) / bytesToMbps
	sendUpdate(progressDownloadDone, fmt.Sprintf("Download complete: %.2f Mbps", dlSpeed), updateChan)

	sendUpdate(progressUploadStart, "Starting upload test...", updateChan)
	ulDone, ulAck := monitorTransferProgress(ctx, transferPhase{
		start:   progressUploadStart,
		span:    progressUploadSpan,
		maxProg: progressUploadMax,
		label:   "upload",
		rateFn:  func() float64 { return float64(server.Context.GetEWMAUploadRate()) },
	}, updateChan)
	err = m.Backend.UploadTest(server)
	close(ulDone)
	<-ulAck
	if ctx.Err() != nil {
		m.Testing = false
		return ctx.Err()
	}
	if err != nil {
		m.Testing = false
		return fmt.Errorf("upload test failed: %v", err)
	}
	ulSpeed := float64(server.ULSpeed) / bytesToMbps
	sendUpdate(progressUploadDone, fmt.Sprintf("Upload complete: %.2f Mbps", ulSpeed), updateChan)

	var userIP, userISP string
	if m.User != nil {
		userIP = m.User.IP
		userISP = m.User.Isp
	}

	result := &SpeedTestResult{
		DownloadSpeed: dlSpeed,
		UploadSpeed:   ulSpeed,
		Ping:          pr.AvgPing,
		Jitter:        pr.Jitter,
		ServerName:    server.Name,
		ServerSponsor: server.Sponsor,
		ServerCountry: server.Country,
		Distance:      server.Distance,
		Timestamp:     time.Now(),
		UserIP:        userIP,
		UserISP:       userISP,
	}

	m.Results = result
	m.TestHistory = append(m.TestHistory, result)
	if saveErr := m.SaveHistory(); saveErr != nil {
		m.Warning = fmt.Sprintf("failed to save history: %v", saveErr)
	}

	sendUpdate(progressComplete, "Test completed", updateChan)
	m.Testing = false
	return nil
}

// ExportResult writes result to a file named lazyspeed_<timestamp>.<ext> in dir.
// format must be "json" or "csv". It returns the full path of the written file.
func ExportResult(result *SpeedTestResult, format string, dir string) (string, error) {
	ts := result.Timestamp.Format("20060102_150405")
	switch format {
	case "json":
		path := filepath.Join(dir, fmt.Sprintf("lazyspeed_%s.json", ts))
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to serialise result: %v", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write file: %v", err)
		}
		return path, nil

	case "csv":
		path := filepath.Join(dir, fmt.Sprintf("lazyspeed_%s.csv", ts))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to create file: %v", err)
		}
		defer func() { _ = f.Close() }()
		w := csv.NewWriter(f)
		_ = w.Write([]string{"timestamp", "server", "country", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "ip", "isp"})
		_ = w.Write(result.CSVRow())
		w.Flush()
		if err := w.Error(); err != nil {
			return "", fmt.Errorf("failed to flush CSV writer: %v", err)
		}
		return path, nil

	default:
		return "", fmt.Errorf("unknown format %q: must be \"json\" or \"csv\"", format)
	}
}

func (m *Model) RunHeadless(ctx context.Context, server *speedtest.Server, opts RunOptions) (*SpeedTestResult, error) {
	callProgressFn(opts.ProgressFn, "Fetching network information...")
	user, _ := m.Backend.FetchUserInfo()
	var userIP, userISP string
	if user != nil {
		userIP = user.IP
		userISP = user.Isp
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	headlessPingCount := pingIterations
	if m.Config != nil && m.Config.Test.PingCount > 0 {
		headlessPingCount = m.Config.Test.PingCount
	}

	callProgressFn(opts.ProgressFn, fmt.Sprintf("Measuring ping (0/%d)...", headlessPingCount))
	pr, err := measurePing(ctx, m.Backend, server, headlessPingCount)
	if err != nil {
		return nil, err
	}

	var dlSpeed, ulSpeed float64

	if !opts.SkipDownload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(opts.ProgressFn, "Testing download...")
		if err := m.Backend.DownloadTest(server); err != nil {
			return nil, fmt.Errorf("download test failed: %v", err)
		}
		dlSpeed = float64(server.DLSpeed) / bytesToMbps
		callProgressFn(opts.ProgressFn, fmt.Sprintf("Download: %.2f Mbps", dlSpeed))
	}

	if !opts.SkipUpload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(opts.ProgressFn, "Testing upload...")
		if err := m.Backend.UploadTest(server); err != nil {
			return nil, fmt.Errorf("upload test failed: %v", err)
		}
		ulSpeed = float64(server.ULSpeed) / bytesToMbps
		callProgressFn(opts.ProgressFn, fmt.Sprintf("Upload: %.2f Mbps", ulSpeed))
	}

	return &SpeedTestResult{
		DownloadSpeed: dlSpeed,
		UploadSpeed:   ulSpeed,
		Ping:          pr.AvgPing,
		Jitter:        pr.Jitter,
		ServerName:    server.Name,
		ServerSponsor: server.Sponsor,
		ServerCountry: server.Country,
		Distance:      server.Distance,
		Timestamp:     time.Now(),
		UserIP:        userIP,
		UserISP:       userISP,
	}, nil
}
