package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
	"github.com/showwin/speedtest-go/speedtest"
)

// exportDoneMsg is sent when an in-TUI export operation completes.
type exportDoneMsg struct {
	path string
	err  error
}

const (
	keyCtrlC = "ctrl+c"
	keyEsc   = "esc"
	keyUp    = "up"
	keyDown  = "down"
	keyJ     = "j"
	keyK     = "k"
)

const fetchingServerListPhase = "Fetching server list..."

const defaultDiagTarget = "8.8.8.8"

const runningDiagnosticsPhase = "Running diagnostics..."

// ViewState represents the TUI view overlay state.
type ViewState int

const (
	ViewMain         ViewState = iota // Delegates to Model.State for rendering
	ViewDiagRunning                   // Diagnostics spinner
	ViewDiagCompact                   // Compact diagnostics summary
	ViewDiagExpanded                  // Full hop-by-hop trace table
	ViewAnalytics                     // Analytics summary view
	ViewComparison                    // Multi-server comparison results
)

type speedTest struct {
	model        *model.Model
	spinner      spinner.Model
	quitting     bool
	progressChan chan model.ProgressUpdate
	errChan      chan error
	cancelTest   context.CancelFunc

	diagResult *diag.DiagResult
	viewState  ViewState
	diagOffset int

	analyticsSummary *model.Summary

	selectedServers       map[int]bool
	comparisonResults     []*model.SpeedTestResult
	comparisonErrors      []model.ServerError
	multiServerResultChan chan multiServerCompleteMsg

	// Viewport / UI navigation state (not part of Model's business logic)
	showHelp         bool
	cursor           int
	serverListOffset int
	displayOrder     []int // maps display position → raw server index
	historyOffset    int
}

type progressMsg struct {
	Progress float64
	Phase    string
}

type testComplete struct {
	err error
}

type serverListMsg struct {
	err error
}

type diagCompleteMsg struct {
	result *diag.DiagResult
	err    error
}

type multiServerCompleteMsg struct {
	results []*model.SpeedTestResult
	errors  []model.ServerError
}

func runDiagCmd(m *model.Model, cfg *diag.DiagConfig) tea.Cmd {
	return func() tea.Msg {
		var target string
		if m.Servers.Len() > 0 {
			srv := m.Servers.Raw()[0]
			target = stripPort(srv.Host)
			if target == "" {
				target = srv.Name
			}
		} else {
			target = defaultDiagTarget
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
		defer cancel()

		backend := &diag.RealDiagBackend{}
		result, err := diag.Run(ctx, backend, target, cfg)
		return diagCompleteMsg{result: result, err: err}
	}
}

func fetchServerListCmd(m *model.Model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.Config.FetchTimeoutDuration())
		defer cancel()
		err := m.FetchServers(ctx)
		return serverListMsg{err: err}
	}
}

func (s *speedTest) Init() tea.Cmd {
	cmds := []tea.Cmd{fetchServerListCmd(s.model)}

	if len(s.model.History.Entries) == 0 {
		s.model.State = model.StateAwaitingServers
		s.model.CurrentPhase = fetchingServerListPhase
		cmds = append(cmds, s.spinner.Tick)
	}

	return tea.Batch(cmds...)
}

func (s *speedTest) cancelTestIfRunning() {
	if s.cancelTest != nil {
		s.cancelTest()
		s.cancelTest = nil
	}
}

// computeDisplayOrder builds the display-position-to-raw-index mapping.
// Favorites appear first (in latency order), then non-favorites (in latency order).
// The raw server list from ServerStore.List() is already sorted by latency,
// so iterating in order preserves latency sorting within each section.
func (s *speedTest) computeDisplayOrder() {
	total := s.model.Servers.Len()
	if total == 0 {
		s.displayOrder = nil
		return
	}

	favSet := s.favoriteSet()
	servers := s.model.Servers.List()
	favorites := make([]int, 0)
	others := make([]int, 0, total)

	for i := range total {
		if favSet[servers[i].ID] {
			favorites = append(favorites, i)
		} else {
			others = append(others, i)
		}
	}

	s.displayOrder = make([]int, 0, total)
	s.displayOrder = append(s.displayOrder, favorites...)
	s.displayOrder = append(s.displayOrder, others...)
}

