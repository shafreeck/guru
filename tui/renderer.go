package tui

import (
	"encoding/json"
	"os"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type Renderer interface {
	Render([]byte) ([]byte, error)
}

type JSONRenderer struct {
}

func (r *JSONRenderer) Render(text []byte, err error) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(text, &v); err != nil {
		return nil, err
	}
	return json.MarshalIndent(v, "", "  ")
}

type TextRenderer struct {
	Style lipgloss.Style
}

func (r *TextRenderer) Render(text []byte) ([]byte, error) {
	return []byte(r.Style.Render(string(text))), nil
}

type MarkdownRender struct {
}

func (r *MarkdownRender) Render(text []byte) ([]byte, error) {
	// use the markdown renderer to render the response
	md, err := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithAutoStyle(),
	)
	if err != nil {
		return nil, err
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		return md.RenderBytes(text)
	} else {
		return text, nil
	}
}
