package main

import (
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
}

type progressMsg struct {
	Progress float64
	Phase    string
}

type testComplete struct {
	err error
}

func (s *speedTest) Init() tea.Cmd {
	return s.spinner.Tick
}

func (s *speedTest) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			s.quitting = true
			return s, tea.Quit
		case "n":
			if !s.model.Testing {
				s.model.Testing = true
				s.model.Progress = 0
				s.model.CurrentPhase = "Starting speed test..."
				s.model.Error = nil

				s.progressChan = make(chan model.ProgressUpdate)
				s.errChan = make(chan error, 1)
				go func() {
					err := s.model.PerformSpeedTest(s.progressChan)
					s.errChan <- err
					close(s.progressChan)
				}()
				s.model.ShowHelp = false // Hide help when starting speed test
				return s, tea.Batch(
					s.spinner.Tick,
					waitForProgress(s.progressChan, s.errChan),
				)
			}
		case "h":
			s.model.ShowHelp = !s.model.ShowHelp
		}

	case tea.WindowSizeMsg:
		s.model.Width = msg.Width
		s.model.Height = msg.Height

	case spinner.TickMsg:
		var tickCmd tea.Cmd
		s.spinner, tickCmd = s.spinner.Update(msg)
		return s, tickCmd

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

	if s.model.Testing {
		b.WriteString(ui.RenderSpinner(s.spinner, s.model.Width, s.model.CurrentPhase))
		b.WriteString("\n\n")
	}

	if s.model.Results != nil || len(s.model.TestHistory) > 0 {
		b.WriteString(ui.RenderResults(s.model, s.model.Width))
		b.WriteString("\n")
	}

	if s.model.Error != nil {
		b.WriteString("\n")
		b.WriteString(ui.RenderError(s.model.Error, s.model.Width))
	}

	if s.model.ShowHelp {
		b.WriteString(ui.RenderHelp(s.model.Width))
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
	s := speedTest{
		model:   model.NewModel(),
		spinner: ui.DefaultSpinner,
	}

	if _, err := tea.NewProgram(&s, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