// favoriteSet returns the current favorite IDs as a set for O(1) lookup.
func (s *speedTest) favoriteSet() map[string]bool {
	ids := s.model.Config.Servers.FavoriteIDs
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// displayServers returns servers reordered by displayOrder for rendering.
func (s *speedTest) displayServers() []model.Server {
	all := s.model.Servers.List()
	if len(s.displayOrder) == 0 {
		return all
	}
	ordered := make([]model.Server, len(s.displayOrder))
	for i, rawIdx := range s.displayOrder {
		ordered[i] = all[rawIdx]
	}
	return ordered
}

// rawIndex translates a display-position cursor to the raw server index.
func (s *speedTest) rawIndex(displayIdx int) int {
	if displayIdx >= 0 && displayIdx < len(s.displayOrder) {
		return s.displayOrder[displayIdx]
	}
	return displayIdx
}

func (s *speedTest) adjustServerListOffset() {
	total := len(s.displayOrder)
	visible := ui.ServerListVisibleLines(s.model.Height, total)

	if s.cursor >= s.serverListOffset+visible {
		s.serverListOffset = s.cursor - visible + 1
	}
	if s.cursor < s.serverListOffset {
		s.serverListOffset = s.cursor
	}
}

func (s *speedTest) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.model.Width = msg.Width
		s.model.Height = msg.Height
		return s, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		return s, cmd

	case exportDoneMsg:
		if msg.err != nil {
			s.model.ExportMessage = fmt.Sprintf("Export failed: %v", msg.err)
		} else {
			s.model.ExportMessage = fmt.Sprintf("Saved to %s", msg.path)
		}
		return s, nil

	case serverListMsg:
		return s.handleServerListMsg(msg)

	case progressMsg:
		return s.handleProgressMsg(msg)

	case testComplete:
		return s.handleTestComplete(msg)

	case diagCompleteMsg:
		return s.handleDiagComplete(msg)

	case multiServerCompleteMsg:
		return s.handleMultiServerComplete(msg)

	case tea.KeyMsg:
		switch s.viewState {
		case ViewComparison:
			return s.handleComparisonKeys(msg)
		case ViewAnalytics:
			return s.handleAnalyticsKeys(msg)
		case ViewDiagExpanded:
			return s.handleDiagExpandedKeys(msg)
		case ViewDiagCompact:
			return s.handleDiagCompactKeys(msg)
		case ViewDiagRunning:
			return s.handleDiagRunningKeys(msg)
		case ViewMain:
			switch s.model.State {
			case model.StateExporting:
				return s.handleExportKeys(msg)
			case model.StateAwaitingServers:
				return s.handleAwaitingServersKeys(msg)
			case model.StateSelectingServer:
				return s.handleServerSelectionKeys(msg)
			case model.StateTesting:
				return s.handleTestingKeys(msg)
			case model.StateIdle:
				return s.handleIdleKeys(msg)
			}
		}
	}
	return s, nil
}

func (s *speedTest) handleServerListMsg(msg serverListMsg) (tea.Model, tea.Cmd) {
	s.model.CurrentPhase = ""
	if msg.err != nil {
		s.model.Error = msg.err
		if s.model.State == model.StateAwaitingServers {
			s.model.State = model.StateIdle
		}
	} else if s.model.State == model.StateAwaitingServers || len(s.model.History.Entries) == 0 {
		s.model.State = model.StateSelectingServer
		s.cursor = 0
		s.serverListOffset = 0
		s.computeDisplayOrder()
	}
	return s, nil
}

