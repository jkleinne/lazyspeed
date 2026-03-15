package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jkleinne/lazyspeed/model"
	"github.com/jkleinne/lazyspeed/ui"
)

type speedTest struct {
	model        *model.Model
	spinner      spinner.Model
	quitting     bool
	err          error
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
		err := m.FetchServerList(context.Background())
		return serverListMsg{err: err}
	}
}

func (s *speedTest) Init() tea.Cmd {
	s.model.FetchingServers = true
	cmds := []tea.Cmd{fetchServerListCmd(s.model)}

	if len(s.model.TestHistory) == 0 {
		// First launch: show loading spinner since there's nothing else to display
		s.model.PendingServerSelection = true
		s.model.CurrentPhase = "Fetching server list..."
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
		if s.model.FetchingServers && s.model.PendingServerSelection {
			// Spinner is visible while waiting for server list — only allow quit
			switch msg.String() {
			case "q", "ctrl+c":
				s.quitting = true
				return s, tea.Quit
			}
		} else if s.model.SelectingServer {
			switch msg.String() {
			case "q", "ctrl+c":
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

				ctx, cancel := context.WithCancel(context.Background())
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
			case "q", "ctrl+c":
				s.cancelTestIfRunning()
				s.quitting = true
				return s, tea.Quit
			case "n":
				if !s.model.Testing && !s.model.SelectingServer {
					if s.model.FetchingServers {
						// Servers still loading — show the spinner and queue the transition
						s.model.PendingServerSelection = true
						s.model.CurrentPhase = "Fetching server list..."
						s.model.ShowHelp = false
						return s, s.spinner.Tick
					}
					s.model.SelectingServer = true
					s.model.ShowHelp = false
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

		if s.model.ShowHelp {
			b.WriteString(ui.RenderHelp(s.model.Width))
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(GetVersionInfo())
		os.Exit(0)
	}

	m := model.NewModel()
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
