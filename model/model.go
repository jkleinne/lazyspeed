package model

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

const (
	bytesPerMbit            = 125_000
	progressInterval        = 200 * time.Millisecond
	pingDelay               = 100 * time.Millisecond
	estimatedTestDurationMs = 15_000
	minPingsForJitter       = 2
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
	progressUploadDone    = 0.96
	progressComplete      = 1.0
)

// DurationMs converts a time.Duration to fractional milliseconds.
func DurationMs(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// SpeedTestCSVHeader is the CSV header row for speed test results.
var SpeedTestCSVHeader = []string{"timestamp", "server", "country", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms", "ip", "isp"}

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
		return fmt.Errorf("failed to unmarshal speed test result: %v", err)
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
		r.Timestamp.Format(time.RFC3339),
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

// HeadlessTestContext groups the parameters shared across headless test functions.
type HeadlessTestContext struct {
	Server  *speedtest.Server
	Opts    RunOptions
	Index   int
	Total   int
	UserIP  string
	UserISP string
}

// Model holds all application state for the TUI and speed test orchestration.
type Model struct {
	History      *HistoryStore
	Servers      *ServerStore
	State        ModelState
	Progress     float64
	CurrentPhase string
	Error        error
	Warning      string
	Width        int
	Height       int
	backend      Backend
	Config       *Config
	user         *speedtest.User
	// ExportMessage is set after an export attempt (success path or error) and
	// shown briefly in the TUI view.
	ExportMessage string
}

func NewModel(backend Backend, cfg *Config) *Model {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Model{
		History: NewHistoryStore(cfg.History),
		Servers: &ServerStore{},
		backend: backend,
		Config:  cfg,
	}
}

func NewDefaultModel() *Model {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}
	m := NewModel(&realBackend{}, cfg)
	if err != nil {
		m.Warning = fmt.Sprintf("could not load config: %v", err)
	}
	return m
}

// userInfo extracts IP and ISP from the User field.
func (m *Model) userInfo() (ip, isp string) {
	if m.user != nil {
		return m.user.IP, m.user.Isp
	}
	return "", ""
}

// pingResult holds the aggregated outcome of a ping measurement.
type pingResult struct {
	pings   []float64
	avgPing float64
	jitter  float64
	lastErr error
}

// pingObserver is called after each successful ping measurement.
// pingNum is 1-based; jitter is 0 when fewer than 2 pings have been recorded.
type pingObserver func(pingNum int, ping, jitter float64)

// measurePing runs count PingTest rounds against server and computes avg/jitter.
// Each round yields multiple ping samples from the library (~10 per call).
// observe is called after each individual sample; pass nil to skip progress reporting.
// Callers should check len(result.pings) == 0 to detect total ping failure.
func measurePing(ctx context.Context, backend Backend, server *speedtest.Server, count int, observe pingObserver) (*pingResult, error) {
	var pings []float64
	var sumPing float64
	var lastErr error

	for i := range count {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		err := backend.PingTest(server, func(latency time.Duration) {
			ping := float64(latency) / float64(time.Millisecond)
			pings = append(pings, ping)
			sumPing += ping
			var currentJitter float64
			if len(pings) >= minPingsForJitter {
				currentJitter = math.Abs(pings[len(pings)-1] - pings[len(pings)-2])
			}
			if observe != nil {
				observe(len(pings), ping, currentJitter)
			}
		})
		if err != nil {
			lastErr = err
			continue
		}
		if i < count-1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(pingDelay):
			}
		}
	}

	result := &pingResult{pings: pings}
	result.lastErr = lastErr

	if len(pings) > 0 {
		result.avgPing = sumPing / float64(len(pings))
	}
	if len(pings) >= minPingsForJitter {
		var sum float64
		for i := 1; i < len(pings); i++ {
			sum += math.Abs(pings[i] - pings[i-1])
		}
		result.jitter = sum / float64(len(pings)-1)
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

// runHeadlessTransfer executes a blocking transfer test while polling rateFn
// to report live speed via the progress callback. This gives headless mode
// the same real-time speed feedback the TUI gets from monitorTransferProgress.
func runHeadlessTransfer(
	ctx context.Context,
	label, prefix string,
	progressFn func(string),
	rateFn func() float64,
	testFn func() error,
) error {
	if progressFn == nil {
		return testFn()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- testFn()
	}()

	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			mbps := rateFn() / bytesPerMbit
			callProgressFn(progressFn, fmt.Sprintf("%sTesting %s: %.2f Mbps...", prefix, label, mbps))
		}
	}
}