func (s *speedTest) handleProgressMsg(msg progressMsg) (tea.Model, tea.Cmd) {
	s.model.Progress = msg.Progress
	s.model.CurrentPhase = msg.Phase
	if s.model.State == model.StateTesting {
		var nextCmd tea.Cmd
		if s.multiServerResultChan != nil {
			nextCmd = waitForMultiServer(s.progressChan, s.multiServerResultChan)
		} else {
			nextCmd = waitForProgress(s.progressChan, s.errChan)
		}
		return s, tea.Batch(s.spinner.Tick, nextCmd)
	}
	return s, nil
}

func (s *speedTest) handleTestComplete(msg testComplete) (tea.Model, tea.Cmd) {
	s.cancelTest = nil
	s.model.State = model.StateIdle
	s.historyOffset = 0
	if msg.err != nil {
		s.model.Error = msg.err
		s.model.History.Results = nil
	}
	return s, nil
}

func (s *speedTest) handleDiagComplete(msg diagCompleteMsg) (tea.Model, tea.Cmd) {
	s.model.CurrentPhase = ""
	if msg.err != nil {
		s.model.Error = msg.err
		s.viewState = ViewMain
	} else {
		s.diagResult = msg.result
		s.viewState = ViewDiagCompact
		cfg := diagConfig(s.model.Config.Diagnostics)
		if cfg.MaxEntries > 0 {
			if err := diag.AppendHistory(cfg.Path, msg.result, cfg.MaxEntries); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to persist diagnostics history: %v\n", err)
			}
		}
	}
	return s, nil
}

func (s *speedTest) handleExportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j":
		s.model.State = model.StateIdle
		return s, exportCmd(s.model.History.Results, formatJSON, s.model)
	case "c":
		s.model.State = model.StateIdle
		return s, exportCmd(s.model.History.Results, formatCSV, s.model)
	case keyEsc, "q", keyCtrlC:
		s.model.State = model.StateIdle
	}
	return s, nil
}

func (s *speedTest) handleAwaitingServersKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	}
	return s, nil
}

func (s *speedTest) handleServerSelectionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	case keyEsc:
		s.model.State = model.StateIdle
		s.showHelp = true
	case keyUp, keyK:
		if s.cursor > 0 {
			s.cursor--
			s.adjustServerListOffset()
		}
	case keyDown, keyJ:
		if s.cursor < len(s.displayOrder)-1 {
			s.cursor++
			s.adjustServerListOffset()
		}
	case " ":
		if s.selectedServers == nil {
			s.selectedServers = make(map[int]bool)
		}
		if s.selectedServers[s.cursor] {
			delete(s.selectedServers, s.cursor)
		} else {
			s.selectedServers[s.cursor] = true
		}
	case "f":
		return s.toggleFavorite()
	case "enter":
		if len(s.selectedServers) >= 2 {
			return s.startMultiServerTest()
		}
		return s.startSpeedTest()
	}
	return s, nil
}

// toggleFavorite adds or removes the highlighted server from favorites,
// saves config, recomputes display order, and moves cursor to follow the server.
func (s *speedTest) toggleFavorite() (tea.Model, tea.Cmd) {
	if s.cursor < 0 || s.cursor >= len(s.displayOrder) {
		return s, nil
	}

	rawIdx := s.rawIndex(s.cursor)
	serverID := s.model.Servers.List()[rawIdx].ID

	favs := s.model.Config.Servers.FavoriteIDs
	found := false
	for i, id := range favs {
		if id == serverID {
			s.model.Config.Servers.FavoriteIDs = append(favs[:i], favs[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		s.model.Config.Servers.FavoriteIDs = append(favs, serverID)
	}

	if err := model.SaveConfig(s.model.Config); err != nil {
		s.model.Warning = fmt.Sprintf("could not save config: %v", err)
	}

	s.computeDisplayOrder()

	// Move cursor to follow the toggled server.
	for i, rawI := range s.displayOrder {
		if rawI == rawIdx {
			s.cursor = i
			break
		}
	}

	s.selectedServers = nil
	s.adjustServerListOffset()
	return s, nil
}

func (s *speedTest) handleTestingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.cancelTestIfRunning()
		s.quitting = true
		return s, tea.Quit
	}
	return s, nil
}

func (s *speedTest) handleDiagRunningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	}
	return s, nil
}

