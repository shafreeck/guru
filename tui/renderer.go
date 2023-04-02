package tui

import (
	"bytes"
	"os"

	"github.com/alecthomas/chroma/quick"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type Renderer interface {
	Render([]byte) ([]byte, error)
}

type JSONRenderer struct {
}

func (r *JSONRenderer) Render(text string) (string, error) {
	out := bytes.NewBuffer(nil)
	if err := quick.Highlight(out, string(text), "json", "terminal256", "monokai"); err != nil {
		return "", err
	}
	return out.String(), nil
}

type TextRenderer struct {
	Style lipgloss.Style
}

func (r *TextRenderer) Render(text []byte) ([]byte, error) {
	return []byte(r.Style.Render(string(text))), nil
}

type MarkdownRender struct {
}

func (r *MarkdownRender) Render(text string) (string, error) {
	// use the markdown renderer to render the response
	md, err := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithAutoStyle(),
	)
	if err != nil {
		return "", err
	}

	if term.IsTerminal(int(os.Stdout.Fd())) {
		return md.Render(text)
	} else {
		return text, nil
	}
}