// runPingPhase measures ping/jitter and reports TUI progress updates.
func runPingPhase(ctx context.Context, backend Backend, server *speedtest.Server, pingCount int, updateChan chan<- ProgressUpdate) (*pingResult, error) {
	sendUpdate(progressPingStart, "Measuring ping and jitter...", updateChan)
	return measurePing(ctx, backend, server, pingCount, func(pingNum int, ping, jitter float64) {
		pingProgress := min(progressPingStart+float64(pingNum)*progressPingIncrement, progressDownloadStart)
		if jitter > 0 {
			sendUpdate(pingProgress,
				fmt.Sprintf("Ping: %.1f ms, Jitter: %.1f ms", ping, jitter), updateChan)
		} else {
			sendUpdate(pingProgress,
				fmt.Sprintf("Ping: %.1f ms", ping), updateChan)
		}
	})
}

// finalizeTest records results, saves history, and sends the completion update.
func (m *Model) finalizeTest(server *speedtest.Server, pr *pingResult, download, upload float64, updateChan chan<- ProgressUpdate) {
	userIP, userISP := m.userInfo()
	htc := HeadlessTestContext{Server: server, UserIP: userIP, UserISP: userISP}
	result := buildResult(htc, pr, download, upload)

	m.History.Append(result)
	if saveErr := m.History.Save(); saveErr != nil {
		m.Warning = fmt.Sprintf("failed to save history: %v", saveErr)
	}

	sendUpdate(progressComplete, "Test completed", updateChan)
	m.State = StateIdle
}

// FetchServers fetches the server list from the backend.
func (m *Model) FetchServers(ctx context.Context) error {
	return m.Servers.Fetch(ctx, m.backend)
}

// transferPhase defines the progress parameters for a download or upload phase.
type transferPhase struct {
	start   float64
	span    float64
	maxProg float64
	label   string
	rateFn  func() float64 // must return rate in bytes/sec
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
				mbps := phase.rateFn() / bytesPerMbit
				sendUpdate(progress, fmt.Sprintf("Testing %s: %.2f Mbps...", phase.label, mbps), updateChan)
			}
		}
	}()
	return done, doneAck
}

// runTransferPhase executes one transfer phase (download or upload), monitoring
// progress via monitorTransferProgress. It returns the measured speed in Mbps.
func runTransferPhase(
	ctx context.Context,
	phase transferPhase,
	testFn func() error,
	rawSpeed func() float64,
	updateChan chan<- ProgressUpdate,
) (float64, error) {
	done, doneAck := monitorTransferProgress(ctx, phase, updateChan)
	err := testFn()
	close(done)
	<-doneAck
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if err != nil {
		return 0, fmt.Errorf("%s test failed: %v", phase.label, err)
	}
	return rawSpeed() / bytesPerMbit, nil
}

// buildResult constructs a SpeedTestResult from the completed test data.
func buildResult(htc HeadlessTestContext, pr *pingResult, download, upload float64) *SpeedTestResult {
	return &SpeedTestResult{
		DownloadSpeed: download,
		UploadSpeed:   upload,
		Ping:          pr.avgPing,
		Jitter:        pr.jitter,
		ServerName:    htc.Server.Name,
		ServerSponsor: htc.Server.Sponsor,
		ServerCountry: htc.Server.Country,
		Distance:      htc.Server.Distance,
		Timestamp:     time.Now(),
		UserIP:        htc.UserIP,
		UserISP:       htc.UserISP,
	}
}

