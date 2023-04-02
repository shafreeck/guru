package tui

import (
	"context"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chzyer/readline"
	"github.com/muesli/termenv"
)

var (
	Stdin  io.ReadCloser = os.Stdin
	Stdout io.Writer     = os.Stdout
	Stderr io.Writer     = os.Stderr
)

// run as a ssh app
var SSHAPPMode bool

type (
	errMsg         error
	doneMsg[V any] struct {
		v V
	}
	eventMsg[E any] struct {
		e E
	}
)

type Model[T any] interface {
	tea.Model
	Value() T
	Error() error
}

func Display[M Model[V], V any](ctx context.Context, m M) (V, error) {
	// set the default output using termenv, tea.WithOutput(Stdout) does not work for vscode terminal
	// TODO figure out why tea.WithOutput breaks
	termenv.SetDefaultOutput(termenv.NewOutput(Stdout, termenv.WithColorCache(true)))
	opts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(Stdin)}
	if !isRenderable() {
		opts = append(opts, tea.WithoutRenderer())
	}
	p := tea.NewProgram(m, opts...)
	done, err := p.Run()
	res := done.(M)
	if res.Error() != nil {
		err = res.Error()
	}
	return res.Value(), err
}

func isRenderable() bool {
	return readline.IsTerminal(int(os.Stdout.Fd())) || SSHAPPMode
}
