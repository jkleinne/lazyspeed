package model

import (
	"context"
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
	bytesToMB               = 1_000_000
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
	ServerLoc     string    `json:"server_loc"`
	Distance      float64   `json:"distance"`
	Timestamp     time.Time `json:"timestamp"`
	UserIP        string    `json:"user_ip"`
	UserISP       string    `json:"user_isp"`
}

type RunOptions struct {
	SkipDownload bool
	SkipUpload   bool
}

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
	SelectingServer        bool
	PendingServerSelection bool
	Cursor                 int
	User                   *speedtest.User
}

func NewModel(backend Backend) *Model {
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
	}
}

func NewDefaultModel() *Model {
	return NewModel(&realBackend{})
}

func sendUpdate(progress float64, phase string, updateChan chan<- ProgressUpdate) {
	if updateChan != nil {
		updateChan <- ProgressUpdate{
			Progress: progress,
			Phase:    phase,
		}
	}
}

func getHistoryFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".lazyspeed_history.json"), nil
}

func (m *Model) LoadHistory() error {
	historyPath, err := getHistoryFilePath()
	if err != nil {
		return err
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
	historyPath, err := getHistoryFilePath()
	if err != nil {
		return err
	}

	// Keep only the last 50 tests
	const maxHistorySize = 50
	if len(m.TestHistory) > maxHistorySize {
		m.TestHistory = m.TestHistory[len(m.TestHistory)-maxHistorySize:]
	}

	data, err := json.MarshalIndent(m.TestHistory, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %v", err)
	}

	if err := os.WriteFile(historyPath, data, 0644); err != nil {
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

	sendUpdate(0.3, "Measuring ping and jitter...", updateChan)
	var sumPing float64
	for i := 0; i < pingIterations; i++ {
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
						ping, currentJitter, i+1, pingIterations), updateChan)
			} else {
				sendUpdate(0.3+float64(i+1)*0.02,
					fmt.Sprintf("Ping: %.1f ms (%d/%d)", ping, i+1, pingIterations), updateChan)
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
				mbps := float64(rate) / bytesToMB

				sendUpdate(progress, fmt.Sprintf("Testing download: %.2f MBps...", mbps), updateChan)
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
	dlSpeed := float64(server.DLSpeed) / bytesToMB
	sendUpdate(0.75, fmt.Sprintf("Download complete: %.2f MBps", dlSpeed), updateChan)

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
				mbps := float64(rate) / bytesToMB

				sendUpdate(progress, fmt.Sprintf("Testing upload: %.2f MBps...", mbps), updateChan)
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
	ulSpeed := float64(server.ULSpeed) / bytesToMB
	sendUpdate(0.9, fmt.Sprintf("Upload complete: %.2f MBps", ulSpeed), updateChan)

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
		ServerLoc:     server.Country,
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

func (m *Model) RunHeadless(ctx context.Context, server *speedtest.Server, opts RunOptions) (*SpeedTestResult, error) {
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

	for i := 0; i < pingIterations; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		err := m.Backend.PingTest(server, func(latency time.Duration) {
			ping := float64(latency.Milliseconds())
			pingResults = append(pingResults, ping)
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
		if err := m.Backend.DownloadTest(server); err != nil {
			return nil, fmt.Errorf("download test failed: %w", err)
		}
		dlSpeed = float64(server.DLSpeed) / bytesToMB
	}

	if !opts.SkipUpload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err := m.Backend.UploadTest(server); err != nil {
			return nil, fmt.Errorf("upload test failed: %w", err)
		}
		ulSpeed = float64(server.ULSpeed) / bytesToMB
	}

	return &SpeedTestResult{
		DownloadSpeed: dlSpeed,
		UploadSpeed:   ulSpeed,
		Ping:          avgPing,
		Jitter:        jitter,
		ServerName:    server.Name,
		ServerSponsor: server.Sponsor,
		ServerLoc:     server.Country,
		Distance:      server.Distance,
		Timestamp:     time.Now(),
		UserIP:        userIP,
		UserISP:       userISP,
	}, nil
}