// initTestState clears model fields and sends the initialization progress update.
func (m *Model) initTestState(updateChan chan<- ProgressUpdate) {
	m.State = StateTesting
	m.Progress = 0
	m.Error = nil
	m.Warning = ""
	m.History.Results = nil
	sendUpdate(progressInit, "Initializing speed test...", updateChan)
}

// fetchUserInfoOrWarn fetches user IP/ISP info and stores it on the model.
// Sets m.Warning on failure instead of returning an error.
func (m *Model) fetchUserInfoOrWarn() {
	user, err := m.backend.FetchUserInfo()
	if err == nil {
		m.user = user
	} else {
		m.Warning = fmt.Sprintf("could not fetch network info: %v", err)
	}
}

// fetchNetworkInfo fetches user IP/ISP and sets a warning on failure.
// Returns a non-nil error only if the context is cancelled.
func (m *Model) fetchNetworkInfo(ctx context.Context, updateChan chan<- ProgressUpdate) error {
	sendUpdate(progressFetchNet, "Fetching network information...", updateChan)
	m.fetchUserInfoOrWarn()
	if ctx.Err() != nil {
		m.State = StateIdle
		return ctx.Err()
	}
	return nil
}

func (m *Model) PerformSpeedTest(ctx context.Context, server *speedtest.Server, updateChan chan<- ProgressUpdate) error {
	m.initTestState(updateChan)

	if err := m.fetchNetworkInfo(ctx, updateChan); err != nil {
		return err
	}

	sendUpdate(progressServer, fmt.Sprintf("Testing with server: %s", server.Name), updateChan)

	pr, err := runPingPhase(ctx, m.backend, server, m.Config.PingCount(), updateChan)
	if err != nil {
		m.State = StateIdle
		return fmt.Errorf("failed to measure ping: %v", err)
	}
	if len(pr.pings) == 0 {
		msg := "all ping measurements failed; ping and jitter are reported as 0"
		if pr.lastErr != nil {
			msg = fmt.Sprintf("%s (last error: %v)", msg, pr.lastErr)
		}
		m.Warning = msg
	}

	sendUpdate(progressDownloadStart, "Starting download test...", updateChan)
	downloadSpeed, err := runTransferPhase(ctx, transferPhase{
		start: progressDownloadStart, span: progressDownloadSpan,
		maxProg: progressDownloadMax, label: "download",
		rateFn: func() float64 { return server.Context.GetEWMADownloadRate() },
	}, func() error { return m.backend.DownloadTest(server) },
		func() float64 { return float64(server.DLSpeed) }, updateChan)
	if err != nil {
		m.State = StateIdle
		return err
	}
	sendUpdate(progressDownloadDone, fmt.Sprintf("Download complete: %.2f Mbps", downloadSpeed), updateChan)

	sendUpdate(progressUploadStart, "Starting upload test...", updateChan)
	uploadSpeed, err := runTransferPhase(ctx, transferPhase{
		start: progressUploadStart, span: progressUploadSpan,
		maxProg: progressUploadMax, label: "upload",
		rateFn: func() float64 { return server.Context.GetEWMAUploadRate() },
	}, func() error { return m.backend.UploadTest(server) },
		func() float64 { return float64(server.ULSpeed) }, updateChan)
	if err != nil {
		m.State = StateIdle
		return err
	}
	sendUpdate(progressUploadDone, fmt.Sprintf("Upload complete: %.2f Mbps", uploadSpeed), updateChan)

	m.finalizeTest(server, pr, downloadSpeed, uploadSpeed, updateChan)
	return nil
}

// ServerError pairs a server name with the reason its test failed.
// It does not implement the error interface; callers inspect Err directly.
type ServerError struct {
	ServerName string
	Err        error
}

