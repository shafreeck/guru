package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

var _ Model[string] = &ContentModel{}

type ContentModel struct {
	Val      string
	Text     string
	renderer Renderer
}

func NewContentModel(text string, rendererName string) *ContentModel {
	return &ContentModel{Text: text, renderer: NewRenderer(rendererName)}
}

func (m *ContentModel) Init() tea.Cmd {
	return tea.Quit
}
func (m *ContentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}
func (m *ContentModel) View() string {
	text, err := m.renderer.Render((m.Text))
	if err != nil {
		return err.Error()
	}
	m.Val = string(text)
	return m.Val
}
func (m *ContentModel) Value() string {
	return m.Text
}

func (m *ContentModel) Error() error {
	return nil
}
