package main

import (
	"bytes"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
	"github.com/shafreeck/guru/tui"
)

type completer func(line []rune, pos int) ([][]rune, int)

func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	return c(line, pos)
}

// {prefix} {delimiter} [suffix...]
// for prefix=guru, delimiter = >, and suffix = >
// the prompt string is:
//
//	guru >>>
type LivePrompt struct {
	Prefix    string
	Suffixes  []string
	Delimiter string

	PrefixStyle    lipgloss.Style
	SuffixStyle    lipgloss.Style
	DelimiterStyle lipgloss.Style
}

func (lp *LivePrompt) PushSuffix(suffix string) {
	lp.Suffixes = append(lp.Suffixes, suffix)
}
func (lp *LivePrompt) PopSuffix() {
	if len(lp.Suffixes) == 0 {
		return
	}
	lp.Suffixes = lp.Suffixes[0 : len(lp.Suffixes)-1]
}

func (lp *LivePrompt) Render() string {
	out := bytes.NewBuffer(nil)
	out.WriteString(lp.PrefixStyle.Render(lp.Prefix))
	out.WriteString(" " + lp.DelimiterStyle.Render(lp.Delimiter))
	for _, suffix := range lp.Suffixes {
		out.WriteString(lp.SuffixStyle.Render(suffix))
	}
	out.WriteString(" ")
	return out.String()
}

type Repl struct {
	prompt *LivePrompt
}

func NewRepl(lp *LivePrompt) *Repl {
	return &Repl{prompt: lp}
}

func (repl *Repl) Loop(e *Evaluator) error {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:         repl.prompt.Render(),
		AutoComplete:   completer(completes.Complete),
		Stdin:          tui.Stdin,
		Stdout:         tui.Stdout,
		Stderr:         tui.Stderr,
		FuncIsTerminal: func() bool { return true },
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	for {
		rl.SetPrompt(repl.prompt.Render())

		line, err := rl.Readline()
		if err != nil {
			break
		}
		e.eval(line)
	}
	return nil
}