// testSingleServerHeadless runs all phases of a headless speed test for one
// server, prefixing each progress update with "[index/total] serverName — ".
// It does not touch Model state (no State transitions, no History writes).
func (m *Model) testSingleServerHeadless(ctx context.Context, htc HeadlessTestContext) (*SpeedTestResult, error) {
	prefix := fmt.Sprintf("[%d/%d] %s — ", htc.Index, htc.Total, htc.Server.Name)

	callProgressFn(htc.Opts.ProgressFn, prefix+"Measuring ping...")
	pr, err := measurePing(ctx, m.backend, htc.Server, m.Config.PingCount(), func(pingNum int, ping, _ float64) {
		callProgressFn(htc.Opts.ProgressFn, fmt.Sprintf("%sMeasuring ping (%d): %.1f ms", prefix, pingNum, ping))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to measure ping: %v", err)
	}
	if len(pr.pings) == 0 {
		errMsg := "all ping measurements failed"
		if pr.lastErr != nil {
			errMsg = fmt.Sprintf("%s (last error: %v)", errMsg, pr.lastErr)
		}
		return nil, fmt.Errorf("failed to measure ping: %s", errMsg)
	}

	var downloadSpeed, uploadSpeed float64

	if !htc.Opts.SkipDownload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(htc.Opts.ProgressFn, prefix+"Testing download...")
		if err := runHeadlessTransfer(ctx, "download", prefix, htc.Opts.ProgressFn,
			func() float64 { return htc.Server.Context.GetEWMADownloadRate() },
			func() error { return m.backend.DownloadTest(htc.Server) },
		); err != nil {
			return nil, fmt.Errorf("failed to measure download speed: %v", err)
		}
		downloadSpeed = float64(htc.Server.DLSpeed) / bytesPerMbit
		callProgressFn(htc.Opts.ProgressFn, fmt.Sprintf("%sDownload: %.2f Mbps", prefix, downloadSpeed))
	}

	if !htc.Opts.SkipUpload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(htc.Opts.ProgressFn, prefix+"Testing upload...")
		if err := runHeadlessTransfer(ctx, "upload", prefix, htc.Opts.ProgressFn,
			func() float64 { return htc.Server.Context.GetEWMAUploadRate() },
			func() error { return m.backend.UploadTest(htc.Server) },
		); err != nil {
			return nil, fmt.Errorf("failed to measure upload speed: %v", err)
		}
		uploadSpeed = float64(htc.Server.ULSpeed) / bytesPerMbit
		callProgressFn(htc.Opts.ProgressFn, fmt.Sprintf("%sUpload: %.2f Mbps", prefix, uploadSpeed))
	}

	return buildResult(htc, pr, downloadSpeed, uploadSpeed), nil
}

// RunMultiServerHeadless runs sequential headless speed tests against each
// server in servers. FetchUserInfo is called once at the start and its result
// is shared across all per-server tests.
//
// Each successful result is appended to History and saved before the next
// server is tested. Per-server failures are collected as ServerErrors and do
// not abort the remaining tests. Context cancellation stops the loop early;
// any untested servers are recorded as ServerErrors with the cancellation error.
func (m *Model) RunMultiServerHeadless(
	ctx context.Context,
	servers []*speedtest.Server,
	opts RunOptions,
) ([]*SpeedTestResult, []ServerError) {
	callProgressFn(opts.ProgressFn, "Fetching network information...")
	m.fetchUserInfoOrWarn()
	userIP, userISP := m.userInfo()

	total := len(servers)
	var results []*SpeedTestResult
	var serverErrors []ServerError

	for i, server := range servers {
		if ctx.Err() != nil {
			// Record all remaining servers as cancelled errors.
			for _, remaining := range servers[i:] {
				serverErrors = append(serverErrors, ServerError{
					ServerName: remaining.Name,
					Err:        ctx.Err(),
				})
			}
			break
		}

		htc := HeadlessTestContext{
			Server: server, Opts: opts,
			Index: i + 1, Total: total,
			UserIP: userIP, UserISP: userISP,
		}
		result, err := m.testSingleServerHeadless(ctx, htc)
		if err != nil {
			serverErrors = append(serverErrors, ServerError{ServerName: server.Name, Err: err})
			continue
		}

		m.History.Append(result)
		if saveErr := m.History.Save(); saveErr != nil {
			m.Warning = fmt.Sprintf("failed to save history: %v", saveErr)
		}
		results = append(results, result)
	}

	return results, serverErrors
}

