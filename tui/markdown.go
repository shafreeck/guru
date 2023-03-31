package tui

import tea "github.com/charmbracelet/bubbletea"

var _ Model[string] = &MarkdownModel{}

type MarkdownModel struct {
	r    MarkdownRender
	Text string
	Val  string
}

func NewMarkdownModel(text string) *MarkdownModel {
	return &MarkdownModel{Text: text}
}

func (m *MarkdownModel) Init() tea.Cmd {
	return tea.Quit
}
func (m *MarkdownModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}
func (m *MarkdownModel) View() string {
	text, err := m.r.Render((m.Text))
	if err != nil {
		return err.Error()
	}
	m.Val = string(text)
	return m.Val
}
func (m *MarkdownModel) Value() string {
	return m.Text
}

func (m *MarkdownModel) Error() error {
	return nil
}
