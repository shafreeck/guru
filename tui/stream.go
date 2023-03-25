package tui

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ tea.Model = &StreamModel[any, chan any]{}

type StreamModel[E any, S chan E] struct {
	spinner.Model
	err      error
	out      *bytes.Buffer
	stream   S
	renderer MarkdownRender
	onEvent  func(event E) (string, error)
}

func NewStreamModel[E any, S chan E](stream S, onEvent func(event E) (string, error)) *StreamModel[E, S] {
	return &StreamModel[E, S]{
		out:     bytes.NewBuffer(nil),
		stream:  stream,
		onEvent: onEvent,
		Model: spinner.Model{
			Spinner: spinner.Points,
			Style:   lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		},
	}
}

func (s *StreamModel[E, S]) drainEvent() tea.Msg {
	ev, ok := <-s.stream
	// the channel close
	if !ok {
		return doneMsg[E]{ev}
	}
	return eventMsg[E]{e: ev}
}
func (s *StreamModel[E, S]) Init() tea.Cmd {
	return tea.Batch(s.Model.Tick, s.drainEvent)
}

func (s *StreamModel[E, S]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			s.err = errors.New("Ctl+C interrupted")
			return s, tea.Quit
		}
	case errMsg:
		s.err = msg
		s.out.WriteByte('\n')
		return s, tea.Quit
	case eventMsg[E]:
		if text, err := s.onEvent(msg.e); err != nil {
			s.err = err
			s.out.WriteByte('\n')
			return s, tea.Quit
		} else {
			s.out.WriteString(text)
		}
		return s, s.drainEvent
	case doneMsg[E]:
		s.out.WriteByte('\n')
		return s, tea.Quit
	default:
		var cmd tea.Cmd
		s.Model, cmd = s.Model.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s *StreamModel[E, S]) View() string {
	text, err := s.renderer.Render(s.out.Bytes())
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s %s", string(text), s.Model.View())
}

func (s *StreamModel[E, S]) Value() string {
	return s.out.String()
}
