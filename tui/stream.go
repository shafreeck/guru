package tui

import (
	"bytes"
	"errors"
	"fmt"
	"unicode/utf8"

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

func WrapWord(text []byte, width int) []byte {
	if len(text) < width {
		return text
	}

	var total, length, prev int
	out := bytes.NewBuffer(nil)
	for i, w := 0, 0; i < len(text); i += w {
		r, size := utf8.DecodeRune(text[i:])
		length += size

		if byte(r) == '\n' || byte(r) == ' ' {
			total += length
			length = 0
		}
		if length > width {
			total += length
			length = 0
			out.Write(text[prev:i])
			prev = i
			out.WriteByte('\n')
		}
		w = size
	}
	out.Write(text[prev:])
	out.WriteByte('\n')
	return out.Bytes()
}

func (s *StreamModel[E, S]) View() string {
	// this is a work around to wrap words for Chinese
	// TODO: find a better way
	text, err := s.renderer.Render(WrapWord(s.out.Bytes(), 110))
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%s %s", string(text), s.Model.View())
}

func (s *StreamModel[E, S]) Value() string {
	return s.out.String()
}

func (s *StreamModel[E, S]) Error() error {
	return s.err
}
