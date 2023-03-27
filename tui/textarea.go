package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

var _ Model[string] = &TextAreaModel{}

type TextAreaModel struct {
	abort bool
	textarea.Model
}

func NewTextAreaModel() *TextAreaModel {
	m := textarea.New()
	m.Focus()
	return &TextAreaModel{Model: m}
}

func (m *TextAreaModel) Init() tea.Cmd {
	return textarea.Blink
}
func (m *TextAreaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+d", "esc":
			return m, tea.Quit
		case "ctrl+c":
			m.abort = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.SetWidth(msg.Width)
	}
	var cmd tea.Cmd
	m.Model, cmd = m.Model.Update(msg)
	return m, cmd
}

func (m *TextAreaModel) View() string {
	return fmt.Sprintf("%s\nctrl+d or esc to save and quit, ctrl+c to abort", m.Model.View())
}

func (m *TextAreaModel) Value() string {
	if m.abort {
		return ""
	}
	return m.Model.Value()
}

func (m *TextAreaModel) Error() error {
	return nil
}