// testSingleServer runs all test phases for one server in TUI mode, using
// runTransferPhase (with its EWMA-based live progress goroutine) for downloads
// and uploads. Progress values are scaled into [baseProgress, baseProgress+serverSpan]
// so each server occupies its own slice of the overall 0–1 progress range.
func (m *Model) testSingleServer(
	ctx context.Context,
	server *speedtest.Server,
	baseProgress, serverSpan float64,
	prefix string,
	updateChan chan<- ProgressUpdate,
) (*SpeedTestResult, error) {
	scaleProgress := func(fraction float64) float64 {
		return baseProgress + fraction*serverSpan
	}

	sendUpdate(scaleProgress(progressPingStart), prefix+" — Measuring ping and jitter...", updateChan)
	pr, err := measurePing(ctx, m.backend, server, m.Config.PingCount(), func(pingNum int, ping, jitter float64) {
		if jitter > 0 {
			sendUpdate(scaleProgress(progressPingStart+float64(pingNum)*progressPingIncrement),
				fmt.Sprintf("%s — Ping: %.1f ms, Jitter: %.1f ms", prefix, ping, jitter), updateChan)
		} else {
			sendUpdate(scaleProgress(progressPingStart+float64(pingNum)*progressPingIncrement),
				fmt.Sprintf("%s — Ping: %.1f ms", prefix, ping), updateChan)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to measure ping: %v", err)
	}
	if len(pr.pings) == 0 {
		errMsg := "all ping measurements failed"
		if pr.lastErr != nil {
			errMsg = fmt.Sprintf("%s (last error: %v)", errMsg, pr.lastErr)
		}
		return nil, fmt.Errorf("failed to measure ping: %s", errMsg)
	}

	sendUpdate(scaleProgress(progressDownloadStart), prefix+" — Starting download test...", updateChan)
	downloadSpeed, err := runTransferPhase(ctx, transferPhase{
		start:   scaleProgress(progressDownloadStart),
		span:    serverSpan * progressDownloadSpan,
		maxProg: scaleProgress(progressDownloadMax),
		label:   "download",
		rateFn:  func() float64 { return server.Context.GetEWMADownloadRate() },
	}, func() error { return m.backend.DownloadTest(server) },
		func() float64 { return float64(server.DLSpeed) }, updateChan)
	if err != nil {
		return nil, err
	}
	sendUpdate(scaleProgress(progressDownloadDone), fmt.Sprintf("%s — Download: %.2f Mbps", prefix, downloadSpeed), updateChan)

	sendUpdate(scaleProgress(progressUploadStart), prefix+" — Starting upload test...", updateChan)
	uploadSpeed, err := runTransferPhase(ctx, transferPhase{
		start:   scaleProgress(progressUploadStart),
		span:    serverSpan * progressUploadSpan,
		maxProg: scaleProgress(progressUploadMax),
		label:   "upload",
		rateFn:  func() float64 { return server.Context.GetEWMAUploadRate() },
	}, func() error { return m.backend.UploadTest(server) },
		func() float64 { return float64(server.ULSpeed) }, updateChan)
	if err != nil {
		return nil, err
	}
	sendUpdate(scaleProgress(progressUploadDone), fmt.Sprintf("%s — Upload: %.2f Mbps", prefix, uploadSpeed), updateChan)

	userIP, userISP := m.userInfo()
	htc := HeadlessTestContext{Server: server, UserIP: userIP, UserISP: userISP}
	return buildResult(htc, pr, downloadSpeed, uploadSpeed), nil
}

// RunMultiServer orchestrates sequential TUI speed tests across multiple servers.
// It sets StateTesting at the start and StateIdle on completion. Unlike
// PerformSpeedTest it does not call initTestState, so existing History.Results
// is preserved between calls to this method and PerformSpeedTest.
//
// Progress updates are sent on updateChan. Each server occupies an equal slice
// of the 0–1 progress range. Per-server failures are collected as ServerErrors;
// context cancellation records all remaining untested servers as errors and
// stops the loop early.
func (m *Model) RunMultiServer(
	ctx context.Context,
	servers []*speedtest.Server,
	updateChan chan<- ProgressUpdate,
) ([]*SpeedTestResult, []ServerError) {
	m.State = StateTesting
	m.Error = nil
	m.Warning = ""
	m.Progress = 0

	if err := m.fetchNetworkInfo(ctx, updateChan); err != nil {
		m.State = StateIdle
		return nil, nil
	}

	total := len(servers)
	var results []*SpeedTestResult
	var serverErrors []ServerError

	for i, server := range servers {
		if ctx.Err() != nil {
			for _, remaining := range servers[i:] {
				serverErrors = append(serverErrors, ServerError{
					ServerName: remaining.Name,
					Err:        ctx.Err(),
				})
			}
			break
		}

		baseProgress := float64(i) / float64(total)
		serverSpan := 1.0 / float64(total)
		prefix := fmt.Sprintf("[%d/%d] %s", i+1, total, server.Name)

		result, err := m.testSingleServer(ctx, server, baseProgress, serverSpan, prefix, updateChan)
		if err != nil {
			serverErrors = append(serverErrors, ServerError{ServerName: server.Name, Err: err})
			continue
		}

		m.History.Append(result)
		if saveErr := m.History.Save(); saveErr != nil {
			m.Warning = fmt.Sprintf("failed to save history: %v", saveErr)
		}
		results = append(results, result)
	}

	m.State = StateIdle
	return results, serverErrors
}

func (m *Model) RunHeadless(ctx context.Context, server *speedtest.Server, opts RunOptions) (*SpeedTestResult, error) {
	callProgressFn(opts.ProgressFn, "Fetching network information...")
	m.fetchUserInfoOrWarn()
	userIP, userISP := m.userInfo()

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	pingCount := m.Config.PingCount()

	callProgressFn(opts.ProgressFn, "Measuring ping...")
	pingResult, err := measurePing(ctx, m.backend, server, pingCount, func(pingNum int, ping, _ float64) {
		callProgressFn(opts.ProgressFn, fmt.Sprintf("Measuring ping (%d): %.1f ms", pingNum, ping))
	})
	if err != nil {
		return nil, fmt.Errorf("failed to measure ping: %v", err)
	}

	var downloadSpeed, uploadSpeed float64

	if !opts.SkipDownload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(opts.ProgressFn, "Testing download...")
		if err := runHeadlessTransfer(ctx, "download", "", opts.ProgressFn,
			func() float64 { return server.Context.GetEWMADownloadRate() },
			func() error { return m.backend.DownloadTest(server) },
		); err != nil {
			return nil, fmt.Errorf("failed to measure download speed: %v", err)
		}
		downloadSpeed = float64(server.DLSpeed) / bytesPerMbit
		callProgressFn(opts.ProgressFn, fmt.Sprintf("Download: %.2f Mbps", downloadSpeed))
	}

	if !opts.SkipUpload {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		callProgressFn(opts.ProgressFn, "Testing upload...")
		if err := runHeadlessTransfer(ctx, "upload", "", opts.ProgressFn,
			func() float64 { return server.Context.GetEWMAUploadRate() },
			func() error { return m.backend.UploadTest(server) },
		); err != nil {
			return nil, fmt.Errorf("failed to measure upload speed: %v", err)
		}
		uploadSpeed = float64(server.ULSpeed) / bytesPerMbit
		callProgressFn(opts.ProgressFn, fmt.Sprintf("Upload: %.2f Mbps", uploadSpeed))
	}

	htc := HeadlessTestContext{
		Server: server, Opts: opts,
		Index: 1, Total: 1,
		UserIP: userIP, UserISP: userISP,
	}
	return buildResult(htc, pingResult, downloadSpeed, uploadSpeed), nil
}
