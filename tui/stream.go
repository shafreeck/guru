package tui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
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
	text     string // the text to draw by view
	stream   S
	renderer MarkdownRender
	onEvent  func(event E) (string, error)
	height   int // the window height
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

func (s *StreamModel[E, S]) Update(msg tea.Msg) (m tea.Model, cmd tea.Cmd) {
	quiting := false
	defer func() {
		if quiting {
			cmd = tea.Sequence(s.scrollLines(s.text), tea.Quit)
		}
	}()
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			s.err = errors.New("Ctl+C interrupted")
			quiting = true
			return s, tea.Quit
		}
	case tea.WindowSizeMsg:
		s.height = msg.Height
	case errMsg:
		s.err = msg
		quiting = true
		return s, tea.Quit
	case eventMsg[E]:
		if text, err := s.onEvent(msg.e); err != nil {
			s.err = err
			quiting = true
			return s, tea.Quit
		} else {
			s.out.WriteString(text)
		}
		// this is a work around to wrap words for Chinese
		// TODO: find a better way
		data, err := s.renderer.Render(string(WrapWord(s.out.Bytes(), 110)))
		if err != nil {
			s.text = err.Error()
		}
		s.text = string(data)
		return s, s.drainEvent
	case doneMsg[E]:
		quiting = true
		return s, tea.Quit
	}
	s.Model, cmd = s.Model.Update(msg)
	return s, cmd
}

func WrapWord(text []byte, width int) []byte {
	if len(text) < width {
		return text
	}

	var length, prev int
	out := bytes.NewBuffer(nil)
	for i, w := 0, 0; i < len(text); i += w {
		r, size := utf8.DecodeRune(text[i:])
		length += size

		if size == 1 && (byte(r) == '\n' || byte(r) == ' ') {
			length = 0
		}
		if length > width {
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

// send back the scrolled lines
func (s *StreamModel[E, S]) scrollLines(text string) tea.Cmd {
	lines := strings.Split(text, "\n")
	if n := len(lines) - s.height; s.height > 0 && n > 0 {
		lines = lines[:n]
		return tea.Printf("%s", strings.Join(lines, "\n"))
	}
	return nil
}

func (s *StreamModel[E, S]) View() string {
	return fmt.Sprintf("%s %s", s.text, s.Model.View())
}

func (s *StreamModel[E, S]) Value() string {
	return s.out.String()
}

func (s *StreamModel[E, S]) Error() error {
	return s.err
}
