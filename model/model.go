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

func (m *Model) PerformSpeedTest(ctx context.Context, server *speedtest.Server, updateChan chan<- ProgressUpdate) error {
	var err error
	m.Testing = true
	m.Progress = 0
	m.Error = nil
	m.Warning = ""
	m.Results = nil
	m.PingResults = make([]float64, 0)

	sendUpdate(0.0, "Initializing speed test...", updateChan)

	sendUpdate(0.1, "Fetching network information...", updateChan)
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

	sendUpdate(0.2, fmt.Sprintf("Testing with server: %s", server.Name), updateChan)

	pingCount := pingIterations
	if m.Config != nil && m.Config.Test.PingCount > 0 {
		pingCount = m.Config.Test.PingCount
	}

	sendUpdate(0.3, "Measuring ping and jitter...", updateChan)
	var sumPing float64
	for i := 0; i < pingCount; i++ {
		if ctx.Err() != nil {
			m.Testing = false
			return ctx.Err()
		}
		err := m.Backend.PingTest(server, func(latency time.Duration) {
			ping := float64(latency.Milliseconds())
			m.PingResults = append(m.PingResults, ping)
			sumPing += ping
			if len(m.PingResults) > 1 {
				// Calculate current jitter for display
				lastIdx := len(m.PingResults) - 1
				currentJitter := math.Abs(m.PingResults[lastIdx] - m.PingResults[lastIdx-1])
				sendUpdate(0.3+float64(i+1)*0.02,
					fmt.Sprintf("Ping: %.1f ms, Jitter: %.1f ms (%d/%d)",
						ping, currentJitter, i+1, pingCount), updateChan)
			} else {
				sendUpdate(0.3+float64(i+1)*0.02,
					fmt.Sprintf("Ping: %.1f ms (%d/%d)", ping, i+1, pingCount), updateChan)
			}
		})
		if err != nil {
			continue
		}
		select {
		case <-ctx.Done():
			m.Testing = false
			return ctx.Err()
		case <-time.After(pingDelay): // Small delay between pings
		}
	}

	var jitter float64
	if len(m.PingResults) > 1 {
		var sum float64
		for i := 1; i < len(m.PingResults); i++ {
			sum += math.Abs(m.PingResults[i] - m.PingResults[i-1])
		}
		jitter = sum / float64(len(m.PingResults)-1)
	}

	if ctx.Err() != nil {
		m.Testing = false
		return ctx.Err()
	}

	sendUpdate(0.5, "Starting download test...", updateChan)
	done := make(chan struct{})
	doneAck := make(chan struct{})
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
				progress := 0.5 + (float64(elapsed.Milliseconds())/estimatedTestDurationMs)*0.25
				if progress > 0.7 {
					progress = 0.7
				}

				// Fetch the Exponential Weighted Moving Average (EWMA) download rate in bytes/sec
				rate := server.Context.GetEWMADownloadRate()
				mbps := float64(rate) / bytesToMbps

				sendUpdate(progress, fmt.Sprintf("Testing download: %.2f Mbps...", mbps), updateChan)
			}
		}
	}()
	err = m.Backend.DownloadTest(server)
	close(done)
	<-doneAck
	if ctx.Err() != nil {
		m.Testing = false
		return ctx.Err()
	}
	if err != nil {
		m.Testing = false
		return fmt.Errorf("download test failed: %v", err)
	}
	dlSpeed := float64(server.DLSpeed) / bytesToMbps
	sendUpdate(0.75, fmt.Sprintf("Download complete: %.2f Mbps", dlSpeed), updateChan)

	sendUpdate(0.8, "Starting upload test...", updateChan)
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
				progress := 0.8 + (float64(elapsed.Milliseconds())/estimatedTestDurationMs)*0.15
				if progress > 0.95 {
					progress = 0.95
				}

				rate := server.Context.GetEWMAUploadRate()
				mbps := float64(rate) / bytesToMbps

				sendUpdate(progress, fmt.Sprintf("Testing upload: %.2f Mbps...", mbps), updateChan)
			}
		}
	}()
	err = m.Backend.UploadTest(server)
	close(done)
	<-doneAck
	if ctx.Err() != nil {
		m.Testing = false
		return ctx.Err()
	}
	if err != nil {
		m.Testing = false
		return fmt.Errorf("upload test failed: %v", err)
	}
	ulSpeed := float64(server.ULSpeed) / bytesToMbps
	sendUpdate(0.9, fmt.Sprintf("Upload complete: %.2f Mbps", ulSpeed), updateChan)

	var userIP, userISP string
	if m.User != nil {
		userIP = m.User.IP
		userISP = m.User.Isp
	}

	var avgPing float64
	if len(m.PingResults) > 0 {
		avgPing = sumPing / float64(len(m.PingResults))
	}

	result := &SpeedTestResult{
		DownloadSpeed: dlSpeed,
		UploadSpeed:   ulSpeed,
		Ping:          avgPing,
		Jitter:        jitter,
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

	sendUpdate(1.0, "Test completed", updateChan)
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
		defer f.Close()
		w := csv.NewWriter(f)
		_ = w.Write([]string{"timestamp", "server", "country", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "ip", "isp"})
		_ = w.Write([]string{
			result.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			result.ServerName,
			result.ServerCountry,
			fmt.Sprintf("%.2f", result.DownloadSpeed),
			fmt.Sprintf("%.2f", result.UploadSpeed),
			fmt.Sprintf("%.2f", result.Ping),
			fmt.Sprintf("%.2f", result.Jitter),
			result.UserIP,
			result.UserISP,
		})
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

	var sumPing float64
	var jitter float64
	pingResults := make([]float64, 0)

	headlessPingCount := pingIterations
	if m.Config != nil && m.Config.Test.PingCount > 0 {
		headlessPingCount = m.Config.Test.PingCount
	}

	callProgressFn(opts.ProgressFn, fmt.Sprintf("Measuring ping (0/%d)...", headlessPingCount))
	for i := 0; i < headlessPingCount; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		err := m.Backend.PingTest(server, func(latency time.Duration) {
			ping := float64(latency.Milliseconds())
			pingResults = append(pingResults, ping)
			sumPing += ping
			callProgressFn(opts.ProgressFn, fmt.Sprintf("Measuring ping (%d/%d): %.1f ms", i+1, headlessPingCount, ping))
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

	if len(pingResults) > 1 {
		var sum float64
		for i := 1; i < len(pingResults); i++ {
			sum += math.Abs(pingResults[i] - pingResults[i-1])
		}
		jitter = sum / float64(len(pingResults)-1)
	}

	var avgPing float64
	if len(pingResults) > 0 {
		avgPing = sumPing / float64(len(pingResults))
	}

	var dlSpeed, ulSpeed float64

	if !opts.SkipDownload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(opts.ProgressFn, "Testing download...")
		if err := m.Backend.DownloadTest(server); err != nil {
			return nil, fmt.Errorf("download test failed: %w", err)
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
			return nil, fmt.Errorf("upload test failed: %w", err)
		}
		ulSpeed = float64(server.ULSpeed) / bytesToMbps
		callProgressFn(opts.ProgressFn, fmt.Sprintf("Upload: %.2f Mbps", ulSpeed))
	}

	return &SpeedTestResult{
		DownloadSpeed: dlSpeed,
		UploadSpeed:   ulSpeed,
		Ping:          avgPing,
		Jitter:        jitter,
		ServerName:    server.Name,
		ServerSponsor: server.Sponsor,
		ServerCountry: server.Country,
		Distance:      server.Distance,
		Timestamp:     time.Now(),
		UserIP:        userIP,
		UserISP:       userISP,
	}, nil
}