func (s *speedTest) startDiagnostics() (tea.Model, tea.Cmd) {
	s.viewState = ViewDiagRunning
	s.diagResult = nil
	s.model.CurrentPhase = runningDiagnosticsPhase
	s.showHelp = false
	cfg := diagConfig(s.model.Config.Diagnostics)
	return s, tea.Batch(s.spinner.Tick, runDiagCmd(s.model, cfg))
}

func (s *speedTest) startNewTest() (tea.Model, tea.Cmd) {
	s.viewState = ViewMain
	s.diagResult = nil
	s.showHelp = false
	s.selectedServers = nil
	s.comparisonResults = nil
	s.comparisonErrors = nil
	s.multiServerResultChan = nil
	if s.model.Servers.Len() == 0 {
		s.model.State = model.StateAwaitingServers
		s.model.CurrentPhase = fetchingServerListPhase
		return s, s.spinner.Tick
	}
	s.model.State = model.StateSelectingServer
	s.cursor = 0
	s.serverListOffset = 0
	s.computeDisplayOrder()
	return s, nil
}

func (s *speedTest) handleDiagExpandedKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	case keyEsc:
		s.viewState = ViewDiagCompact
	case keyUp, keyK:
		if s.diagOffset > 0 {
			s.diagOffset--
		}
	case keyDown, keyJ:
		if s.diagResult != nil && s.diagOffset < len(s.diagResult.Hops)-1 {
			s.diagOffset++
		}
	case "d":
		return s.startDiagnostics()
	}
	return s, nil
}

func (s *speedTest) handleDiagCompactKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	case keyEsc:
		s.viewState = ViewMain
		s.diagResult = nil
		s.showHelp = true
	case "enter":
		s.viewState = ViewDiagExpanded
		s.diagOffset = 0
	case "d":
		return s.startDiagnostics()
	case "n":
		return s.startNewTest()
	}
	return s, nil
}

func (s *speedTest) handleIdleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.cancelTestIfRunning()
		s.quitting = true
		return s, tea.Quit
	case keyUp, keyK:
		if s.historyOffset > 0 {
			s.historyOffset--
		}
	case keyDown, keyJ:
		totalRows := len(s.model.History.Entries) - 1
		maxVisible := ui.HistoryVisibleRows(s.model.Height, totalRows)
		if totalRows > maxVisible && s.historyOffset < totalRows-maxVisible {
			s.historyOffset++
		}
	case "n":
		return s.startNewTest()
	case "d":
		return s.startDiagnostics()
	case "e":
		if s.model.History.Results != nil {
			s.model.State = model.StateExporting
			s.model.ExportMessage = ""
		}
	case "h":
		s.showHelp = !s.showHelp
	case "a":
		if len(s.model.History.Entries) > 0 {
			s.analyticsSummary = model.ComputeSummary(s.model.History.Entries)
			s.viewState = ViewAnalytics
			s.showHelp = false
		}
	}
	return s, nil
}

func (s *speedTest) handleAnalyticsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	case keyEsc:
		s.viewState = ViewMain
		s.analyticsSummary = nil
		s.showHelp = true
	case "n":
		s.analyticsSummary = nil
		return s.startNewTest()
	}
	return s, nil
}

