package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

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
	p := tea.NewProgram(m, tea.WithContext(ctx))
	done, err := p.Run()
	res := done.(M)
	if res.Error() != nil {
		err = res.Error()
	}
	return res.Value(), err
}
