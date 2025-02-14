package model

import (
	"fmt"
	"time"

	"sort"

	"github.com/showwin/speedtest-go/speedtest"
)

type ProgressUpdate struct {
	Progress float64
	Phase    string
}

type SpeedTestResult struct {
	DownloadSpeed float64
	UploadSpeed   float64
	Ping          float64
	Jitter        float64
	ServerName    string
	ServerLoc     string
	Timestamp     time.Time
}

type Model struct {
	Results       *SpeedTestResult
	TestHistory   []*SpeedTestResult
	Testing       bool
	Progress      float64
	CurrentPhase  string
	Error         error
	ShowHelp      bool
	Width, Height int
	PingResults   []float64 // Used for jitter calculation
}

func NewModel() *Model {
	return &Model{
		Results:      nil,
		TestHistory:  make([]*SpeedTestResult, 0),
		Testing:      false,
		Progress:     0,
		CurrentPhase: "",
		ShowHelp:     true,
	}
}

func sendUpdate(progress float64, phase string, updateChan chan<- ProgressUpdate) {
	if updateChan != nil {
		updateChan <- ProgressUpdate{
			Progress: progress,
			Phase:    phase,
		}
	}
}

func (m *Model) PerformSpeedTest(updateChan chan<- ProgressUpdate) error {
	m.Testing = true
	m.Progress = 0
	m.Error = nil
	m.Results = nil
	m.PingResults = make([]float64, 0)

	sendUpdate(0.0, "Initializing speed test...", updateChan)

	sendUpdate(0.1, "Finding closest server...", updateChan)
	serverList, err := speedtest.FetchServers()
	if err != nil {
		return fmt.Errorf("failed to fetch servers: %v", err)
	}

	if len(serverList) == 0 {
		return fmt.Errorf("no servers available")
	}

	// Find the closest server
	sort.Slice(serverList, func(i, j int) bool {
		return serverList[i].Latency < serverList[j].Latency
	})
	server := serverList[0]
	sendUpdate(0.15, fmt.Sprintf("Selected server: %s with latency %.2f ms", server.Name, server.Latency), updateChan)
	sendUpdate(0.2, fmt.Sprintf("Selected server: %s", server.Name), updateChan)

	sendUpdate(0.3, "Measuring ping and jitter...", updateChan)
	var sumPing float64
	for i := 0; i < 10; i++ {
		err := server.PingTest(func(latency time.Duration) {
			ping := float64(latency.Milliseconds())
			m.PingResults = append(m.PingResults, ping)
			sumPing += ping
			if len(m.PingResults) > 1 {
				// Calculate current jitter for display
				lastIdx := len(m.PingResults) - 1
				currentJitter := abs(m.PingResults[lastIdx] - m.PingResults[lastIdx-1])
				sendUpdate(0.3+float64(i+1)*0.02,
					fmt.Sprintf("Ping: %.1f ms, Jitter: %.1f ms (%d/10)",
						ping, currentJitter, i+1), updateChan)
			} else {
				sendUpdate(0.3+float64(i+1)*0.02,
					fmt.Sprintf("Ping: %.1f ms (%d/10)", ping, i+1), updateChan)
			}
		})
		if err != nil {
			continue
		}
		time.Sleep(100 * time.Millisecond) // Small delay between pings
	}

	var jitter float64
	if len(m.PingResults) > 1 {
		var sum float64
		for i := 1; i < len(m.PingResults); i++ {
			sum += abs(m.PingResults[i] - m.PingResults[i-1])
		}
		jitter = sum / float64(len(m.PingResults)-1)
	}

	sendUpdate(0.5, "Starting download test...", updateChan)
	done := make(chan struct{})
	go func() {
		progress := 0.5
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				progress += 0.02
				if progress > 0.7 {
					progress = 0.7
				}
				sendUpdate(progress, "Testing download speed...", updateChan)
			}
		}
	}()
	err = server.DownloadTest()
	sendUpdate(0.75, fmt.Sprintf("Download test completed. server.DLSpeed: %f bps", server.DLSpeed), updateChan)
	close(done)
	if err != nil {
		return fmt.Errorf("download test failed: %v", err)
	}
	dlSpeed := float64(server.DLSpeed) / 1000000
	sendUpdate(0.7, fmt.Sprintf("Download complete: %.2f MBps (server.DLSpeed: %f)", dlSpeed, server.DLSpeed), updateChan)

	sendUpdate(0.8, "Starting upload test...", updateChan)
	done = make(chan struct{})
	go func() {
		progress := 0.8
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				progress += 0.01
				if progress > 0.9 {
					progress = 0.9
				}
				sendUpdate(progress, "Testing upload speed...", updateChan)
			}
		}
	}()
	err = server.UploadTest()
	close(done)
	if err != nil {
		return fmt.Errorf("upload test failed: %v", err)
	}
	ulSpeed := float64(server.ULSpeed) / 1000000
	sendUpdate(0.9, fmt.Sprintf("Upload complete: %.2f MBps", ulSpeed), updateChan)

	result := &SpeedTestResult{
		DownloadSpeed: dlSpeed,
		UploadSpeed:   ulSpeed,
		Ping:          sumPing / float64(len(m.PingResults)),
		Jitter:        jitter,
		ServerName:    server.Name,
		ServerLoc:     server.Country,
		Timestamp:     time.Now(),
	}

	m.Results = result
	m.TestHistory = append(m.TestHistory, result)

	sendUpdate(1.0, "Test completed", updateChan)
	m.Testing = false
	return nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
