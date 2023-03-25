package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ tea.Model = &SpinnerModel[any]{}

type SpinnerModel[V any] struct {
	spinner.Model
	hint string
	err  error

	do  func() (V, error)
	Val V
}

func NewSpinnerModel[V any](hint string, do func() (V, error)) *SpinnerModel[V] {
	return &SpinnerModel[V]{
		hint: hint,
		do:   do,
		Model: spinner.Model{
			Spinner: spinner.Dot,
			Style:   lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		},
	}
}
func (s *SpinnerModel[V]) Init() tea.Cmd {
	return tea.Batch(s.Model.Tick,
		func() tea.Msg {
			if v, err := s.do(); err != nil {
				return errMsg(err)
			} else {
				return doneMsg[V]{v: v}
			}
		})
}

func (s *SpinnerModel[V]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			s.err = errors.New("Ctl+C interrupted")
			return s, tea.Quit
		}
	case errMsg:
		s.err = msg
	case doneMsg[V]:
		s.Val = msg.v
		return s, tea.Quit
	default:
		var cmd tea.Cmd
		s.Model, cmd = s.Model.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s *SpinnerModel[V]) View() string {
	str := fmt.Sprintf("%s %s", s.Model.View(), s.hint)
	return str + strings.Repeat(" ", 10) + "\r"
}

func (s *SpinnerModel[V]) Value() V {
	return s.Val
}
