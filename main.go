package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jkleinne/lazyspeed/diag"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
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

	// Viewport / UI navigation state (not part of Model's business logic)
	showHelp         bool
	cursor           int
	serverListOffset int
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

func (s *speedTest) adjustServerListOffset() {
	total := s.model.Servers.Len()
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

	case tea.KeyMsg:
		switch s.viewState {
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
	}
	return s, nil
}

func (s *speedTest) handleProgressMsg(msg progressMsg) (tea.Model, tea.Cmd) {
	s.model.Progress = msg.Progress
	s.model.CurrentPhase = msg.Phase
	if s.model.State == model.StateTesting {
		return s, tea.Batch(
			s.spinner.Tick,
			waitForProgress(s.progressChan, s.errChan),
		)
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
		if err := diag.AppendHistory(cfg.Path, msg.result, cfg.MaxEntries); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to persist diagnostics history: %v\n", err)
		}
	}
	return s, nil
}

func (s *speedTest) handleExportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j":
		s.model.State = model.StateIdle
		return s, exportCmd(s.model.History.Results, "json", s.model)
	case "c":
		s.model.State = model.StateIdle
		return s, exportCmd(s.model.History.Results, "csv", s.model)
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
		if s.cursor < s.model.Servers.Len()-1 {
			s.cursor++
			s.adjustServerListOffset()
		}
	case "enter":
		return s.startSpeedTest()
	}
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
	if s.model.Servers.Len() == 0 {
		s.model.State = model.StateAwaitingServers
		s.model.CurrentPhase = fetchingServerListPhase
		return s, s.spinner.Tick
	}
	s.model.State = model.StateSelectingServer
	s.cursor = 0
	s.serverListOffset = 0
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
	}
	return s, nil
}

func (s *speedTest) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(ui.RenderTitle(s.model.Width))
	b.WriteString("\n\n")

	switch s.viewState {
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
	if s.cursor < 0 || s.cursor >= s.model.Servers.Len() {
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
		server := s.model.Servers.Raw()[s.cursor]
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
		b.WriteString(ui.RenderServerSelection(s.model.Servers.List(), ui.Viewport{
			Width:  s.model.Width,
			Height: s.model.Height,
			Offset: s.serverListOffset,
			Cursor: s.cursor,
		}))

	case model.StateTesting:
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase, s.model.Progress))
		b.WriteString("\n\n")

	case model.StateExporting, model.StateIdle:
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
	}

	return b.String()
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
		path, err := model.ExportResult(result, format, dir)
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
