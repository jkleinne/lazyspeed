package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
)

// exportDoneMsg is sent when an in-TUI export operation completes.
type exportDoneMsg struct {
	path string
	err  error
}

const keyCtrlC = "ctrl+c"

const fetchingServerListPhase = "Fetching server list..."

type speedTest struct {
	model        *model.Model
	spinner      spinner.Model
	quitting     bool
	progressChan chan model.ProgressUpdate
	errChan      chan error
	cancelTest   context.CancelFunc
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

func fetchServerListCmd(m *model.Model) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.FetchTimeoutDuration())
		defer cancel()
		err := m.FetchServerList(ctx)
		return serverListMsg{err: err}
	}
}

func (s *speedTest) Init() tea.Cmd {
	s.model.FetchingServers = true
	cmds := []tea.Cmd{fetchServerListCmd(s.model)}

	if len(s.model.TestHistory) == 0 {
		// First launch: show loading spinner since there's nothing else to display
		s.model.PendingServerSelection = true
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

func (s *speedTest) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Export format prompt takes priority over all other key handlers
		if s.model.Exporting {
			switch msg.String() {
			case "j":
				s.model.Exporting = false
				return s, exportCmd(s.model.Results, "json")
			case "c":
				s.model.Exporting = false
				return s, exportCmd(s.model.Results, "csv")
			case "esc", "q", keyCtrlC:
				s.model.Exporting = false
			}
			return s, nil
		}

		if s.model.FetchingServers && s.model.PendingServerSelection {
			// Spinner is visible while waiting for server list — only allow quit
			switch msg.String() {
			case "q", keyCtrlC:
				s.quitting = true
				return s, tea.Quit
			}
		} else if s.model.SelectingServer {
			switch msg.String() {
			case "q", keyCtrlC:
				s.quitting = true
				return s, tea.Quit
			case "up", "k":
				if s.model.Cursor > 0 {
					s.model.Cursor--
				}
			case "down", "j":
				if s.model.Cursor < len(s.model.ServerList)-1 {
					s.model.Cursor++
				}
			case "enter":
				if s.model.Cursor < 0 || s.model.Cursor >= len(s.model.ServerList) {
					s.model.Error = fmt.Errorf("invalid server selection")
					s.model.SelectingServer = false
					s.model.ShowHelp = false
					return s, nil
				}
				s.model.SelectingServer = false
				s.model.Testing = true
				s.model.Progress = 0
				s.model.CurrentPhase = "Starting speed test..."
				s.model.Error = nil

				ctx, cancel := context.WithTimeout(context.Background(), s.model.TestTimeoutDuration())
				s.cancelTest = cancel

				s.progressChan = make(chan model.ProgressUpdate)
				s.errChan = make(chan error, 1)
				go func() {
					server := s.model.ServerList[s.model.Cursor]
					err := s.model.PerformSpeedTest(ctx, server, s.progressChan)
					s.errChan <- err
					close(s.progressChan)
				}()
				s.model.ShowHelp = false
				return s, tea.Batch(
					s.spinner.Tick,
					waitForProgress(s.progressChan, s.errChan),
				)
			}
		} else {
			switch msg.String() {
			case "q", keyCtrlC:
				s.cancelTestIfRunning()
				s.quitting = true
				return s, tea.Quit
			case "n":
				if !s.model.Testing && !s.model.SelectingServer {
					if s.model.FetchingServers {
						// Servers still loading — show the spinner and queue the transition
						s.model.PendingServerSelection = true
						s.model.CurrentPhase = fetchingServerListPhase
						s.model.ShowHelp = false
						return s, s.spinner.Tick
					}
					s.model.SelectingServer = true
					s.model.ShowHelp = false
				}
			case "e":
				if !s.model.Testing && s.model.Results != nil {
					s.model.Exporting = true
					s.model.ExportMessage = ""
				}
			case "h":
				s.model.ShowHelp = !s.model.ShowHelp
			}
		}

	case tea.WindowSizeMsg:
		s.model.Width = msg.Width
		s.model.Height = msg.Height

	case spinner.TickMsg:
		var tickCmd tea.Cmd
		s.spinner, tickCmd = s.spinner.Update(msg)
		return s, tickCmd

	case serverListMsg:
		s.model.FetchingServers = false
		s.model.CurrentPhase = ""
		if msg.err != nil {
			s.model.Error = msg.err
			s.model.PendingServerSelection = false
		} else if s.model.PendingServerSelection || len(s.model.TestHistory) == 0 {
			s.model.SelectingServer = true
			s.model.PendingServerSelection = false
		}
		return s, nil

	case progressMsg:
		s.model.Progress = msg.Progress
		s.model.CurrentPhase = msg.Phase
		// Continue listening for further updates if the test is still running
		if s.model.Testing {
			return s, tea.Batch(
				s.spinner.Tick,
				waitForProgress(s.progressChan, s.errChan),
			)
		}
		return s, nil

	case testComplete:
		s.cancelTest = nil
		s.model.Testing = false
		if msg.err != nil {
			s.model.Error = msg.err
			s.model.Results = nil
		}
		return s, nil

	case exportDoneMsg:
		if msg.err != nil {
			s.model.ExportMessage = fmt.Sprintf("Export failed: %v", msg.err)
		} else {
			s.model.ExportMessage = fmt.Sprintf("Saved to %s", msg.path)
		}
		return s, nil
	}

	return s, cmd
}

func (s *speedTest) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(ui.RenderTitle(s.model.Width))
	b.WriteString("\n\n")

	if s.model.FetchingServers && s.model.PendingServerSelection {
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase, 0))
		b.WriteString("\n\n")
	} else if s.model.SelectingServer {
		b.WriteString(ui.RenderServerSelection(s.model, s.model.Width))
	} else if s.model.Testing {
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase, s.model.Progress))
		b.WriteString("\n\n")
	} else {
		if s.model.Results != nil || len(s.model.TestHistory) > 0 {
			b.WriteString(ui.RenderResults(s.model, s.model.Width))
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

		if s.model.Exporting {
			b.WriteString("\n")
			b.WriteString(ui.RenderExportPrompt(s.model.Width))
		} else if s.model.ExportMessage != "" {
			b.WriteString("\n")
			b.WriteString(ui.RenderExportMessage(s.model.ExportMessage, s.model.Width))
		}

		if s.model.ShowHelp {
			b.WriteString(ui.RenderHelp(s.model.Width, s.model.Results != nil))
		}
	}

	b.WriteString("\n")

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
func exportCmd(result *model.SpeedTestResult, format string) tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return exportDoneMsg{err: fmt.Errorf("could not determine working directory: %v", err)}
		}
		path, err := model.ExportResult(result, format, cwd)
		return exportDoneMsg{path: path, err: err}
	}
}

func migrateHistoryIfNeeded() {
	legacy, err := model.LegacyHistoryPath()
	if err != nil {
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
		return
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
		return
	}
	if err := os.WriteFile(newPath, data, 0600); err != nil {
		return
	}
	// Remove the legacy file
	_ = os.Remove(legacy)
	fmt.Fprintf(os.Stderr, "Info: migrated history from %s to %s\n", legacy, newPath)
}

func runTUI() {
	migrateHistoryIfNeeded()
	m := model.NewDefaultModel()
	if err := m.LoadHistory(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load test history: %v\n", err)
	}

	s := speedTest{
		model:   m,
		spinner: ui.DefaultSpinner,
	}

	if _, err := tea.NewProgram(&s, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