func (s *speedTest) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(ui.RenderTitle(s.model.Width))
	b.WriteString("\n\n")

	switch s.viewState {
	case ViewComparison:
		if s.comparisonResults != nil || s.comparisonErrors != nil {
			b.WriteString(ui.RenderComparison(s.comparisonResults, s.comparisonErrors, s.model.Width))
			b.WriteString("\n")
		}

	case ViewAnalytics:
		b.WriteString(ui.RenderAnalytics(s.analyticsSummary, s.model.Width))
		b.WriteString("\n")

	case ViewDiagRunning:
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase, 0))
		b.WriteString("\n\n")

	case ViewDiagCompact:
		if s.diagResult != nil {
			b.WriteString(ui.RenderDiagCompact(s.diagResult, s.model.Width))
			b.WriteString("\n")
		}

	case ViewDiagExpanded:
		if s.diagResult != nil {
			b.WriteString(ui.RenderDiagExpanded(s.diagResult, s.model.Width, s.model.Height, s.diagOffset))
			b.WriteString("\n")
		}

	case ViewMain:
		b.WriteString(s.renderMainView())
	}

	b.WriteString("\n")
	return b.String()
}

func (s *speedTest) startSpeedTest() (tea.Model, tea.Cmd) {
	rawIdx := s.rawIndex(s.cursor)
	if rawIdx < 0 || rawIdx >= s.model.Servers.Len() {
		s.model.Error = fmt.Errorf("invalid server selection")
		s.model.State = model.StateIdle
		s.showHelp = false
		return s, nil
	}

	s.model.State = model.StateTesting
	s.model.Progress = 0
	s.model.CurrentPhase = "Starting speed test..."
	s.model.Error = nil

	ctx, cancel := context.WithTimeout(context.Background(), s.model.Config.TestTimeoutDuration())
	s.cancelTest = cancel

	s.progressChan = make(chan model.ProgressUpdate)
	s.errChan = make(chan error, 1)
	go func() {
		server := s.model.Servers.Raw()[rawIdx]
		err := s.model.PerformSpeedTest(ctx, server, s.progressChan)
		s.errChan <- err
		close(s.progressChan)
	}()
	s.showHelp = false
	return s, tea.Batch(
		s.spinner.Tick,
		waitForProgress(s.progressChan, s.errChan),
	)
}

func (s *speedTest) renderMainView() string {
	var b strings.Builder

	switch s.model.State {
	case model.StateAwaitingServers:
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase, 0))
		b.WriteString("\n\n")

	case model.StateSelectingServer:
		b.WriteString(ui.RenderServerSelection(s.displayServers(), ui.Viewport{
			Width:  s.model.Width,
			Height: s.model.Height,
			Offset: s.serverListOffset,
			Cursor: s.cursor,
		}, s.selectedServers, s.favoriteSet()))

	case model.StateTesting:
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase, s.model.Progress))
		b.WriteString("\n\n")

	case model.StateExporting, model.StateIdle:
		b.WriteString(s.renderIdleView())
	}

	return b.String()
}

func (s *speedTest) renderIdleView() string {
	var b strings.Builder

	if s.model.History.Results != nil || len(s.model.History.Entries) > 0 {
		b.WriteString(ui.RenderResults(s.model.History.Entries, ui.Viewport{
			Width:  s.model.Width,
			Height: s.model.Height,
			Offset: s.historyOffset,
		}))
		b.WriteString("\n")
	}

	if s.model.Error != nil {
		b.WriteString("\n")
		b.WriteString(ui.RenderError(s.model.Error, s.model.Width))
	}

	if s.model.Warning != "" {
		b.WriteString("\n")
		b.WriteString(ui.RenderWarning(s.model.Warning, s.model.Width))
	}

	if s.model.State == model.StateExporting {
		b.WriteString("\n")
		b.WriteString(ui.RenderExportPrompt(s.model.Width))
	} else if s.model.ExportMessage != "" {
		b.WriteString("\n")
		b.WriteString(ui.RenderExportMessage(s.model.ExportMessage, s.model.Width))
	}

	if s.showHelp {
		b.WriteString(ui.RenderHelp(s.model.Width, s.model.History.Results != nil))
	}

	return b.String()
}

func waitForMultiServer(progressChan chan model.ProgressUpdate, resultChan chan multiServerCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-progressChan
		if !ok {
			return <-resultChan
		}
		return progressMsg{
			Progress: update.Progress,
			Phase:    update.Phase,
		}
	}
}

