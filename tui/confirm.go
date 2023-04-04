package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var _ Model[bool] = &ConfirmModel{}

type ConfirmModel struct {
	prompt        string
	quiting       bool
	buttons       []string
	focusIndex    int
	focusedRender func(button string) string
	blurredRender func(button string) string

	confirmed bool
}

func NewConfimModel(prompt string) *ConfirmModel {
	buttons := []string{"Yes", "No"}
	focusedRender := func(button string) string {
		return focusedStyle.Copy().Render(fmt.Sprintf("[ %s ]", button))
	}
	blurredRender := func(button string) string {
		return fmt.Sprintf("[ %s ]", blurredStyle.Render(button))
	}
	return &ConfirmModel{prompt: prompt, buttons: buttons,
		focusedRender: focusedRender, blurredRender: blurredRender}
}

func (m *ConfirmModel) Init() tea.Cmd {
	return nil
}

func (m *ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.quiting = true
			m.confirmed = false
			return m, tea.Quit
		case "y", "Y":
			m.focusIndex = 1
		case "n", "N":
			m.focusIndex = 2
		// Set focus to next input
		case "tab", "shift+tab", "enter", "left", "right", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so, exit.
			if s == "enter" {
				m.confirmed = m.focusIndex == 1
				return m, tea.Quit
			}

			// Cycle indexes
			if s == "up" || s == "left" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.buttons) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.buttons)
			}
		}
	}
	return m, nil
}
func (m *ConfirmModel) View() string {
	if m.quiting {
		return ""
	}
	var b strings.Builder

	buttons := []string{
		m.blurredRender(m.buttons[0]),
		m.blurredRender(m.buttons[1]),
	}
	if m.focusIndex > 0 {
		buttons[m.focusIndex-1] = m.focusedRender(m.buttons[m.focusIndex-1])
	}

	fmt.Fprintf(&b, "%s  %s\n\n", buttons[0], buttons[1])
	b.WriteString(helpStyle.Render("(ctrl+c, esc or q to quit)"))

	return b.String()
}
func (m *ConfirmModel) Value() bool {
	return m.confirmed
}
func (m *ConfirmModel) Error() error {
	return nil
}