func (s *speedTest) startMultiServerTest() (tea.Model, tea.Cmd) {
	indices := make([]int, 0, len(s.selectedServers))
	for displayIdx := range s.selectedServers {
		indices = append(indices, s.rawIndex(displayIdx))
	}
	slices.Sort(indices)

	raw := s.model.Servers.Raw()
	servers := make([]*speedtest.Server, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(raw) {
			servers = append(servers, raw[idx])
		}
	}

	s.model.State = model.StateTesting
	s.model.Progress = 0
	s.model.CurrentPhase = "Starting multi-server test..."
	s.model.Error = nil

	timeout := s.model.Config.TestTimeoutDuration() * time.Duration(len(servers))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	s.cancelTest = cancel

	s.progressChan = make(chan model.ProgressUpdate)
	resultChan := make(chan multiServerCompleteMsg, 1)
	s.multiServerResultChan = resultChan

	go func() {
		results, errs := s.model.RunMultiServer(ctx, servers, s.progressChan)
		close(s.progressChan)
		resultChan <- multiServerCompleteMsg{results: results, errors: errs}
	}()

	s.showHelp = false
	return s, tea.Batch(
		s.spinner.Tick,
		waitForMultiServer(s.progressChan, resultChan),
	)
}

func (s *speedTest) handleMultiServerComplete(msg multiServerCompleteMsg) (tea.Model, tea.Cmd) {
	s.cancelTest = nil
	s.model.State = model.StateIdle
	s.comparisonResults = msg.results
	s.comparisonErrors = msg.errors
	s.selectedServers = nil
	s.viewState = ViewComparison
	return s, nil
}

func (s *speedTest) handleComparisonKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", keyCtrlC:
		s.quitting = true
		return s, tea.Quit
	case keyEsc:
		s.viewState = ViewMain
		s.comparisonResults = nil
		s.comparisonErrors = nil
		s.showHelp = true
	case "n":
		s.comparisonResults = nil
		s.comparisonErrors = nil
		return s.startNewTest()
	}
	return s, nil
}

func waitForProgress(progressChan chan model.ProgressUpdate, errChan chan error) tea.Cmd {
	return func() tea.Msg {
		update, ok := <-progressChan
		if !ok {
			err := <-errChan
			return testComplete{err: err}
		}
		return progressMsg{
			Progress: update.Progress,
			Phase:    update.Phase,
		}
	}
}

// exportCmd runs the file export in a goroutine and returns the result as a tea.Cmd.
func exportCmd(result *model.SpeedTestResult, format string, m *model.Model) tea.Cmd {
	return func() tea.Msg {
		dir, err := m.Config.ExportDir()
		if err != nil {
			return exportDoneMsg{err: err}
		}
		path, err := ExportResult(result, format, dir)
		return exportDoneMsg{path: path, err: err}
	}
}

func migrateHistoryIfNeeded() {
	legacy, err := model.LegacyHistoryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to resolve legacy history path: %v\n", err)
		return
	}
	// Check if the legacy file exists
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return
	}
	// Check if the new path already exists — don't overwrite
	newPath := model.DefaultConfig().History.Path
	if _, err := os.Stat(newPath); err == nil {
		return
	}
	// Copy legacy → new path
	data, err := os.ReadFile(legacy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read legacy history for migration: %v\n", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create directory for history migration: %v\n", err)
		return
	}
	if err := os.WriteFile(newPath, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write migrated history: %v\n", err)
		return
	}
	// Remove the legacy file
	if err := os.Remove(legacy); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove legacy history file: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "Info: migrated history from %s to %s\n", legacy, newPath)
}

func runTUI() {
	migrateHistoryIfNeeded()
	m := model.NewDefaultModel()
	if err := m.History.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load test history: %v\n", err)
	}

	s := speedTest{
		model:    m,
		spinner:  ui.DefaultSpinner,
		showHelp: true,
	}

	if _, err := tea.NewProgram(&s, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
